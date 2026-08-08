package strategy

import (
	"fmt"
	"helix/models"
	"sort"
)

type CapitalMetrics struct {
	InitialCapital      float64
	FinalCapital        float64
	SimpleGainPercent   float64
	CompoundGainPercent float64
	MaxDrawdownPercent  float64
	MaxOpenTrades       int
	MaxAllocatedPercent float64
}

type StrategyResult struct {
	Trades []models.Trade

	TotalTrades    int
	Wins           int
	Losses         int
	WinRate        float64
	AvgWinPercent  float64
	AvgLossPercent float64

	Capital CapitalMetrics
}

// PulseStrategyConfig پارامترهای قابل تنظیم استراتژی
type PulseStrategyConfig struct {
	BodyThresholdPercent float64 // حداقل درصد بادی کندل برای سیگنال (مثلاً 0.1)
	TPBodyRatio          float64 // نسبت TP به بادی کندل (0.25 یعنی ۲۵٪ کندل)
	SLMultiplier         float64 // ضریب حد ضرر نسبت به فاصله TP (مثلاً 5)
	AllocationPercent    float64 // درصد سرمایه درگیر در هر معامله (مثلاً 10)
	Leverage             int     // لوریج (مثلاً 5)
	FeePercent           float64 // کارمزد هر معامله (مثلاً 0.1)
	InitialCapital       float64 // سرمایه اولیه (مثلاً 1000)
	MaxConcurrentTrades  int     // حداکثر معاملات همزمان (مثلاً 10)
}

func PulseStrategy(candles []models.Candle, cfg PulseStrategyConfig) StrategyResult {

	// ========== گام ۱: شناسایی سیگنال‌های معتبر ==========
	type signalInfo struct {
		index int
	}
	var signals []signalInfo

	for i := 0; i < len(candles); i++ {
		c := candles[i]

		// شرط ۱: کندل ماروبوزو باشد
		if !c.IsMarubozu() {
			continue
		}

		// شرط ۲: بادی کندل بیشتر از آستانه مشخص شده باشد
		if c.BodyPercentage() < cfg.BodyThresholdPercent {
			continue
		}

		signals = append(signals, signalInfo{index: i})
	}

	// ========== گام ۲: شبیه‌سازی مستقل هر سیگنال ==========
	var simulatedTrades []models.Trade
	for _, sig := range signals {
		trade, closed := SimulatePulseTrade(candles, sig.index, cfg)
		if closed {
			simulatedTrades = append(simulatedTrades, trade)
		}
	}

	// ========== گام ۳: مرتب‌سازی بر اساس زمان ورود ==========
	sort.Slice(simulatedTrades, func(i, j int) bool {
		return simulatedTrades[i].OpenTime < simulatedTrades[j].OpenTime
	})

	// ========== گام ۴: اعمال محدودیت معاملات همزمان ==========
	// فقط معاملاتی پذیرفته می‌شوند که در زمان ورودشان،
	// تعداد معاملات باز کمتر از حد مجاز باشد
	var acceptedTrades []models.Trade
	for _, trade := range simulatedTrades {
		openCount := 0
		for _, openTrade := range acceptedTrades {
			// اگر معامله قبلی هنوز در زمان ورود این معامله باز بوده
			if openTrade.CloseTime > trade.OpenTime {
				openCount++
			}
		}

		if openCount < cfg.MaxConcurrentTrades {
			acceptedTrades = append(acceptedTrades, trade)
		}
	}

	// ========== گام ۵: محاسبه نتایج نهایی ==========
	result := StrategyResult{
		Trades:  acceptedTrades,
		Capital: calculateCapitalParallel(acceptedTrades, cfg.AllocationPercent, cfg.InitialCapital),
	}
	result.calculateTradeStats()

	return result
}

