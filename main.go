package main

import (
	"context"
	"fmt"
	"helix/database"
	"helix/indicators"
	"log"
)

func main() {

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	symbol := "BTCUSDT"
	timeframe := "15m"
	startTime := "2026-09-01"
	endTime := "2026-09-20"
	tpPercent := 0.3
	slPercent := 0.8

	candles, err := candlesRepo.Fetch(context.Background(), symbol, timeframe, startTime, endTime)
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	optimizedResults := indicators.OptimizeUTBot(candles, tpPercent, slPercent)

	for _, result := range optimizedResults {
		fmt.Printf("ATR=%d Sens=%.0f Gain=%.2f%% Trades=%d WinRate=%.1f%%\n",
			result.ATR, result.Sensitivity, result.TotalGain, result.TradeCount, result.WinRate)
	}

	newCandles, newerr := candlesRepo.Fetch(context.Background(), symbol, timeframe, "2026-09-01", "2026-09-25")
	if newerr != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	details := indicators.BacktestUTBotDetail(newCandles, 8, 40, tpPercent, slPercent)
	fmt.Printf("Trads count: %d\n", len(details.Trades))
	for _, t := range details.Trades {
		duration := t.ExitTime.Sub(t.EntryTime)
		fmt.Printf("%s | %s → %s (%s) | Entry: %.2f | Exit: %.2f | %s | %.2f%%\n",
			t.TradeType,
			t.EntryTime.Format("2006-01-02 15:04"),
			t.ExitTime.Format("2006-01-02 15:04"),
			duration,
			t.EntryPrice, t.ExitPrice,
			t.ExitReason, t.GainPercent)
	}
}
