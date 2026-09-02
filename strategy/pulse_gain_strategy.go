package strategy

import (
	"fmt"
	"helix/models"
	"sort"
	"strings"
)

type BacktestOutput struct {
	TPValues         []float64
	SLValues         []float64
	GainMatrix       [][]float64               // ماتریس سود خالص برای رسم نمودار
	CountMatrix      [][]int                   // ماتریس تعداد تریدها برای رسم نمودار
	TradeLogs        map[string][]models.Trade // دیکشنری برای دسترسی به لیست تریدهای هر حالت
	WinRateMatrix    [][]float64
	LiquidatedMatrix [][]bool
}

type MaxGainPoint struct {
	TP      float64
	SL      float64
	Gain    float64
	IndexTP int
	IndexSL int
}

func RunBacktest(
	htfCandles []models.Candle,
	ltfCandles []models.Candle,
	initialCapital float64,
	leverage float64,
	tpValues []float64,
	slValues []float64,
) *BacktestOutput {

	sort.Slice(htfCandles, func(i, j int) bool { return htfCandles[i].Time < htfCandles[j].Time })
	sort.Slice(ltfCandles, func(i, j int) bool { return ltfCandles[i].Time < ltfCandles[j].Time })

	output := &BacktestOutput{
		TPValues:      tpValues,
		SLValues:      slValues,
		GainMatrix:    make([][]float64, len(tpValues)),
		CountMatrix:   make([][]int, len(tpValues)),
		WinRateMatrix: make([][]float64, len(tpValues)), // 🆕
		TradeLogs:     make(map[string][]models.Trade),
	}

	for i := 0; i < len(tpValues); i++ {
		output.GainMatrix[i] = make([]float64, len(slValues))
		output.CountMatrix[i] = make([]int, len(slValues))
		output.WinRateMatrix[i] = make([]float64, len(slValues)) // 🆕
	}

	for i, tpUSD := range tpValues {
		for j, slUSD := range slValues {
			trades, netGain, winCount, _ := runSingleBacktest(htfCandles, ltfCandles, initialCapital, leverage, tpUSD, slUSD, 0)

			output.GainMatrix[i][j] = netGain
			output.CountMatrix[i][j] = len(trades)

			// 🆕 محاسبه Win Rate
			if len(trades) > 0 {
				output.WinRateMatrix[i][j] = (float64(winCount) / float64(len(trades))) * 100
			} else {
				output.WinRateMatrix[i][j] = 0
			}

			key := fmt.Sprintf("TP_%v_SL_%v", tpUSD, slUSD)
			output.TradeLogs[key] = trades
		}
	}

	return output
}

