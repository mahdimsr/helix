package indicators

import "math"

// CalculateFibonacciDirectional محاسبه فیبوناچی با در نظر گرفتن جهت روند
// startPoint: نقطه شروع (مثلاً open کندل)
// endPoint: نقطه پایان (مثلاً close کندل)
// precision: تعداد ارقام اعشار
func CalculateFibonacciDirectional(startPoint, endPoint float64, precision int) []FibLevel {
	// تشخیص جهت روند
	isUptrend := endPoint > startPoint

	var high, low float64
	if isUptrend {
		high = endPoint
		low = startPoint
	} else {
		high = startPoint
		low = endPoint
	}

	diff := high - low

	// نسبت‌های استاندارد فیبوناچی
	ratios := []float64{0.0, 0.236, 0.382, 0.500, 0.618, 0.786, 1.0, 1.618}

	var levels []FibLevel
	multiplier := math.Pow(10, float64(precision))

	for _, r := range ratios {
		var price float64

		if isUptrend {
			// در روند صعودی: 0% در پایین، 100% در بالا
			// Price = Low + (diff * ratio)
			price = low + (diff * r)
		} else {
			// در روند نزولی: 0% در بالا، 100% در پایین
			// Price = High - (diff * ratio)
			price = high - (diff * r)
		}

		roundedPrice := math.Round(price*multiplier) / multiplier

		levels = append(levels, FibLevel{
			Ratio: r,
			Price: roundedPrice,
		})
	}

	return levels
}

// CalculateFibonacci تابع قدیمی (برای سازگاری با کدهای قبلی)
func CalculateFibonacci(high, low float64, precision int) []FibLevel {
	return CalculateFibonacciDirectional(high, low, precision)
}

// FindNearestFibLevel نزدیک‌ترین سطح استاندارد فیبوناچی را بر اساس درصد ورودی پیدا می‌کند
// startPoint: نقطه شروع (مثلاً open کندل)
// endPoint: نقطه پایان (مثلاً close کندل)
// percent: درصد مورد نظر (مثلاً 78 یا 161)
func FindNearestFibLevel(startPoint, endPoint, percent float64, precision int) FibLevel {
	isUptrend := endPoint > startPoint

	var high, low float64
	if isUptrend {
		high = endPoint
		low = startPoint
	} else {
		high = startPoint
		low = endPoint
	}

	ratio := percent / 100.0
	ratios := []float64{0.0, 0.236, 0.382, 0.500, 0.618, 0.786, 1.0, 1.618}

	// پیدا کردن نزدیک‌ترین نسبت به مقدار ورودی
	nearestRatio := ratios[0]
	minDiff := math.Abs(ratio - ratios[0])

	for _, r := range ratios {
		diff := math.Abs(ratio - r)
		if diff < minDiff {
			minDiff = diff
			nearestRatio = r
		}
	}

	// محاسبه قیمت با در نظر گرفتن جهت روند
	diff := high - low
	var price float64
	if isUptrend {
		price = low + (diff * nearestRatio)
	} else {
		price = high - (diff * nearestRatio)
	}

	multiplier := math.Pow(10, float64(precision))
	roundedPrice := math.Round(price*multiplier) / multiplier

	return FibLevel{
		Ratio: nearestRatio,
		Price: roundedPrice,
	}
}

// GetPriceByPercent قیمت را مستقیماً بر اساس هر درصد دلخواهی محاسبه می‌کند
// startPoint: نقطه شروع (مثلاً open کندل)
// endPoint: نقطه پایان (مثلاً close کندل)
// percent: درصد مورد نظر (مثلاً 45 یا 127.2)
func GetPriceByPercent(startPoint, endPoint, percent float64, precision int) float64 {
	isUptrend := endPoint > startPoint

	var high, low float64
	if isUptrend {
		high = endPoint
		low = startPoint
	} else {
		high = startPoint
		low = endPoint
	}

	ratio := percent / 100.0
	diff := high - low

	var price float64
	if isUptrend {
		price = low + (diff * ratio)
	} else {
		price = high - (diff * ratio)
	}

	multiplier := math.Pow(10, float64(precision))
	return math.Round(price*multiplier) / multiplier
}
