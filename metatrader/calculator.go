package metatrader

import (
	"math"
	"strings"
)

func CalculateTargetPrice(entryPrice, volumeUSD, targetUSD float64, symbol string, side string) float64 {
	if volumeUSD <= 0 {
		return 0 // حجم دلاری باید عددی مثبت باشد
	}

	upperSymbol := strings.ToUpper(symbol)
	var targetPrice float64

	// تشخیص اینکه آیا ارز پایه (Base Currency) دلار است یا خیر
	// مثال: USDJPY, USDCHF (ارز پایه USD است)
	// مثال: EURUSD, XAUUSD, BTCUSD (ارز مظنه (Quote) USD است)
	isBaseUSD := strings.HasPrefix(upperSymbol, "USD") && len(upperSymbol) == 6

	if !isBaseUSD {
		// حالت اول: Quote = USD (مثل EURUSD, GBPUSD, XAUUSD, BTCUSD)
		// تعداد واحد ارز پایه = volumeUSD / entryPrice
		// فرمول سود: Profit = (TargetPrice - EntryPrice) * (volumeUSD / entryPrice)
		// با بازآرایی فرمول برای TargetPrice:

		priceDiff := (targetUSD * entryPrice) / volumeUSD

		if side == "BUY" {
			targetPrice = entryPrice + priceDiff
		} else {
			targetPrice = entryPrice - priceDiff
		}
	} else {
		// حالت دوم: Base = USD (مثل USDJPY, USDCHF)
		// تعداد واحد ارز پایه (که خود دلار است) = volumeUSD
		// فرمول سود: Profit = (TargetPrice - EntryPrice) * volumeUSD / TargetPrice
		// با حل معادله ریاضی برای TargetPrice:

		if side == "BUY" {
			denominator := volumeUSD - targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // جلوگیری از تقسیم بر صفر
			}
			targetPrice = (entryPrice * volumeUSD) / denominator
		} else {
			denominator := volumeUSD + targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // جلوگیری از تقسیم بر صفر
			}
			targetPrice = (entryPrice * volumeUSD) / denominator
		}
	}

	return targetPrice
}

func CalculateLotSize(margin, leverage, price, contractSize float64, symbol string) float64 {
	// ۱. محاسبه ارزش کل پوزیشن
	positionValue := margin * leverage

	// ۲. محاسبه مقدار واحد (مثلاً چند بیت‌کوین یا چند یورو)
	quantity := positionValue / price

	// ۳. تبدیل به لات
	lot := quantity / contractSize

	// ۴. گرد کردن به 2 رقم اعشار (استاندارد اکثر بروکرها)
	lot = lot * 100 / 100

	return lot
}

func CalculateMaxVolume(accountBalance, currentPrice, leverage, contractSize float64) (maxLots, maxVolumeUSD float64) {
	if accountBalance <= 0 || leverage <= 0 || currentPrice <= 0 || contractSize <= 0 {
		return 0, 0
	}

	// ۱. محاسبه حداکثر حجم دلاری (Notional Value)
	// این عددی است که در تابع قبلی (CalculateTargetPriceByVolume) به عنوان volumeUSD استفاده می‌شد
	maxVolumeUSD = accountBalance * leverage

	// ۲. محاسبه ارزش واقعی یک لات کامل
	notionalValuePerLot := currentPrice * contractSize

	// ۳. محاسبه مارجین مورد نیاز برای یک لات کامل
	marginPerLot := notionalValuePerLot / leverage

	// ۴. محاسبه حداکثر تعداد لات
	if marginPerLot > 0 {
		maxLots = accountBalance / marginPerLot
	}

	return maxLots, maxVolumeUSD
}
