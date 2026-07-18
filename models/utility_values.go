package models

type Position string

const (
	BuyPosition  Position = "BUY"
	SellPosition Position = "SELL"
	NonePosition Position = "NONE"
)

func CalcGainPercent(volumeToUSD, entry, exit float64, leverage int, fee float64, isLong bool) float64 {
	lots := CalculateLots(volumeToUSD, leverage, entry)
	commission := CalculateCommission(lots, fee)

	var priceDiff float64
	if isLong {
		priceDiff = exit - entry
	} else {
		priceDiff = entry - exit
	}

	grossProfit := priceDiff * lots

	netProfit := grossProfit - commission

	gainPercent := (netProfit / volumeToUSD) * 100.0

	return gainPercent
}

func CalculateLots(margin float64, leverage int, assetPrice float64) float64 {
	if assetPrice == 0 {
		return 0
	}
	totalPositionValue := margin * float64(leverage)
	lots := totalPositionValue / assetPrice
	return lots
}

func CalculateCommission(lots float64, oneWayCommission float64) float64 {
	roundTurnCommission := oneWayCommission * 2
	return lots * roundTurnCommission
}