// --- منطق داخلی بررسی یک ترکیب خاص ---
func runSingleBacktest(
	htfCandles []models.Candle,
	ltfCandles []models.Candle,
	initialCapital float64,
	leverage float64,
	tpUSD float64,
	slUSD float64,
	liquidationThreshold float64,
) ([]models.Trade, float64, int, bool) {

	var trades []models.Trade
	currentCapital := initialCapital
	winCount := 0
	liquidated := false

	liquidationLevel := initialCapital * liquidationThreshold

	// 🆕 متغیر برای ذخیره زمان بسته شدن آخرین ترید
	var lastCloseTime int64 = 0

	for _, htf := range htfCandles {
		if currentCapital <= liquidationLevel {
			liquidated = true
			break
		}

		// 🛑 چک کردن همپوشانی: اگر زمان این کندل قبل از بسته شدن ترید قبلی است، نادیده بگیر
		if htf.Time < lastCloseTime {
			continue
		}

		if !htf.IsMarubozu() {
			continue
		}

		var tradeType string
		if htf.IsGreen() {
			tradeType = "Short"
		} else if htf.IsRed() {
			tradeType = "Long"
		} else {
			continue
		}

		entryPrice := htf.Close
		if entryPrice == 0 {
			continue
		}
		entryTime := htf.Time

		tradeCapital := currentCapital
		quantity := (tradeCapital * leverage) / entryPrice

		priceDistTP := tpUSD / quantity
		priceDistSL := slUSD / quantity

		var tpPrice, slPrice float64
		if tradeType == "Long" {
			tpPrice = entryPrice + priceDistTP
			slPrice = entryPrice - priceDistSL
		} else {
			tpPrice = entryPrice - priceDistTP
			slPrice = entryPrice + priceDistSL
		}

		startIdx := sort.Search(len(ltfCandles), func(i int) bool {
			return ltfCandles[i].Time > entryTime
		})

		var status string
		var exitPrice float64
		var exitTime int64
		mfe := 0.0
		mae := 0.0
		mfeTime := entryTime
		maeTime := entryTime

		for i := startIdx; i < len(ltfCandles); i++ {
			ltf := ltfCandles[i]
			hitTP := false
			hitSL := false

			if tradeType == "Long" {
				profit := ltf.High - entryPrice
				loss := entryPrice - ltf.Low
				if profit > mfe {
					mfe = profit
					mfeTime = ltf.Time
				}
				if loss > mae {
					mae = loss
					maeTime = ltf.Time
				}

				if ltf.High >= tpPrice {
					hitTP = true
				}
				if ltf.Low <= slPrice {
					hitSL = true
				}
			} else {
				profit := entryPrice - ltf.Low
				loss := ltf.High - entryPrice
				if profit > mfe {
					mfe = profit
					mfeTime = ltf.Time
				}
				if loss > mae {
					mae = loss
					maeTime = ltf.Time
				}

				if ltf.Low <= tpPrice {
					hitTP = true
				}
				if ltf.High >= slPrice {
					hitSL = true
				}
			}

			if hitTP && hitSL {
				status = "SL"
				exitPrice = slPrice
				exitTime = ltf.Time
				break
			} else if hitTP {
				status = "TP"
				exitPrice = tpPrice
				exitTime = ltf.Time
				break
			} else if hitSL {
				status = "SL"
				exitPrice = slPrice
				exitTime = ltf.Time
				break
			}
		}

		if status == "" {
			if len(ltfCandles) > 0 {
				lastLtf := ltfCandles[len(ltfCandles)-1]
				exitPrice = lastLtf.Close
				exitTime = lastLtf.Time
				status = "TIME_EXIT"
			} else {
				continue
			}
		}

		var pnl float64
		if tradeType == "Long" {
			pnl = (exitPrice - entryPrice) * quantity
		} else {
			pnl = (entryPrice - exitPrice) * quantity
		}

		currentCapital += pnl

		// 🛠️ به‌روزرسانی زمان بسته شدن برای جلوگیری از همپوشانی در دور بعدی حلقه
		lastCloseTime = exitTime

		if status == "TP" {
			winCount++
		}

		if currentCapital <= liquidationLevel {
			liquidated = true
			trade := models.Trade{ /* ... (همان کد قبلی برای LIQUIDATED) ... */ }
			trades = append(trades, trade)
			break
		}

		trade := models.Trade{
			Type:          tradeType,
			OpenPrice:     entryPrice,
			ClosePrice:    exitPrice,
			Duration:      exitTime - entryTime,
			OpenTime:      entryTime,
			CloseTime:     exitTime,
			RunupPrice:    mfe,
			RunupPercent:  (mfe / entryPrice) * 100,
			RunupTime:     mfeTime,
			RunupDuration: mfeTime - entryTime,
			RiskPrice:     mae,
			RiskPercent:   (mae / entryPrice) * 100,
			RiskTime:      maeTime,
			RiskDuration:  maeTime - entryTime,
			GainPercent:   (pnl / tradeCapital) * 100,
			Status:        status,
			Tp:            tpUSD,
			Sl:            slUSD,
		}
		trades = append(trades, trade)
	}

	netGain := currentCapital - initialCapital
	return trades, netGain, winCount, liquidated
}

