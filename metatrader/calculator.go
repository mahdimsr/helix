package metatrader

import (
	"math"
	"strings"
)

func CalculateTargetPrice(entryPrice, lot, targetUSD float64, symbol string, side string) float64 {
	var contractSize float64
	var isQuoteUSD bool

	upperSymbol := strings.ToUpper(symbol)

	// ۱. تشخیص ContractSize و نوع جفت‌ارز
	if strings.Contains(upperSymbol, "XAU") || strings.Contains(upperSymbol, "GOLD") {
		contractSize = 100.0 // طلا: هر لات = 100 اونس
		isQuoteUSD = true    // XAUUSD (Quote = USD)
	} else if strings.HasPrefix(upperSymbol, "USD") && len(upperSymbol) == 6 {
		contractSize = 100000.0 // جفت‌ارزهای استاندارد
		isQuoteUSD = false      // USDJPY, USDCHF (Base = USD)
	} else if strings.Contains(upperSymbol, "BTC") || strings.Contains(upperSymbol, "BITCOIN") {
		contractSize = 1.0 // بیت‌کوین: هر لات = 1 واحد بیت‌کوین (در 99% بروکرها)
		isQuoteUSD = true
	} else {
		contractSize = 100000.0 // EURUSD, GBPUSD, etc.
		isQuoteUSD = true       // Quote = USD
	}

	var targetPrice float64

	// ۲. محاسبه بر اساس نوع جفت‌ارز
	if isQuoteUSD {
		// برای EURUSD, GBPUSD, XAUUSD
		// فرمول: Profit = (ExitPrice - EntryPrice) × Lot × ContractSize
		priceDiff := targetUSD / (lot * contractSize)

		if side == "BUY" {
			targetPrice = entryPrice + priceDiff
		} else {
			targetPrice = entryPrice - priceDiff
		}
	} else {
		// برای USDJPY, USDCHF
		// فرمول: Profit = (ExitPrice - EntryPrice) × Lot × ContractSize / ExitPrice
		// این معادله را برای ExitPrice حل می‌کنیم

		if side == "BUY" {
			denominator := (lot * contractSize) - targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // خطا: تقسیم بر صفر
			}
			targetPrice = (entryPrice * lot * contractSize) / denominator
		} else {
			denominator := (lot * contractSize) + targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // خطا: تقسیم بر صفر
			}
			targetPrice = (entryPrice * lot * contractSize) / denominator
		}
	}

	return targetPrice
}
