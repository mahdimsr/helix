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

	candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-06-01", "2026-06-30")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	results := strategy.RunPulseOptimization(candles)
	bestResult := results[0]

	cfg := strategy.PulseStrategyConfig{
		BodyThresholdPercent: bestResult.Config.BodyThresholdPercent, // بادی بیشتر از 0.1%
		TPBodyRatio:          bestResult.Config.TPBodyRatio,          // tp روی x درصد از بادی کندل
		SLMultiplier:         bestResult.Config.SLMultiplier,         // حد ضرر ۵ برابر فاصله حد سود
		AllocationPercent:    bestResult.Config.AllocationPercent,    // درصد سرمایه در هر معامله
		Leverage:             bestResult.Config.Leverage,             // لوریج 0
		FeePercent:           0,                                      // کارمزد ۰.۱٪
		InitialCapital:       1000,                                   // سرمایه اولیه ۱۰۰۰
		MaxConcurrentTrades:  2,                                      // حداکثر  معامله همزمان
	}

	result := strategy.PulseStrategy(candles, cfg)

	result.PrintResult()
}
