package strategy

import (
	"fmt"
	"helix/indicators"
	"helix/models"
	"time"
)

func CalculateSignal(candles []models.Candle, ATR int, Sensitivity int) indicators.Signal {

	//signal := indicators.GetLatestUTBotSignal(candles, float64(Sensitivity), ATR)
	trades := indicators.BacktestUTBot(candles, float64(Sensitivity), ATR)

	var offset int64 = 3600 * 3
	trade := trades[len(trades)-1]
	t := time.Unix(trade.OpenTime-offset, 0).UTC().Format("2006-01-02 15:04:05")
	test := time.Unix(trade.OpenTime-offset, 0).UTC()
	now := time.Now().UTC()
	diffInMinutes := now.Sub(test).Abs().Minutes()

	fmt.Printf("Trade | open:%.4f type:%s time:%s timestamP: %d \n", trade.OpenPrice, trade.Type, t, trade.OpenTime)
	fmt.Printf("NOW | %s \n", now.Format("2006-01-02 15:04:05"))
	fmt.Printf("DIFF | %3.f \n", diffInMinutes)

	if diffInMinutes <= 10 {

		if trade.Type == "Long" {
			return indicators.BuySignal
		} else {
			return indicators.SellSignal
		}

	} else {
		return indicators.NoneSignal
	}
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
