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

func PulseStrategy(candles []models.Candle, allocationPercent float64, initialCapital float64, maxConcurrentTrades int) StrategyResult {

	// ========== گام ۱: شناسایی سیگنال‌های معتبر ==========
	type signalInfo struct {
		index int
	}
	var signals []signalInfo

	for i := 1; i < len(candles); i++ {
		c := candles[i]

		if !c.IsMarubozu() {
			continue
		}
		if !isCandleBodyBigger(candles, i, 100, 60) {
			continue
		}
		if c.BodyPercentage() < 0.3 {
			continue
		}

		signals = append(signals, signalInfo{index: i})
	}

	// ========== گام ۲: شبیه‌سازی مستقل هر سیگنال ==========
	// (منطق TP/SL دقیقاً مثل قبل است)
	var simulatedTrades []models.Trade
	for _, sig := range signals {
		trade, closed := SimulatePulseTrade(candles, sig.index, 100, 10, 1, 100, 8)
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

		if openCount < maxConcurrentTrades {
			acceptedTrades = append(acceptedTrades, trade)
		}
	}

	// ========== گام ۵: محاسبه نتایج نهایی ==========
	result := StrategyResult{
		Trades:  acceptedTrades,
		Capital: calculateCapitalParallel(acceptedTrades, allocationPercent, initialCapital),
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

func SimulatePulseTrade(candles []models.Candle, signalIdx int, lookBack, retraceNext, leverage int, margin, fee float64) (models.Trade, bool) {

	//preCandle := candles[signalIdx-1]
	signal := candles[signalIdx]

	tpPct := dynamicTPPercent(candles, signalIdx, lookBack, retraceNext)
	//tpPct := 30.0
	if tpPct < 0 {
		// we will not trade
		return models.Trade{}, false
	}

	distPrice := signal.Body() * (tpPct / 100)
	entry := signal.Close

	var tp, sl float64
	var position models.Position

	if signal.IsGreen() {
		tp = entry - distPrice
		sl = entry + 5*distPrice

		position = models.SellPosition
	} else {
		tp = entry + distPrice
		sl = entry - 5*distPrice

		position = models.BuyPosition
	}

	return simulateTrade(candles, signalIdx, position, entry, tp, sl, margin, fee, leverage)
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

		//percentageFee := models.FixedFeeToPercent(5, 100, leverage)

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

func maxBodyRetracement(candles []models.Candle, idx, n int) float64 {
	c := candles[idx]
	body := c.Body()
	if body == 0 {
		return 0
	}

	maxRetr := 0.0

	for i := idx + 1; i <= idx+n && i < len(candles); i++ {

		var retr float64
		if c.IsGreen() {
			// how much price get lower of close price
			retr = (c.Close - candles[i].Low) / body * 100
		} else {
			// how much price get upper of close price
			retr = (candles[i].High - c.Close) / body * 100
		}
		if retr > maxRetr {
			maxRetr = retr
		}
	}

	if maxRetr < 0 {
		return 0
	}

	//maxRetr = (maxRetr / 100)
	return maxRetr
}

func dynamicTPPercent(candles []models.Candle, signalIdx, lookback, n int) float64 {
	start := signalIdx - lookback
	if start < 0 {
		start = 0
	}

	sum := 0.0
	count := 0

	for j := start; j < signalIdx; j++ {

		if !candles[j].IsMarubozu() && candles[j].BodyPercentage() > 0.3 {
			continue
		}

		r := maxBodyRetracement(candles, j, n)
		if r <= 0 {
			continue
		}

		sum += r
		count++
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// this function check if target candle body is bigger than x percentage of its previous candles
func isCandleBodyBigger(candles []models.Candle, signalIdx int, lookBack int, dominancePercentage float64) bool {

	if signalIdx < lookBack {
		return false
	}

	currentCandle := candles[signalIdx]

	smallerCount := 0
	for i := signalIdx - lookBack; i < signalIdx; i++ {
		if candles[i].Body() < currentCandle.Body() {
			smallerCount++
		}
	}

	ratio := float64(smallerCount) / float64(lookBack) * 100
	return ratio >= dominancePercentage
}

func CalculateCapitalMetrics(trades []models.Trade, allocationPercent float64, initialCapital float64) CapitalMetrics {
	if len(trades) == 0 || initialCapital <= 0 {
		return CapitalMetrics{}
	}

	allocation := allocationPercent / 100.0

	currentCapital := initialCapital
	peakCapital := initialCapital
	maxDrawdown := 0.0
	simpleSum := 0.0

	for _, t := range trades {

		tradeROI := t.GainPercent / 100.0

		simpleSum += tradeROI * allocation

		allocatedMargin := currentCapital * allocation
		profitLoss := allocatedMargin * tradeROI
		currentCapital += profitLoss

		if currentCapital < 0 {
			currentCapital = 0
		}

		if currentCapital > peakCapital {
			peakCapital = currentCapital
		}

		drawdown := (peakCapital - currentCapital) / peakCapital * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return CapitalMetrics{
		SimpleGainPercent:   simpleSum * 100,
		CompoundGainPercent: ((currentCapital - initialCapital) / initialCapital) * 100,
		MaxDrawdownPercent:  maxDrawdown,
		FinalCapital:        currentCapital,
	}
}

func (metric *CapitalMetrics) PrintCapitalMetrics() {
	fmt.Println("========== Metrics Result ==========")
	fmt.Printf("Simple : %.2f\n", metric.SimpleGainPercent)
	fmt.Printf("Compund         : %.2f\n", metric.CompoundGainPercent)
	fmt.Printf("Max Drawdown       : %.2f\n", metric.MaxDrawdownPercent)
	fmt.Printf("Final Capital     : %.2f%%\n", metric.FinalCapital)
	fmt.Println("-------------------------------------")
}
