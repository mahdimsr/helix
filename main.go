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

	cfg := strategy.PulseStrategyConfig{
		BodyThresholdPercent: 0.2,  // بادی بیشتر از 0.1%
		TPBodyRatio:          0.5,  // tp روی x درصد از بادی کندل
		SLMultiplier:         5,    // حد ضرر ۵ برابر فاصله حد سود
		AllocationPercent:    30,   // ۱۰٪ سرمایه در هر معامله
		Leverage:             1,    // لوریج 0
		FeePercent:           0,    // کارمزد ۰.۱٪
		InitialCapital:       1000, // سرمایه اولیه ۱۰۰۰
		MaxConcurrentTrades:  3,    // حداکثر ۱۰ معامله همزمان
	}

	result := strategy.PulseStrategy(candles, cfg)

	result.PrintResult()
}
