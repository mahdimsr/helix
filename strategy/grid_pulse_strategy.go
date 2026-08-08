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

// ============================================================
// اجرای بهینه‌سازی
// ============================================================

// RunPulseOptimization تمام ترکیبات ممکن را تست می‌کند
// و 5 ترکیب برتر را بر اساس سود مرکب برمی‌گرداند
func RunPulseOptimization(candles []models.Candle) []OptimizationResult {

	// ========== مقادیر تست ==========
	bodyThresholds := []float64{0.1, 0.2, 0.3}
	tpBodyRatios := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	slMultipliers := []int{2, 3, 4, 5, 6, 7, 8, 9, 10}
	allocations := []float64{10, 15, 20, 25, 30, 35, 40, 45, 50}
	leverages := []int{1, 5, 10, 20, 50, 100}

	totalCombos := len(bodyThresholds) * len(tpBodyRatios) * len(slMultipliers) * len(allocations) * len(leverages)
	fmt.Printf("🚀 Starting optimization with %d combinations...\n\n", totalCombos)

	var allResults []OptimizationResult
	current := 0
	skipped := 0 // شمارنده ترکیب‌های رد شده توسط فیلتر ایمنی

	// ========== حلقه‌های تو در تو برای تمام ترکیبات ==========
	for _, bt := range bodyThresholds {
		for _, tpr := range tpBodyRatios {
			for _, slm := range slMultipliers {
				for _, alloc := range allocations {
					for _, lev := range leverages {

						// ==========================================
						// 🛡️ فیلتر ایمنی: جلوگیری از لیکوئید شدن قبل از SL
						// ==========================================
						// فرض: میانگین بادی یک کندل ماروبوزوی معتبر حدود 0.5% است
						estimatedBodyPercent := 0.5

						// فاصله حد ضرر (SL) از نقطه ورود به درصد
						slDistancePercent := tpr * float64(slm) * estimatedBodyPercent

						// فاصله تقریبی قیمت لیکوئید از نقطه ورود به درصد (100 / لوریج)
						liqDistancePercent := 100.0 / float64(lev)

						// اگر قیمت لیکوئید نزدیک‌تر از حد ضرر باشد، این ترکیب پرریسک است!
						// یعنی قبل از اینکه SL بخورد، کل مارجین از دست می‌رود.
						if liqDistancePercent <= slDistancePercent {
							skipped++
							continue // رد کردن این ترکیب و رفتن به ترکیب بعدی
						}
						// ==========================================

						current++

						// محاسبه MaxConcurrentTrades
						maxConcurrent := int(1000.0 / alloc)

						cfg := PulseStrategyConfig{
							BodyThresholdPercent: bt,
							TPBodyRatio:          tpr,
							SLMultiplier:         float64(slm),
							AllocationPercent:    alloc,
							Leverage:             lev,
							FeePercent:           0,
							InitialCapital:       1000,
							MaxConcurrentTrades:  maxConcurrent,
						}

						// اجرای استراتژی
						strategyResult := PulseStrategy(candles, cfg)

						// صرف‌نظر از نتایج با تعداد معاملات خیلی کم
						if strategyResult.TotalTrades < 5 {
							continue
						}

						// ذخیره نتیجه
						allResults = append(allResults, OptimizationResult{
							Config:       cfg,
							CompoundGain: strategyResult.Capital.CompoundGainPercent,
							SimpleGain:   strategyResult.Capital.SimpleGainPercent,
							TotalTrades:  strategyResult.TotalTrades,
							WinRate:      strategyResult.WinRate,
							MaxDrawdown:  strategyResult.Capital.MaxDrawdownPercent,
							FinalCapital: strategyResult.Capital.FinalCapital,
						})

						// نمایش پیشرفت هر 500 تا
						if current%500 == 0 {
							fmt.Printf("⏳ Progress: %d tested | %d skipped\n", current, skipped)
						}
					}
				}
			}
		}
	}

	fmt.Printf("✅ Optimization completed. Tested: %d | Skipped (Unsafe): %d\n", current, skipped)
	fmt.Println("Sorting results...\n")

	// ========== سورت بر اساس CompoundGain (نزولی) ==========
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].CompoundGain > allResults[j].CompoundGain
	})

	// ========== انتخاب 5 تای برتر ==========
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
	fmt.Println("┌────┬────────┬──────────┬─────────┬────────┬─────┬────────┬─────────┬──────────────┬──────────┐")
	fmt.Println("│ #  │ Body%  │ TP Ratio │ SL Mult │ Alloc% │ Lev │ Trades │ WinRate │ CompoundGain │ Drawdown │")
	fmt.Println("├────┼────────┼──────────┼─────────┼────────┼─────┼────────┼─────────┼──────────────┼──────────┤")

	limit := count
	if len(results) < limit {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		r := results[i]
		fmt.Printf("│ %-2d │ %-6.1f │ %-8.1f │ %-0.1f │ %-6.0f │ %-3d │ %-6d │ %-6.1f%% │ %-12.2f%% │ %-7.2f%% │\n",
			i+1,
			r.Config.BodyThresholdPercent,
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

	fmt.Println("└────┴────────┴──────────┴─────────┴────────┴─────┴────────┴─────────┴──────────────┴──────────┘")
}