func calculateCapitalParallel(trades []models.Trade, allocationPercent float64, initialCapital float64) CapitalMetrics {

	metrics := CapitalMetrics{
		InitialCapital: initialCapital,
		FinalCapital:   initialCapital,
	}

	if len(trades) == 0 || initialCapital <= 0 {
		return metrics
	}

	allocation := allocationPercent / 100.0

	// ---------- ساخت رویدادها (باز/بسته شدن معاملات) ----------
	type tradeEvent struct {
		time     int64
		tradeIdx int
		isOpen   bool
	}

	var events []tradeEvent
	for i, t := range trades {
		events = append(events, tradeEvent{time: t.OpenTime, tradeIdx: i, isOpen: true})
		events = append(events, tradeEvent{time: t.CloseTime, tradeIdx: i, isOpen: false})
	}

	// مرتب‌سازی رویدادها بر اساس زمان
	// اگر زمان یکسان بود، اول close بعد open (تا سرمایه آزاد شود)
	sort.Slice(events, func(i, j int) bool {
		if events[i].time == events[j].time {
			return !events[i].isOpen
		}
		return events[i].time < events[j].time
	})

	// ---------- شبیه‌سازی رویدادها ----------
	currentCapital := initialCapital
	peakCapital := initialCapital
	simpleSum := 0.0
	openCount := 0

	for _, ev := range events {
		if ev.isOpen {
			// باز شدن معامله
			openCount++
			if openCount > metrics.MaxOpenTrades {
				metrics.MaxOpenTrades = openCount
			}

			// ردیابی حداکثر سرمایه درگیر همزمان
			allocatedNow := float64(openCount) * allocationPercent
			if allocatedNow > metrics.MaxAllocatedPercent {
				metrics.MaxAllocatedPercent = allocatedNow
			}
		} else {
			// بسته شدن معامله
			openCount--
			t := trades[ev.tradeIdx]
			roi := t.GainPercent / 100.0

			// --- سود ساده: درصد ثابت از سرمایه اولیه ---
			simpleSum += roi * allocation

			// --- سود مرکب: درصد از سرمایه فعلی ---
			allocatedMargin := currentCapital * allocation
			profitLoss := allocatedMargin * roi
			currentCapital += profitLoss

			if currentCapital < 0 {
				currentCapital = 0
			}

			// --- محاسبه Drawdown ---
			if currentCapital > peakCapital {
				peakCapital = currentCapital
			}
			drawdown := (peakCapital - currentCapital) / peakCapital * 100
			if drawdown > metrics.MaxDrawdownPercent {
				metrics.MaxDrawdownPercent = drawdown
			}
		}
	}

	metrics.FinalCapital = currentCapital
	metrics.SimpleGainPercent = simpleSum * 100
	metrics.CompoundGainPercent = (currentCapital - initialCapital) / initialCapital * 100

	return metrics
}

func (r *StrategyResult) calculateTradeStats() {
	r.TotalTrades = len(r.Trades)

	var totalWin, totalLoss float64
	var winCount, lossCount int

	for _, t := range r.Trades {
		if t.GainPercent >= 0 {
			winCount++
			totalWin += t.GainPercent
		} else {
			lossCount++
			totalLoss += t.GainPercent
		}
	}

	r.Wins = winCount
	r.Losses = lossCount

	if r.TotalTrades > 0 {
		r.WinRate = float64(winCount) / float64(r.TotalTrades) * 100
	}
	if winCount > 0 {
		r.AvgWinPercent = totalWin / float64(winCount)
	}
	if lossCount > 0 {
		r.AvgLossPercent = totalLoss / float64(lossCount)
	}
}

func (r StrategyResult) PrintResult() {
	fmt.Println("========== Strategy Result ==========")
	fmt.Printf("Total Trades      : %d\n", r.TotalTrades)
	fmt.Printf("Wins              : %d\n", r.Wins)
	fmt.Printf("Losses            : %d\n", r.Losses)
	fmt.Printf("Win Rate          : %.2f%%\n", r.WinRate)
	fmt.Printf("Avg Win           : %.2f%%\n", r.AvgWinPercent)
	fmt.Printf("Avg Loss          : %.2f%%\n", r.AvgLossPercent)
	fmt.Println("-------------------------------------")
	fmt.Printf("Initial Capital   : %.2f\n", r.Capital.InitialCapital)
	fmt.Printf("Final Capital     : %.2f\n", r.Capital.FinalCapital)
	fmt.Printf("Simple Gain       : %.2f%%\n", r.Capital.SimpleGainPercent)
	fmt.Printf("Compound Gain     : %.2f%%\n", r.Capital.CompoundGainPercent)
	fmt.Printf("Max Drawdown      : %.2f%%\n", r.Capital.MaxDrawdownPercent)
	fmt.Printf("Max Open Trades   : %d\n", r.Capital.MaxOpenTrades)
	fmt.Printf("Max Allocated     : %.2f%%\n", r.Capital.MaxAllocatedPercent)
	fmt.Println("-------------------------------------")
	fmt.Println("Trades List:")
	for i, t := range r.Trades {
		status := "WIN"
		if t.GainPercent < 0 {
			status = "LOSS"
		}
		fmt.Printf("#%d %s | entry=%.2f exit=%.2f pct=%.3f%% status=%s open=%d close=%d\n",
			i+1, t.Type, t.OpenPrice, t.ClosePrice, t.GainPercent, status, t.OpenTime, t.CloseTime)
	}
	fmt.Println("=====================================")
}

