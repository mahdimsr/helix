package strategy

import "helix/models"

func CalculateSignal(candles []models.Candle) string {

	// only return BUY or SELL
	return "BUY"
}

func CalculateOrderUtils(currentPrice float64, side string) (float64, float64, float64) {

	// amount, tp, sl
	amount := 0.1
	tp := calculateTP(currentPrice, side, 0.5)
	sl := calculateSL(currentPrice, side, 0.5)

	return float64(amount), tp, sl
}

func calculateTP(price float64, side string, percentage float64) float64 {

	offset := price * (percentage / 100.0)
	if side == "BUY" {
		return price + offset
	}
	return price - offset
}

func calculateSL(price float64, side string, percentage float64) float64 {

	offset := price * (percentage / 100.0)
	if side == "BUY" {
		return price - offset
	}
	return price + offset
}
