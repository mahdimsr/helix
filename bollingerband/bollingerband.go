package bollingerband

import (
	"helix/models"
	"math"
)

type BollingerBand struct {
	Upper  float64 `json:"upper"`
	Middle float64 `json:"middle"` // همان Basis یا EMA
	Lower  float64 `json:"lower"`
}

func CalculateBollingerBandsEMA(candles []models.Candle, length int, multiplier float64) []BollingerBand {
	if len(candles) == 0 || length <= 0 {
		return nil
	}

	result := make([]BollingerBand, len(candles))
	closes := make([]float64, len(candles))
	for i, c := range candles {
		closes[i] = c.Close
	}

	// محاسبه EMA برای تمام قیمت‌های بسته شدن
	emas := calculateEMA(closes, length)

	for i := range candles {
		// اگر تعداد داده‌ها برای تشکیل یک پنجره کامل به اندازه length کافی نباشد، مقدار NaN برمی‌گردانیم
		// این رفتار مشابه تابع ta.stdev در Pine Script است که در کندل‌های اولیه مقدار na می‌دهد.
		if i < length-1 {
			result[i] = BollingerBand{
				Upper:  math.NaN(),
				Middle: math.NaN(),
				Lower:  math.NaN(),
			}
			continue
		}

		middle := emas[i]
		stdev := calculatePopulationStdev(closes, i, length)

		result[i] = BollingerBand{
			Upper:  middle + (multiplier * stdev),
			Middle: middle,
			Lower:  middle - (multiplier * stdev),
		}
	}

	return result
}

// calculateEMA میانگین متحرک نمایی (Exponential Moving Average) را محاسبه می‌کند.
func calculateEMA(closes []float64, length int) []float64 {
	emas := make([]float64, len(closes))
	if len(closes) == 0 {
		return emas
	}

	// ضریب هموارسازی (Smoothing Factor)
	k := 2.0 / float64(length+1)
	emas[0] = closes[0]

	for i := 1; i < len(closes); i++ {
		emas[i] = (closes[i] * k) + (emas[i-1] * (1 - k))
	}

	return emas
}

// calculatePopulationStdev انحراف معیار جامعه (Population Standard Deviation) را برای پنجره مشخص‌شده محاسبه می‌کند.
// این تابع دقیقاً مشابه رفتار پیش‌فرض ta.stdev در Pine Script عمل می‌کند (تقسیم بر n به جای n-1).
func calculatePopulationStdev(closes []float64, currentIndex int, length int) float64 {
	start := currentIndex - length + 1
	window := closes[start : currentIndex+1]

	// محاسبه میانگین (Mean)
	sum := 0.0
	for _, val := range window {
		sum += val
	}
	mean := sum / float64(length)

	// محاسبه مجموع مربعات اختلاف از میانگین
	varianceSum := 0.0
	for _, val := range window {
		diff := val - mean
		varianceSum += diff * diff
	}

	// واریانس جامعه (تقسیم بر length) و سپس جذر گرفتن برای انحراف معیار
	variance := varianceSum / float64(length)
	return math.Sqrt(variance)
}
