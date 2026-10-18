package main

import (
	"context"
	"helix/database"
	"helix/walking"
	"log"
	"time"
)

func main() {

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	layout := "2006-01-02 15:04:05"

	startTimeObj, err := time.Parse(layout, "2026-10-14 00:00:00")
	if err != nil {
		log.Fatalln(err)
	}

	endTimeObj, err := time.Parse(layout, "2026-10-17 00:00:00")
	if err != nil {
		log.Fatalln(err)
	}

	highCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "15m", "2026-02-01", "2026-10-18")
	lowCandles, _ := candlesRepo.Fetch(context.Background(), "BTCUSDT", "5m", "2026-02-01", "2026-10-18")

	tpDollars := makeRange(65.0, 65.0, 1.0)
	slDollars := makeRange(1.0, 1.0, 1.0)

	result := walking.WalkForward(
		highCandles,
		15,
		lowCandles,
		startTimeObj.Unix(),
		endTimeObj.Unix(),
		1,
		1,
		700.0,     // سرمایه اولیه
		10.0,      // اهرم
		tpDollars, // TP ثابت: 9 دلار
		slDollars, // SL ثابت: 13 دلار
		3,
	)

	// چاپ تریدها
	walking.PrintWalkForwardTrades(result)

	/*optResult := walking.OptimizeWalkForwardParameters(
		highCandles,
		lowCandles,
		startTimeObj.Unix(),
		endTimeObj.Unix(),
		windowDaysRange,
		lookbackDaysRange,
		1000.0,
		10.0,
		tpDollars,
		slDollars,
	)

	fmt.Printf("\n✅ بهترین ترکیب پیدا شد:\n")
	fmt.Printf("   Window=%d روز, Lookback=%d روز\n",
		optResult.BestWindowDays, optResult.BestLookbackDays)
	fmt.Printf("   سود=$%.2f, WinRate=%.1f%%\n",
		optResult.BestTotalPnL, optResult.BestWinRate)*/

	/*//log.Fatal("Here")
	/*results := strategy.RunBacktest(
		highCandles,
		lowCandles,
		1000.0, // سرمایه اولیه 1000 دلار
		10.0,   // لوریج 10
		tpDollars,
		slDollars,
	)
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

	fmt.Println("\n(این لینک را کپی کرده و در مرورگر خود باز کنید)")*/

	/*

		// statics
			trades := walking.ExecuteTradesWithStaticTPSL(
				highCandles,
				lowCandles,
				50.0, // سرمایه اولیه
				10.0, // اهرم
				2.0,  // TP ثابت: 9 دلار
				5.0,  // SL ثابت: 13 دلار
			)

			// چاپ تریدها
			walking.PrintStaticTrades(trades)








		results := strategy.RunBacktest(
			highCandles,
			lowCandles,
			100.0, // سرمایه اولیه 1000 دلار
			100.0, // لوریج 10
			tpDollars,
			slDollars,
		)


		err = chart.GenerateInteractiveChart(results, "chart.html")
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

		scoredResults := topResults.ScoreResults()
		strategy.PrintScoredResults(scoredResults)*/

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
