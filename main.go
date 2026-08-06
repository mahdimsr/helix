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

	candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-08-01", "2026-08-30")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	result := strategy.PulseStrategy(candles, 10, 1000, 10)

	result.PrintResult()
}
