package main

import (
	"context"
	"helix/database"
	"helix/strategy"
)

func main() {

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	/*candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-08-19", "2026-08-20")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	results := strategy.RunPulseOptimization(candles)
	bestResult := results[1]
	cfg := strategy.PulseStrategyConfig{
		BodyThresholdPercent:    bestResult.Config.BodyThresholdPercent,    // بادی بیشتر از 0.1%
		BodyThresholdPercentMax: bestResult.Config.BodyThresholdPercentMax, // بادی بیشتر از 0.1%
		TPBodyRatio:             bestResult.Config.TPBodyRatio,             // tp روی x درصد از بادی کندل
		SLMultiplier:            bestResult.Config.SLMultiplier,            // حد ضرر ۵ برابر فاصله حد سود
		AllocationPercent:       bestResult.Config.AllocationPercent,       // درصد سرمایه در هر معامله
		Leverage:                bestResult.Config.Leverage,                // لوریج 0
		FeePercent:              0,                                         // کارمزد ۰.۱٪
		InitialCapital:          1000,                                      // سرمایه اولیه ۱۰۰۰
		MaxConcurrentTrades:     int(100 / bestResult.Config.AllocationPercent),
	}*/

	nexCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-08-20", "2026-08-21")

	cfg := strategy.PulseStrategyConfig{
		BodyThresholdPercent:    0.0, // بادی بیشتر از 0.1%
		BodyThresholdPercentMax: 0.5, // بادی بیشتر از 0.1%
		TPBodyRatio:             7,   // tp روی x درصد از بادی کندل
		SLMultiplier:            10,  // حد ضرر ۵ برابر فاصله حد سود
		AllocationPercent:       50,  // درصد سرمایه در هر معامله
		Leverage:                10,  // لوریج 0
		FeePercent:              0,   // کارمزد ۰.۱٪
		InitialCapital:          600, // سرمایه اولیه ۱۰۰۰
		MaxConcurrentTrades:     2,
	}

	result := strategy.PulseStrategy(nexCandles, cfg)

	result.PrintResult()
}
