package main

import (
	"context"
	"helix/database"
	"helix/strategy"
	"log"
)

func main() {

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-06-05", "2026-06-30")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	backtest := strategy.PulseStrategy(candles)

	backtest.PrintBacktest()
}
