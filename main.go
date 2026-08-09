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

	candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-07-23", "2026-08-01")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	results := strategy.RunPulseOptimization(candles)
	bestResult := results[0]

	nexCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-08-02", "2026-08-07")

	cfg := strategy.PulseStrategyConfig{
		BodyThresholdPercent: bestResult.Config.BodyThresholdPercent, // بادی بیشتر از 0.1%
		TPBodyRatio:          bestResult.Config.TPBodyRatio,          // tp روی x درصد از بادی کندل
		SLMultiplier:         bestResult.Config.SLMultiplier,         // حد ضرر ۵ برابر فاصله حد سود
		AllocationPercent:    50,                                     // درصد سرمایه در هر معامله
		Leverage:             bestResult.Config.Leverage,             // لوریج 0
		FeePercent:           0,                                      // کارمزد ۰.۱٪
		InitialCapital:       1000,                                   // سرمایه اولیه ۱۰۰۰
		MaxConcurrentTrades:  2,
	}

	result := strategy.PulseStrategy(nexCandles, cfg)

	result.PrintResult()
}
