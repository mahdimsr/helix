package strategy

import (
	"helix/models"
)

func PulseStrategy(candles []models.Candle) models.BackTest {

	var backtest models.BackTest

	for i := 1; i < len(candles); i++ {
		c := candles[i]

		if !c.IsMarubozu() {
			continue
		}

		if !isCandleBodyBigger(candles, i, 100, 60) {
			continue
		}

		trade, closed := SimulatePulseTrade(candles, i, 100, 5, 1, 0.1)
		if closed {
			backtest.Trades = append(backtest.Trades, trade)
		}
	}

	backtest.Calculate()

	return backtest
}

func SimulatePulseTrade(candles []models.Candle, signalIdx int, lookBack, retraceNext, leverage int, fee float64) (models.Trade, bool) {

	//preCandle := candles[signalIdx-1]
	signal := candles[signalIdx]

	tpPct := dynamicTPPercent(candles, signalIdx, lookBack, retraceNext)
	if tpPct == 0 {
		// we will not trade
		return models.Trade{}, false
	}

	distPrice := signal.Body() * (tpPct / 100)
	entry := signal.Close

	var tp, sl float64
	var position models.Position

	if signal.IsGreen() {
		tp = entry - distPrice
		sl = entry + 3*distPrice

		position = models.SellPosition
	} else {
		tp = entry + distPrice
		sl = entry - 3*distPrice

		position = models.BuyPosition
	}

	return simulateTrade(candles, signalIdx, position, entry, tp, sl, fee, leverage)
}

func simulateTrade(candles []models.Candle, entryIdx int, position models.Position, price, tp, sl, fee float64, leverage int) (models.Trade, bool) {

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

		percentageFee := models.FixedFeeToPercent(5, 100, leverage)

		if position == models.BuyPosition {
			t.GainPercent = models.CalcGainPercent(price, exitPrice, leverage, percentageFee, true)
			t.RunupPercent = (runupPrice - price) / price * 100
			t.RiskPercent = (riskPrice - price) / price * 100 // معمولاً منفی
		} else {
			t.GainPercent = models.CalcGainPercent(price, exitPrice, leverage, percentageFee, false)
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
