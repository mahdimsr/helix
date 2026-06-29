package indicators

import (
	"helix/models"
	"math"
	"strconv"
)

func BacktestUTBot(candles []models.Candle, sensitivity float64, atrPeriod int) []models.Trade {
	n := len(candles)
	if n == 0 {
		return nil
	}

	trs := make([]float64, n)
	atrs := make([]float64, n)
	var prevStop float64

	var trades []models.Trade
	var currentTrade *models.Trade

	// آلفا برای محاسبه RMA
	alpha := 1.0 / float64(atrPeriod)
	sum := 0.0

	for i := 0; i < n; i++ {
		// --- 1. استخراج قیمت‌های کندل فعلی ---
		high, _ := strconv.ParseFloat(candles[i].High, 64)
		low, _ := strconv.ParseFloat(candles[i].Low, 64)
		closePrice, _ := strconv.ParseFloat(candles[i].Close, 64)
		time := candles[i].Time

		// --- 2. آپدیت مقادیر Runup و Risk برای ترید باز فعلی ---
		// این کار قبل از بررسی خروج انجام می‌شود تا نوسانات کندل فعلی روی ترید باز اعمال شود
		if currentTrade != nil {
			if currentTrade.Type == "Long" {
				// Runup برای لانگ (بالاترین قیمت)
				if high > currentTrade.RunupPrice {
					currentTrade.RunupPrice = high
					currentTrade.RunupTime = time
					currentTrade.RunupPercent = ((high - currentTrade.OpenPrice) / currentTrade.OpenPrice) * 100
					currentTrade.RunupDuration = time - currentTrade.OpenTime
				}
				// Risk برای لانگ (پایین‌ترین قیمت)
				if low < currentTrade.RiskPrice {
					currentTrade.RiskPrice = low
					currentTrade.RiskTime = time
					currentTrade.RiskPercent = ((low - currentTrade.OpenPrice) / currentTrade.OpenPrice) * 100
					currentTrade.RiskDuration = time - currentTrade.OpenTime
				}
			} else { // Short
				// Runup برای شورت (پایین‌ترین قیمت)
				if low < currentTrade.RunupPrice {
					currentTrade.RunupPrice = low
					currentTrade.RunupTime = time
					currentTrade.RunupPercent = ((currentTrade.OpenPrice - low) / currentTrade.OpenPrice) * 100
					currentTrade.RunupDuration = time - currentTrade.OpenTime
				}
				// Risk برای شورت (بالاترین قیمت)
				if high > currentTrade.RiskPrice {
					currentTrade.RiskPrice = high
					currentTrade.RiskTime = time
					currentTrade.RiskPercent = ((currentTrade.OpenPrice - high) / currentTrade.OpenPrice) * 100
					currentTrade.RiskDuration = time - currentTrade.OpenTime
				}
			}
		}

		// --- 3. محاسبات اندیکاتور UT Bot ---
		if i == 0 {
			trs[i] = high - low
			sum += trs[i]
			atrs[i] = sum
			prevStop = closePrice
			continue
		}

		prevClose, _ := strconv.ParseFloat(candles[i-1].Close, 64)
		trs[i] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

		if i < atrPeriod {
			sum += trs[i]
			atrs[i] = sum / float64(i+1)
		} else {
			atrs[i] = alpha*trs[i] + (1.0-alpha)*atrs[i-1]
		}

		nLoss := sensitivity * atrs[i]
		var stop float64

		if prevClose > prevStop && closePrice > prevStop {
			stop = math.Max(prevStop, closePrice-nLoss)
		} else if prevClose < prevStop && closePrice < prevStop {
			stop = math.Min(prevStop, closePrice+nLoss)
		} else if closePrice > prevStop {
			stop = closePrice - nLoss
		} else {
			stop = closePrice + nLoss
		}

		above := prevClose <= prevStop && closePrice > stop
		below := prevStop <= prevClose && stop > closePrice

		buySignal := closePrice > stop && above
		sellSignal := closePrice < stop && below

		prevStop = stop
		// ذخیره استاپ برای دور بعد

		// --- 4. منطق باز و بسته کردن معاملات ---

		// اگر سیگنال خرید داریم
		if buySignal {
			// اگر ترید شورت باز است، آن را می‌بندیم
			if currentTrade != nil && currentTrade.Type == "Short" {
				currentTrade.ClosePrice = closePrice
				currentTrade.CloseTime = time
				currentTrade.Duration = time - currentTrade.OpenTime
				trades = append(trades, *currentTrade)
				currentTrade = nil
			}
			// باز کردن ترید لانگ جدید
			if currentTrade == nil {
				currentTrade = &models.Trade{
					Type:        "Long",
					OpenPrice:   closePrice,
					OpenTime:    time,
					RunupPrice:  closePrice, // مقداردهی اولیه
					RiskPrice:   closePrice, // مقداردهی اولیه
					ATR:         atrPeriod,
					Sensitivity: sensitivity,
				}
			}
		}

		// اگر سیگنال فروش داریم
		if sellSignal {
			// اگر ترید لانگ باز است، آن را می‌بندیم
			if currentTrade != nil && currentTrade.Type == "Long" {
				currentTrade.ClosePrice = closePrice
				currentTrade.CloseTime = time
				currentTrade.Duration = time - currentTrade.OpenTime
				trades = append(trades, *currentTrade)
				currentTrade = nil
			}
			// باز کردن ترید شورت جدید
			if currentTrade == nil {
				currentTrade = &models.Trade{
					Type:        "Short",
					OpenPrice:   closePrice,
					OpenTime:    time,
					RunupPrice:  closePrice, // مقداردهی اولیه
					RiskPrice:   closePrice, // مقداردهی اولیه
					ATR:         atrPeriod,
					Sensitivity: sensitivity,
				}
			}
		}
	}

	// بستن آخرین ترید باز در انتهای داده‌ها (اختیاری)
	if currentTrade != nil {
		lastCandle := candles[n-1]
		currentTrade.ClosePrice, _ = strconv.ParseFloat(lastCandle.Close, 64)
		currentTrade.CloseTime = lastCandle.Time
		currentTrade.Duration = lastCandle.Time - currentTrade.OpenTime
		trades = append(trades, *currentTrade)
	}

	return trades
}
