package walking

import (
	"fmt"
	"helix/models"
	"helix/strategy"
	"sort"
	"time"
)

type WalkForwardTrade struct {
	EntryTime   int64   `json:"entryTime"`
	ExitTime    int64   `json:"exitTime"`
	Type        string  `json:"type"` // Long / Short
	EntryPrice  float64 `json:"entryPrice"`
	ExitPrice   float64 `json:"exitPrice"`
	TP          float64 `json:"tp"` // دلاری
	SL          float64 `json:"sl"` // دلاری
	PnL         float64 `json:"pnl"`
	Status      string  `json:"status"` // TP / SL / TIME_EXIT
	WindowStart int64   `json:"windowStart"`
	WindowEnd   int64   `json:"windowEnd"`
}

type WalkForwardResult struct {
	Trades      []WalkForwardTrade `json:"trades"`
	TotalPnL    float64            `json:"totalPnL"`
	WinCount    int                `json:"winCount"`
	TotalTrades int                `json:"totalTrades"`
	WinRate     float64            `json:"winRate"`
}

type Window struct {
	Start int64
	End   int64
}

func filterCandlesByTime(candles []models.Candle, start, end int64) []models.Candle {
	// 🆕 نرمال‌سازی ورودی‌ها
	start = NormalizeTimestamp(start)
	end = NormalizeTimestamp(end)

	var filtered []models.Candle
	for _, c := range candles {
		cTime := NormalizeTimestamp(c.Time)
		if cTime >= start && cTime < end {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func filterLTFCandlesForTrading(ltfCandles []models.Candle, entryTime, windowEnd int64) []models.Candle {
	var filtered []models.Candle
	for _, c := range ltfCandles {
		if c.Time > entryTime && c.Time <= windowEnd {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func executeTradesWithFixedTPSL(
	htfWindow []models.Candle,
	ltfAll []models.Candle,
	initialCapital float64,
	leverage float64,
	tpUSD float64,
	slUSD float64,
	windowEnd int64,
) []WalkForwardTrade {

	windowEnd = NormalizeTimestamp(windowEnd)
	htfWindow = NormalizeCandleTimes(htfWindow)
	ltfAll = NormalizeCandleTimes(ltfAll)

	var trades []WalkForwardTrade
	currentCapital := initialCapital

	// مرتب‌سازی کندل‌های HTF
	sort.Slice(htfWindow, func(i, j int) bool {
		return htfWindow[i].Time < htfWindow[j].Time
	})

	// مرتب‌سازی کندل‌های LTF
	sort.Slice(ltfAll, func(i, j int) bool {
		return ltfAll[i].Time < ltfAll[j].Time
	})

	ltfTimestamps := make([]int64, len(ltfAll))
	for i, c := range ltfAll {
		ltfTimestamps[i] = c.Time
	}

	var lastCloseTime int64 = 0

	for _, htf := range htfWindow {
		if currentCapital <= 0 {
			break
		}

		// جلوگیری از همپوشانی تریدها
		if htf.Time < lastCloseTime {
			continue
		}

		// شرط ورود
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

		quantity := (currentCapital * leverage) / entryPrice
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

		// پیدا کردن کندل‌های LTF بعد از ورود و قبل از پایان پنجره
		ltfForTrade := filterLTFCandlesForTrading(ltfAll, entryTime, windowEnd)

		var status string
		var exitPrice float64
		var exitTime int64

		for _, ltf := range ltfForTrade {
			hitTP := false
			hitSL := false

			if tradeType == "Long" {
				if ltf.High >= tpPrice {
					hitTP = true
				}
				if ltf.Low <= slPrice {
					hitSL = true
				}
			} else {
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

		// اگر تا پایان پنجره بسته نشد
		if status == "" {
			if len(ltfForTrade) > 0 {
				lastLtf := ltfForTrade[len(ltfForTrade)-1]
				exitPrice = lastLtf.Close
				exitTime = lastLtf.Time
			} else {
				exitPrice = entryPrice
				exitTime = entryTime
			}
			status = "TIME_EXIT"
		}

		var pnl float64
		if tradeType == "Long" {
			pnl = (exitPrice - entryPrice) * quantity
		} else {
			pnl = (entryPrice - exitPrice) * quantity
		}

		currentCapital += pnl
		lastCloseTime = exitTime

		trades = append(trades, WalkForwardTrade{
			EntryTime:  entryTime,
			ExitTime:   exitTime,
			Type:       tradeType,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			TP:         tpUSD,
			SL:         slUSD,
			PnL:        pnl,
			Status:     status,
		})
	}

	return trades
}

func WalkForward(
	htfCandles []models.Candle,
	ltfCandles []models.Candle,
	startTime int64,
	endTime int64,
	windowDays int,
	initialCapital float64,
	leverage float64,
	tpRange []float64,
	slRange []float64,
) *WalkForwardResult {

	htfCandles = NormalizeCandleTimes(htfCandles)
	ltfCandles = NormalizeCandleTimes(ltfCandles)
	startTime = NormalizeTimestamp(startTime)
	endTime = NormalizeTimestamp(endTime)

	// 🆕 دیباگ: نمایش بازه زمانی دیتا
	DebugTimeRange(htfCandles, "HTF Candles (High TF)")
	DebugTimeRange(ltfCandles, "LTF Candles (Low TF)")
	fmt.Printf("🎯 بازه درخواستی: %s → %s\n",
		time.Unix(startTime, 0).Format("2006-01-02 15:04:05"),
		time.Unix(endTime, 0).Format("2006-01-02 15:04:05"))

	windowSeconds := int64(windowDays * 24 * 3600)

	// ساخت پنجره‌ها
	var windows []Window
	for t := startTime; t < endTime; t += windowSeconds {
		windowEnd := t + windowSeconds
		if windowEnd > endTime {
			windowEnd = endTime
		}
		windows = append(windows, Window{Start: t, End: windowEnd})
	}

	if len(windows) < 2 {
		fmt.Println("⚠️ حداقل 2 پنجره زمانی نیاز است. بازه زمانی را بزرگ‌تر کنید.")
		return &WalkForwardResult{}
	}

	fmt.Printf("\n🚀 شروع پیمایش پنجره‌ای:\n")
	fmt.Printf("   بازه: %s تا %s\n",
		time.Unix(startTime, 0).Format("2006-01-02"),
		time.Unix(endTime, 0).Format("2006-01-02"))
	fmt.Printf("   تعداد پنجره‌ها: %d (هر کدام %d روز)\n", len(windows), windowDays)
	fmt.Println("─────────────────────────────────────────────────────────")

	var allTrades []WalkForwardTrade
	var bestTP, bestSL float64
	currentCapital := initialCapital

	for i, win := range windows {
		htfWindow := filterCandlesByTime(htfCandles, win.Start, win.End)
		ltfWindow := filterCandlesByTime(ltfCandles, win.Start, win.End)

		winStartStr := time.Unix(win.Start, 0).Format("2006-01-02")
		winEndStr := time.Unix(win.End, 0).Format("2006-01-02")

		if i == 0 {
			// پنجره اول: فقط محاسبه بهترین ترکیب (بدون ترید)
			fmt.Printf("\n📐 پنجره %d [%s → %s]: محاسبه بهترین ترکیب...\n", i+1, winStartStr, winEndStr)

			if len(htfWindow) == 0 || len(ltfWindow) == 0 {
				fmt.Println("   ⚠️ دیتای کافی نیست، رد شد.")
				continue
			}

			results := strategy.RunBacktest(htfWindow, ltfWindow, initialCapital, leverage, tpRange, slRange)
			scored := results.ScoreResults()

			if len(scored) > 0 {
				bestTP = scored[0].TP
				bestSL = scored[0].SL
				fmt.Printf("   ✅ بهترین ترکیب: TP=$%.0f, SL=$%.0f (Score: %.3f)\n",
					bestTP, bestSL, scored[0].TotalScore)
			} else {
				fmt.Println("   ⚠️ هیچ ترکیب معتبری پیدا نشد.")
			}
			continue
		}

		// پنجره‌های بعدی: ترید + محاسبه ترکیب جدید
		fmt.Printf("\n📈 پنجره %d [%s → %s]:\n", i+1, winStartStr, winEndStr)

		// بخش 1: ترید با بهترین ترکیب پنجره قبلی
		if bestTP > 0 && bestSL > 0 && len(htfWindow) > 0 && len(ltfWindow) > 0 {
			fmt.Printf("   🔄 ترید با ترکیب قبلی: TP=$%.0f, SL=$%.0f\n", bestTP, bestSL)

			trades := executeTradesWithFixedTPSL(
				htfWindow, ltfWindow, currentCapital, leverage, bestTP, bestSL, win.End)

			// ثبت WindowStart و WindowEnd برای هر ترید
			windowPnL := 0.0
			windowWins := 0
			for idx := range trades {
				trades[idx].WindowStart = win.Start
				trades[idx].WindowEnd = win.End
				windowPnL += trades[idx].PnL
				currentCapital += trades[idx].PnL
				if trades[idx].Status == "TP" {
					windowWins++
				}
			}

			fmt.Printf("   📊 %d ترید | سود پنجره: $%.2f | برد: %d\n",
				len(trades), windowPnL, windowWins)

			allTrades = append(allTrades, trades...)
		} else {
			fmt.Println("   ⚠️ ترکیب بهینه قبلی موجود نیست، ترید انجام نشد.")
		}

		// بخش 2: محاسبه بهترین ترکیب جدید از این پنجره
		if len(htfWindow) > 0 && len(ltfWindow) > 0 {
			results := strategy.RunBacktest(htfWindow, ltfWindow, initialCapital, leverage, tpRange, slRange)
			scored := results.ScoreResults()

			if len(scored) > 0 {
				bestTP = scored[0].TP
				bestSL = scored[0].SL
				fmt.Printf("   🧮 بهترین ترکیب جدید: TP=$%.0f, SL=$%.0f (Score: %.3f)\n",
					bestTP, bestSL, scored[0].TotalScore)
			} else {
				fmt.Println("   ⚠️ ترکیب جدیدی پیدا نشد، ترکیب قبلی حفظ می‌شود.")
			}
		}
	}

	// محاسبه آمار نهایی
	result := &WalkForwardResult{
		Trades:      allTrades,
		TotalTrades: len(allTrades),
	}

	for _, t := range allTrades {
		result.TotalPnL += t.PnL
		if t.Status == "TP" {
			result.WinCount++
		}
	}

	if result.TotalTrades > 0 {
		result.WinRate = (float64(result.WinCount) / float64(result.TotalTrades)) * 100
	}

	// خلاصه نهایی
	fmt.Println("\n═════════════════════════════════════════════════════════")
	fmt.Println("📋 خلاصه نهایی پیمایش پنجره‌ای:")
	fmt.Printf("   کل تریدها: %d\n", result.TotalTrades)
	fmt.Printf("   تریدهای برنده: %d\n", result.WinCount)
	fmt.Printf("   Win Rate: %.1f%%\n", result.WinRate)
	fmt.Printf("   سود خالص کل: $%.2f\n", result.TotalPnL)
	fmt.Printf("   سرمایه نهایی: $%.2f\n", initialCapital+result.TotalPnL)
	fmt.Println("═════════════════════════════════════════════════════════")

	return result
}

func PrintWalkForwardTrades(result *WalkForwardResult) {
	if len(result.Trades) == 0 {
		fmt.Println("هیچ تریدی ثبت نشده است.")
		return
	}

	fmt.Printf("\n📜 لیست تریدهای پیمایش پنجره‌ای (%d ترید):\n", len(result.Trades))
	fmt.Println("┌─────┬────────────────────┬────────┬──────────┬──────────┬────────┬────────┬──────────┬────────────┐")
	fmt.Println("│  #  │       Time         │  Type  │  Entry   │   Exit   │   TP   │   SL   │   PnL    │   Status   │")
	fmt.Println("├─────┼────────────────────┼────────┼──────────┼──────────┼────────┼────────┼──────────┼────────────┤")

	for i, t := range result.Trades {
		timeStr := time.Unix(t.EntryTime, 0).Format("2006-01-02 15:04")
		fmt.Printf("│ %3d │ %s │ %-6s │ %8.2f │ %8.2f │ $%5.0f │ $%4.0f │ $%7.2f │ %-10s │\n",
			i+1,
			timeStr,
			t.Type,
			t.EntryPrice,
			t.ExitPrice,
			t.TP,
			t.SL,
			t.PnL,
			t.Status,
		)
	}
	fmt.Println("└─────┴────────────────────┴────────┴──────────┴──────────┴────────┴────────┴──────────┴────────────┘")
}

func NormalizeTimestamp(ts int64) int64 {
	// اگر عدد بزرگ‌تر از سال 3000 به ثانیه باشد، قطعاً میلی‌ثانیه است
	// 32503680000 = 1 ژانویه 3000 به ثانیه
	if ts > 32503680000 {
		return ts / 1000 // تبدیل میلی‌ثانیه به ثانیه
	}
	return ts
}

func NormalizeCandleTimes(candles []models.Candle) []models.Candle {
	normalized := make([]models.Candle, len(candles))
	copy(normalized, candles)
	for i := range normalized {
		normalized[i].Time = NormalizeTimestamp(normalized[i].Time)
	}
	return normalized
}

// DebugTimeRange بازه زمانی دیتا را نمایش می‌دهد (برای دیباگ)
func DebugTimeRange(candles []models.Candle, label string) {
	if len(candles) == 0 {
		fmt.Printf("⚠️ %s: هیچ کندلی وجود ندارد\n", label)
		return
	}
	first := candles[0]
	last := candles[len(candles)-1]
	fmt.Printf("🔍 %s:\n", label)
	fmt.Printf("   تعداد کندل‌ها: %d\n", len(candles))
	fmt.Printf("   اولین: %s (Time=%d)\n", time.Unix(first.Time, 0).Format("2006-01-02 15:04:05"), first.Time)
	fmt.Printf("   آخرین: %s (Time=%d)\n", time.Unix(last.Time, 0).Format("2006-01-02 15:04:05"), last.Time)
}
