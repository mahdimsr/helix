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

	symbol := "BTCUSDT"
	timeframe := "15m"
	startTime := "2026-08-01"
	endTime := "2026-09-01"

	candles, err := candlesRepo.Fetch(context.Background(), symbol, timeframe, startTime, endTime)
	if err != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	var allResults []indicators.OptimizationResult

	for tp := 0.01; tp <= 5.00; tp += 0.05 {
		// حلقه برای SL از 0.01 تا 2.00 با فاصله 0.05
		for sl := 0.01; sl <= 2.00; sl += 0.05 {
			// اگه SL کمتر از TP باشه، skip کن (منطقی نیست)
			if sl <= tp {
				continue
			}

			// محاسبه برای این جفت TP/SL
			optimizedResults := indicators.OptimizeUTBot(candles, tp, sl)

			// اگه نتیجه‌ای بود، ذخیره کن
			if len(optimizedResults) > 0 {
				for _, result := range optimizedResults {
					allResults = append(allResults, indicators.OptimizationResult{
						SigATR:     result.ATR,
						SigSens:    result.Sensitivity,
						TPPercent:  tp,
						SLPercent:  sl,
						TotalGain:  result.TotalGain,
						TradeCount: result.TradeCount,
						WinRate:    result.WinRate,
					})
				}
			}
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		scoreI := allResults[i].WinRate*0.6 + allResults[i].TotalGain*0.4
		scoreJ := allResults[j].WinRate*0.6 + allResults[j].TotalGain*0.4
		return scoreI > scoreJ
	})

	fmt.Println("\n=== TOP 10 CONFIGURATIONS ===")
	fmt.Printf("%-5s | %-8s | %-8s | %-10s | %-10s | %-8s | %s | %s\n",
		"Rank", "TP%", "SL%", "WinRate%", "Gain%", "Trades", "Atr", "Sens")
	fmt.Println("--------------------------------------------------------------")

	topN := 10
	if len(allResults) < topN {
		topN = len(allResults)
	}

	for i := 0; i < topN; i++ {
		r := allResults[i]
		fmt.Printf("%-5d | %-8.2f | %-8.2f | %-10.2f | %-10.2f | %-8d | %d | %.2f\n",
			i+1, r.TPPercent, r.SLPercent, r.WinRate, r.TotalGain, r.TradeCount, r.SigATR, r.SigSens)
	}

	// اگه خواستی همه نتایج رو ببینی
	fmt.Printf("\nTotal configurations tested: %d\n", len(allResults))

	/*newCandles, newerr := candlesRepo.Fetch(context.Background(), symbol, timeframe, "2026-09-01", "2026-09-20")
	if newerr != nil {
		log.Fatalf("Error getching candle: %d", err)
	}

	details := indicators.BacktestUTBotDetail(newCandles, 7, 25, 0.30, 1.95)
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
	}*/
}
