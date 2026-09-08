package strategy

import (
	"fmt"
	"helix/models"
	"sort"
)

// ============================================================
// ساختار نتایج بهینه‌سازی
// ============================================================

type OptimizationResult struct {
	Rank         int
	Config       PulseStrategyConfig
	CompoundGain float64
	SimpleGain   float64
	TotalTrades  int
	WinRate      float64
	MaxDrawdown  float64
	FinalCapital float64
}

// TradeWithAlloc برای نگهداری معامله و درصد سرمایه مخصوص آن در حالت داینامیک
type TradeWithAlloc struct {
	Trade      models.Trade
	Allocation float64
}

// ============================================================
// اجرای بهینه‌سازی (با فیلترهای ایمنی و WinRate)
// ============================================================
func RunPulseOptimization(candles []models.Candle) []OptimizationResult {
	bodyThresholds := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.5}
	tpBodyRatios := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	slMultipliers := []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 20, 30}
	allocations := []float64{10, 15, 20, 25, 30, 35, 40}
	leverages := []int{10}

	totalCombos := len(bodyThresholds) * len(tpBodyRatios) * len(slMultipliers) * len(allocations) * len(leverages)
	fmt.Printf("🚀 Starting optimization with %d combinations...\n\n", totalCombos)

	var allResults []OptimizationResult
	current := 0
	skipped := 0

	for _, bt := range bodyThresholds {
		for _, maxBody := range bodyThresholds {
			for _, tpr := range tpBodyRatios {
				for _, slm := range slMultipliers {
					for _, alloc := range allocations {
						for _, lev := range leverages {

							// ==========================================
							// 🛡️ فیلتر ایمنی: جلوگیری از لیکوئید شدن (حتی در لوریج 100)
							// ==========================================
							// میانگین بادی کندل‌های ماروبوزو در تایم‌فریم 15 دقیقه معمولاً بین 0.5% تا 1.5% است.
							// برای سخت‌گیرانه‌ترین حالت، 1.0% را به عنوان پایه در نظر می‌گیریم.
							estimatedBodyPercent := 1.0
							slDistancePercent := tpr * float64(slm) * estimatedBodyPercent

							// فاصله لیکوئید با 15% حاشیه ایمنی (Safety Buffer)
							// در لوریج 100، فاصله لیکوئید حدود 1% است. 15% حاشیه = 0.85%
							liqDistancePercent := (100.0 / float64(lev)) * 0.85

							if slDistancePercent >= liqDistancePercent {
								skipped++
								continue // رد کردن ترکیب‌های پرریسک
							}
							// ==========================================

							current++
							maxConcurrent := int(100.0 / alloc)

							cfg := PulseStrategyConfig{
								BodyThresholdPercent:    bt,
								BodyThresholdPercentMax: maxBody,
								TPBodyRatio:             tpr,
								SLMultiplier:            float64(slm),
								AllocationPercent:       alloc,
								Leverage:                lev,
								FeePercent:              0,
								InitialCapital:          1000,
								MaxConcurrentTrades:     maxConcurrent,
							}

							strategyResult := PulseStrategy(candles, cfg)

							if strategyResult.TotalTrades < 5 {
								continue
							}

							// ==========================================
							// 🎯 فیلتر نرخ برد (WinRate) - هدف: 30% برخورد با SL
							// ==========================================
							// اگر 30% معاملات SL بخورند، یعنی WinRate باید حدود 70% باشد.
							// ما بازه 60% تا 80% را به عنوان نتایج معتبر و سودده می‌پذیریم.
							if strategyResult.WinRate < 70.0 {
								continue
							}
							// ==========================================

							allResults = append(allResults, OptimizationResult{
								Config:       cfg,
								CompoundGain: strategyResult.Capital.CompoundGainPercent,
								SimpleGain:   strategyResult.Capital.SimpleGainPercent,
								TotalTrades:  strategyResult.TotalTrades,
								WinRate:      strategyResult.WinRate,
								MaxDrawdown:  strategyResult.Capital.MaxDrawdownPercent,
								FinalCapital: strategyResult.Capital.FinalCapital,
							})

							if current%500 == 0 {
								fmt.Printf("⏳ Progress: %d tested | %d skipped\n", current, skipped)
							}
						}
					}
				}
			}

		}
	}

	fmt.Printf("✅ Optimization completed. Tested: %d | Skipped (Unsafe/Bad WinRate): %d\n", current, skipped)
	fmt.Println("Sorting results...\n")

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].CompoundGain > allResults[j].CompoundGain
	})

	topCount := 5
	if len(allResults) < topCount {
		topCount = len(allResults)
	}

	topResults := make([]OptimizationResult, topCount)
	for i := 0; i < topCount; i++ {
		topResults[i] = allResults[i]
		topResults[i].Rank = i + 1
	}

	printTopResults(allResults, topCount)
	return topResults
}

// ============================================================
// چاپ نتایج به صورت جدول
// ============================================================

func printTopResults(results []OptimizationResult, count int) {
	fmt.Println("┌────┬────────┬────────┬──────────┬─────────┬────────┬─────┬────────┬─────────┬──────────────┬──────────┐")
	fmt.Println("│ #  │ Min %  │  Max%  │ TP Ratio │ SL Mult │ Alloc% │ Lev │ Trades │ WinRate │ CompoundGain │ Drawdown │")
	fmt.Println("├────┼────────┼────────┼──────────┼─────────┼────────┼─────┼────────┼─────────┼──────────────┼──────────┤")

	limit := count
	if len(results) < limit {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		r := results[i]
		fmt.Printf("│ %-2d │ %-6.1f │ %-6.1f │ %-8.1f │ %-0.1f │ %-6.0f │ %-3d │ %-6d │ %-6.1f%% │ %-12.2f%% │ %-7.2f%% │\n",
			i+1,
			r.Config.BodyThresholdPercent,
			r.Config.BodyThresholdPercentMax,
			r.Config.TPBodyRatio,
			r.Config.SLMultiplier,
			r.Config.AllocationPercent,
			r.Config.Leverage,
			r.TotalTrades,
			r.WinRate,
			r.CompoundGain,
			r.MaxDrawdown,
		)
	}

	fmt.Println("└────┴────────┴────────┴──────────┴─────────┴────────┴─────┴────────┴─────────┴──────────────┴──────────┘")
}
