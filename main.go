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

	symbol := "XAUTUSDT"
	timeframe := "5m"
	startTime := "2026-07-20"
	endTime := "2026-08-02"
	tpPercent := 0.2
	slPercent := 2.0

	candles, err := candlesRepo.Fetch(context.Background(), symbol, timeframe, startTime, endTime)
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	optimizedResults := indicators.OptimizeUTBot(candles, tpPercent, slPercent)
	sort.Slice(optimizedResults, func(i, j int) bool {
		return optimizedResults[i].TotalGain > optimizedResults[j].TotalGain
	})

	for _, result := range optimizedResults {
		fmt.Printf("ATR=%d Sens=%.0f Gain=%.2f%% Trades=%d WinRate=%.1f%%\n",
			result.ATR, result.Sensitivity, result.TotalGain, result.TradeCount, result.WinRate)
	}

	newCandles, newerr := candlesRepo.Fetch(context.Background(), symbol, timeframe, startTime, endTime)
	if newerr != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	details := indicators.BacktestUTBotDetail(newCandles, 4, 80, tpPercent, slPercent)
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