func PrintCombinedMatrix(results *BacktestOutput) {
	fmt.Println("\n--- ماتریس ترکیبی: Gain (تعداد ترید) ---")
	fmt.Println("(سطر = TP / ستون = SL)")

	// چاپ هدر ستون‌ها (SL)
	fmt.Printf("%-8s |", "TP \\ SL")
	for _, sl := range results.SLValues {
		fmt.Printf("%12.0f ", sl)
	}
	fmt.Println()

	// خط جداکننده
	fmt.Println(strings.Repeat("-", 9+len(results.SLValues)*13))

	// چاپ سطرها
	for i, tp := range results.TPValues {
		fmt.Printf("TP $%-4d |", int(tp))
		for j := range results.SLValues {
			gain := results.GainMatrix[i][j]
			count := results.CountMatrix[i][j]
			// فرمت: Gain (Count)
			fmt.Printf("%7.0f(%3d) ", gain, count)
		}
		fmt.Println()
	}
}

func (results *BacktestOutput) FindMaxGainPoint() *MaxGainPoint {
	maxGain := -float64(1<<63 - 1) // کوچکترین مقدار ممکن
	maxPoint := &MaxGainPoint{}

	for i, tp := range results.TPValues {
		for j, sl := range results.SLValues {
			gain := results.GainMatrix[i][j]
			if gain > maxGain {
				maxGain = gain
				maxPoint.TP = tp
				maxPoint.SL = sl
				maxPoint.Gain = gain
				maxPoint.IndexTP = i
				maxPoint.IndexSL = j
			}
		}
	}

	return maxPoint
}

func (results *BacktestOutput) FilterByGainThreshold(threshold float64) *BacktestOutput {
	if threshold < 0 || threshold > 100 {
		threshold = 90
	}

	threshold = threshold / 100

	// پیدا کردن Max Gain
	maxPoint := results.FindMaxGainPoint()
	thresholdValue := maxPoint.Gain * threshold

	// ساخت خروجی جدید
	filtered := &BacktestOutput{
		TPValues:         results.TPValues,
		SLValues:         results.SLValues,
		GainMatrix:       make([][]float64, len(results.TPValues)),
		CountMatrix:      make([][]int, len(results.TPValues)),
		WinRateMatrix:    make([][]float64, len(results.TPValues)),
		LiquidatedMatrix: make([][]bool, len(results.TPValues)),
		TradeLogs:        make(map[string][]models.Trade),
	}

	// مقداردهی اولیه ماتریس‌ها
	for i := range results.TPValues {
		filtered.GainMatrix[i] = make([]float64, len(results.SLValues))
		filtered.CountMatrix[i] = make([]int, len(results.SLValues))
		filtered.WinRateMatrix[i] = make([]float64, len(results.SLValues))
	}

	// فیلتر کردن نقاط
	filteredCount := 0
	for i, tp := range results.TPValues {
		for j, sl := range results.SLValues {
			gain := results.GainMatrix[i][j]

			// اگر gain >= threshold، این نقطه را نگه دار
			if gain >= thresholdValue {
				filtered.GainMatrix[i][j] = gain
				filtered.CountMatrix[i][j] = results.CountMatrix[i][j]
				filtered.WinRateMatrix[i][j] = results.WinRateMatrix[i][j]

				key := fmt.Sprintf("TP_%v_SL_%v", tp, sl)
				filtered.TradeLogs[key] = results.TradeLogs[key]

				filteredCount++
			}
			// در غیر این صورت، مقادیر صفر باقی می‌مانند
		}
	}

	fmt.Printf("🔍 فیلتر 90%% Max Gain:\n")
	fmt.Printf("   Max Gain: $%.2f (TP=%.0f, SL=%.0f)\n", maxPoint.Gain, maxPoint.TP, maxPoint.SL)
	fmt.Printf("   Threshold: $%.2f (90%% of Max)\n", thresholdValue)
	fmt.Printf("   نقاط فیلتر شده: %d از %d\n", filteredCount, len(results.TPValues)*len(results.SLValues))

	return filtered
}
