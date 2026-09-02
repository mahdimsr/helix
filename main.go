package main

import (
	"context"
	"fmt"
	"helix/chart"
	"helix/database"
	"helix/strategy"
	"os"
	"time"
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

	nexCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-08-24", "2026-08-29")
	lowCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "5m", "2026-08-24", "2026-08-29")

	tpDollars := makeRange(1.0, 100.0, 1.0)
	slDollars := makeRange(1.0, 50.0, 1.0)

	results := strategy.RunBacktest(
		nexCandles,
		lowCandles,
		100.0, // سرمایه اولیه 1000 دلار
		100.0, // لوریج 10
		tpDollars,
		slDollars,
	)

	//strategy.PrintCombinedMatrix(results)

	err := chart.GenerateInteractiveChart(results, "chart.html")
	chartURL, err := chart.GenerateShortChartURL(results)
	if err != nil {
		fmt.Println("خطا در تولید نمودار:", err)
		return
	}

	fmt.Println("\n✅ لینک نمودار تعاملی شما آماده است:")
	//fmt.Println(chartURL)

	filename := "chart_link.txt"

	// تبدیل رشته به بایت و ذخیره با دسترسی خواندن/نوشتن (0644)
	err = os.WriteFile(filename, []byte(chartURL), 0644)
	if err != nil {
		fmt.Println("❌ خطا در ذخیره فایل:", err)
		return
	}

	fmt.Println("\n(این لینک را کپی کرده و در مرورگر خود باز کنید)")

	time.Sleep(1 * time.Second)

	topResults := results.FilterByGainThreshold(90)

	err = chart.GenerateInteractiveChart(topResults, "top.html")
	if err != nil {
		fmt.Println("خطا در تولید نمودار:", err)
		return
	}

	/*fmt.Println("--- ماتریس سود خالص (سطرها: TP / ستون‌ها: SL) ---")

	fmt.Printf("%-8s |", "TP \\ SL")
	for _, sl := range results.SLValues {
		fmt.Printf("%8.0f ", sl)
	}
	fmt.Println()

	fmt.Println(strings.Repeat("-", 9+len(results.SLValues)*9))

	for i, tp := range results.TPValues {
		fmt.Printf("TP $%-4d |", int(tp))
		for j := range results.SLValues {
			// فرمت‌دهی: اگر عدد منفی بود با رنگ/علامت مشخص، اگر مثبت بود با فاصله مناسب
			fmt.Printf("%8.2f ", results.GainMatrix[i][j])
		}
		fmt.Println()
	}*/

	/*targetTP := 6.0
	targetSL := 50.0
	key := fmt.Sprintf("TP_%v_SL_%v", targetTP, targetSL)

	fmt.Printf("\n--- لیست تریدها برای %s ---\n", key)
	specificTrades := results.TradeLogs[key]
	for _, t := range specificTrades {

		openTime := time.UnixMilli(t.OpenTime).UTC().Format("2006-01-02 15:04")
		closeTime := time.UnixMilli(t.CloseTime).UTC().Format("2006-01-02 15:04")

		fmt.Printf("[%s] Entry: %.2f %s | Exit: %.2f %s | Result: %s | PnL%%: %.2f%%\n",
			t.Type, t.OpenPrice, openTime, t.ClosePrice, closeTime, t.Status, t.GainPercent)
	}*/

}

func makeRange(from, to, step float64) []float64 {

	var result []float64
	for i := from; i <= to; i += step {
		result = append(result, i)
	}
	return result
}