func SimulatePulseTrade(candles []models.Candle, signalIdx int, cfg PulseStrategyConfig) (models.Trade, bool) {

	signal := candles[signalIdx]

	// نقطه ورود: قیمت Close کندل سیگنال
	entry := signal.Close

	// فاصله TP: نسبت مشخص‌شده از بادی کندل
	// TPBodyRatio = 0.25 یعنی ۲۵٪ بادی کندل
	tpDist := signal.Body() * cfg.TPBodyRatio

	var tp, sl float64
	var position models.Position

	if signal.IsGreen() {
		// کندل صعودی -> معامله فروش (خلاف جهت کندل)
		position = models.SellPosition
		tp = entry - tpDist
		// حد ضرر = ضریب × فاصله TP
		sl = entry + cfg.SLMultiplier*tpDist
	} else {
		// کندل نزولی -> معامله خرید (خلاف جهت کندل)
		position = models.BuyPosition
		tp = entry + tpDist
		// حد ضرر = ضریب × فاصله TP
		sl = entry - cfg.SLMultiplier*tpDist
	}

	return simulateTrade(candles, signalIdx, position, entry, tp, sl, cfg.AllocationPercent, cfg.FeePercent, cfg.Leverage)
}

func simulateTrade(candles []models.Candle, entryIdx int, position models.Position, price, tp, sl, margin, fee float64, leverage int) (models.Trade, bool) {

	open := candles[entryIdx]

	t := models.Trade{
		OpenPrice:   price,
		OpenTime:    open.Time,
		ATR:         0,
		Sensitivity: 0,
	}
	if position == models.BuyPosition {
		t.Type = "Long"
	} else {
		t.Type = "Short"
	}

	runupPrice, riskPrice := price, price
	runupTime, riskTime := open.Time, open.Time

	for j := entryIdx + 1; j < len(candles); j++ {
		bar := candles[j]

		if position == models.BuyPosition {
			if bar.High > runupPrice {
				runupPrice = bar.High
				runupTime = bar.Time
			}
			if bar.Low < riskPrice {
				riskPrice = bar.Low
				riskTime = bar.Time
			}
		} else { // Short
			if bar.Low < runupPrice {
				runupPrice = bar.Low
				runupTime = bar.Time
			}
			if bar.High > riskPrice {
				riskPrice = bar.High
				riskTime = bar.Time
			}
		}

		var hitTP, hitSL bool
		if position == models.BuyPosition {
			hitTP = bar.High >= tp
			hitSL = bar.Low <= sl
		} else {
			hitTP = bar.Low <= tp
			hitSL = bar.High >= sl
		}

		if !hitTP && !hitSL {
			continue
		}
		won := hitTP && !hitSL
		exitPrice := sl
		if won {
			exitPrice = tp
		}

		if position == models.BuyPosition {
			if runupPrice > tp {
				runupPrice = tp
			}
			if riskPrice < sl {
				riskPrice = sl
			}
		} else {
			if runupPrice < tp {
				runupPrice = tp
			}
			if riskPrice > sl {
				riskPrice = sl
			}
		}

		t.ClosePrice = exitPrice
		t.CloseTime = bar.Time
		t.Duration = bar.Time - open.Time

		t.RunupPrice = runupPrice
		t.RunupTime = runupTime
		t.RunupDuration = runupTime - open.Time

		t.RiskPrice = riskPrice
		t.RiskTime = riskTime
		t.RiskDuration = riskTime - open.Time

		if position == models.BuyPosition {
			t.GainPercent = models.CalcGainPercent(margin, price, exitPrice, leverage, fee, true)
			t.RunupPercent = (runupPrice - price) / price * 100
			t.RiskPercent = (riskPrice - price) / price * 100 // معمولاً منفی
		} else {
			t.GainPercent = models.CalcGainPercent(margin, price, exitPrice, leverage, fee, false)
			t.RunupPercent = (price - runupPrice) / price * 100
			t.RiskPercent = (price - riskPrice) / price * 100 // معمولاً منفی
		}

		return t, true
	}

	return models.Trade{}, false
}
