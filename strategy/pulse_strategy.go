package strategy

import "helix/models"

func PulseStrategy(candles []models.Candle) models.BackTest {

	var backtest models.BackTest

	for i := 1; i < len(candles); i++ {
		c := candles[i]

		if !c.IsMarubozu() {
			continue
		}

		prev := candles[i-1]
		entry := c.Close
		tp := (prev.High + prev.Low) / 2
		var sl float64
		var position models.Position

		switch {
		case c.IsGreen():
			position = models.SellPosition
			if tp >= entry {
				continue
			}
			dist := entry - tp
			sl = entry + 4*dist

		default:
			position = models.BuyPosition
			if tp <= entry {
				continue
			}
			dist := tp - entry
			sl = entry - 4*dist
		}

		trade, closed := SimulatePulseTrade(candles, i, position, entry, tp, sl)
		if closed {
			backtest.Trades = append(backtest.Trades, trade)
		}
	}

	backtest.Calculate()

	return backtest
}

func SimulatePulseTrade(candles []models.Candle, entryIdx int, position models.Position, price, tp, sl float64) (models.Trade, bool) {

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
			t.GainPercent = (exitPrice - price) / price * 100
			t.RunupPercent = (runupPrice - price) / price * 100
			t.RiskPercent = (riskPrice - price) / price * 100 // معمولاً منفی
		} else {
			t.GainPercent = (price - exitPrice) / price * 100
			t.RunupPercent = (price - runupPrice) / price * 100
			t.RiskPercent = (price - riskPrice) / price * 100 // معمولاً منفی
		}

		return t, true
	}

	return models.Trade{}, false
}
