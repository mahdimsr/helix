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
			trades, netGain, winCount := runSingleBacktest(htfCandles, ltfCandles, initialCapital, leverage, tpUSD, slUSD)

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
) ([]models.Trade, float64, int) { // 🆕 مقدار برگشتی سوم: تعداد تریدهای برنده

	var trades []models.Trade
	currentCapital := initialCapital
	winCount := 0 // 🆕 شمارنده تریدهای برنده

	for _, htf := range htfCandles {
		if currentCapital <= 0 {
			break
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

		// 🆕 شمارش ترید برنده
		if status == "TP" {
			winCount++
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
	return trades, netGain, winCount // 🆕 برگرداندن winCount
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
