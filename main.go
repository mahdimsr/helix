package main

import (
	"context"
	"fmt"
	"helix/database"
	"helix/indicators"
	"log"
	"sort"
)

func main() {

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	candles, err := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-06-01", "2026-06-30")
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	optimizedResult := indicators.OptimizeDualUTBot(candles)
	sort.Slice(optimizedResult, func(i, j int) bool {
		return optimizedResult[i].TotalGainPerc > optimizedResult[j].TotalGainPerc
	})

	for _, result := range optimizedResult {
		fmt.Printf("optimized result| CondATR:%d CondSens:%d SignAtr:%d SignSens:%d Trades:%d Wintrate:%.2f Gain:%.2f \n",
			int64(result.CondATR),
			int64(result.CondSens),
			int64(result.SigATR),
			int64(result.SigSens),
			result.TotalTrades,
			result.WinRate,
			result.TotalGainPerc,
		)
	}
}
