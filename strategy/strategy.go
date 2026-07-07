package strategy

import (
	"helix/indicators"
	"helix/models"
)

func CalculateSignal(candles []models.Candle, ATR int, Sensitivity int) indicators.Signal {

	signal := indicators.GetLatestUTBotSignal(candles, float64(Sensitivity), ATR)

	return signal
}

func CalculateOrderUtils(currentPrice float64, side string) (float64, float64, float64) {

	// amount, tp, sl
	amount := 0.1
	tp := calculateTP(currentPrice, side, 0.2)
	sl := calculateSL(currentPrice, side, 0.2)

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
