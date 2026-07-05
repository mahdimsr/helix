package strategy

import "helix/models"

func CalculateSignal(candles []models.Candle) string {

	// only return LONG or SHORT
	return "LONG"
}

func CalculateOrderUtils(currentPrice float64, side string) (float64, float64, float64) {

	// amount, tp, sl
	amount := 1 / 3
	tp := calculateTP(currentPrice, side, 0.3)
	sl := calculateSL(currentPrice, side, 0.3)

	return float64(amount), tp, sl
}

func calculateTP(price float64, side string, percentage float64) float64 {

	offset := price * (percentage / 100.0)
	if side == "LONG" {
		return price + offset
	}
	return price - offset
}

func calculateSL(price float64, side string, percentage float64) float64 {

	offset := price * (percentage / 100.0)
	if side == "LONG" {
		return price - offset
	}
	return price + offset
}
