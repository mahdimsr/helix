package walking_forward_ratio

import (
	"fmt"
	"helix/models"
	"helix/strategy"
	"helix/walking"
	"sort"
	"time"
)

// BodyGroup یک گروه از کندل‌ها بر اساس درصد Body
type BodyGroup struct {
	Index      int     `json:"index"`
	MinPercent float64 `json:"minPercent"`
	MaxPercent float64 `json:"maxPercent"`
	AvgBody    float64 `json:"avgBody"` // 🆕 میانگین بدنه این گروه
	BestTP     float64 `json:"bestTP"`
	BestSL     float64 `json:"bestSL"`
	BestScore  float64 `json:"bestScore"`
	TradeCount int     `json:"tradeCount"`
	WinCount   int     `json:"winCount"`
	WinRate    float64 `json:"winRate"`
	TotalPnL   float64 `json:"totalPnL"`
}

// WindowGroups گروه‌های یک پنجره زمانی
type WindowGroups struct {
	WindowStart int64
	WindowEnd   int64
	Groups      [5]BodyGroup
}

// GetBodyGroupIndex درصد Body کندل را محاسبه کرده و ایندکس گروه را برمی‌گرداند
func GetBodyGroupIndex(candle *models.Candle) int {
	bodyPercent := candle.BodyPercentage()

	if bodyPercent >= 1.0 {
		return 4
	} else if bodyPercent >= 0.8 {
		return 3
	} else if bodyPercent >= 0.6 {
		return 2
	} else if bodyPercent >= 0.4 {
		return 1
	} else if bodyPercent >= 0.2 {
		return 0
	}
	return -1
}

// GetGroupRange محدوده درصدی یک گروه را برمی‌گرداند
func GetGroupRange(groupIdx int) (float64, float64) {
	switch groupIdx {
	case 0:
		return 0.2, 0.4
	case 1:
		return 0.4, 0.6
	case 2:
		return 0.6, 0.8
	case 3:
		return 0.8, 1.0
	case 4:
		return 1.0, 100.0
	default:
		return 0, 0
	}
}

// filterByBodyGroup کندل‌های HTF را بر اساس گروه Body فیلتر می‌کند
func filterByBodyGroup(candles []models.Candle, groupIdx int) []models.Candle {
	var filtered []models.Candle
	for _, c := range candles {
		if GetBodyGroupIndex(&c) == groupIdx {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// calculateAverageBody میانگین بدنه کندل‌ها را محاسبه می‌کند
func calculateAverageBody(candles []models.Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	total := 0.0
	for _, c := range candles {
		total += c.Body()
	}
	return total / float64(len(candles))
}

// WalkForwardWithBodyGroups پیمایش پنجره‌ای با دسته‌بندی بر اساس Body Percentage
func WalkForwardWithBodyGroups(
	htfCandles []models.Candle,
	ltfCandles []models.Candle,
	startTime int64,
	endTime int64,
	windowDays int,
	initialCapital float64,
	leverage float64,
	tpRatios []float64, // 🆕 نسبت‌های TP به Body (مثلاً 0.1, 0.2, 0.3)
	slRatios []float64, // 🆕 نسبت‌های SL به Body
) *walking.WalkForwardResult {

	htfCandles = walking.NormalizeCandleTimes(htfCandles)
	ltfCandles = walking.NormalizeCandleTimes(ltfCandles)
	startTime = walking.NormalizeTimestamp(startTime)
	endTime = walking.NormalizeTimestamp(endTime)

	windowSeconds := int64(windowDays * 24 * 3600)

	var windows []walking.Window
	for t := startTime; t < endTime; t += windowSeconds {
		windowEnd := t + windowSeconds
		if windowEnd > endTime {
			windowEnd = endTime
		}
		windows = append(windows, walking.Window{Start: t, End: windowEnd})
	}

	if len(windows) < 2 {
		fmt.Println("⚠️ حداقل 2 پنجره زمانی نیاز است.")
		return &walking.WalkForwardResult{}
	}

	fmt.Printf("\n🚀 شروع پیمایش پنجره‌ای با گروه‌بندی Body:\n")
	fmt.Printf("   بازه: %s تا %s\n",
		time.Unix(startTime, 0).Format("2006-01-02"),
		time.Unix(endTime, 0).Format("2006-01-02"))
	fmt.Printf("   تعداد پنجره‌ها: %d (هر کدام %d روز)\n", len(windows), windowDays)
	fmt.Printf("   نسبت‌های TP: %v\n", tpRatios)
	fmt.Printf("   نسبت‌های SL: %v\n", slRatios)
	fmt.Println("─────────────────────────────────────────────────────────")

	var allTrades []walking.WalkForwardTrade
	var currentWindowGroups WindowGroups
	currentCapital := initialCapital

	for i, win := range windows {
		htfWindow := filterCandlesByTime(htfCandles, win.Start, win.End)
		ltfWindow := filterCandlesByTime(ltfCandles, win.Start, win.End)

		winStartStr := time.Unix(win.Start, 0).Format("2006-01-02")
		winEndStr := time.Unix(win.End, 0).Format("2006-01-02")

		if i == 0 {
			fmt.Printf("\n📐 پنجره %d [%s → %s]: محاسبه بهترین ترکیب برای هر گروه...\n", i+1, winStartStr, winEndStr)

			if len(htfWindow) == 0 || len(ltfWindow) == 0 {
				fmt.Println("   ⚠️ دیتای کافی نیست، رد شد.")
				continue
			}

			currentWindowGroups = calculateBestCombinationsByGroup(htfWindow, ltfWindow, initialCapital, leverage, tpRatios, slRatios)
			printGroupCombinations(currentWindowGroups)
			continue
		}

		fmt.Printf("\n📈 پنجره %d [%s → %s]:\n", i+1, winStartStr, winEndStr)

		if len(htfWindow) > 0 && len(ltfWindow) > 0 {
			fmt.Printf("   🔄 ترید با ترکیب‌های گروه‌بندی شده...\n")

			trades := executeTradesWithBodyGroups(
				htfWindow, ltfWindow, currentCapital, leverage, currentWindowGroups, win.End)

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
			fmt.Println("   ⚠️ دیتای کافی نیست، ترید انجام نشد.")
		}

		if len(htfWindow) > 0 && len(ltfWindow) > 0 {
			currentWindowGroups = calculateBestCombinationsByGroup(htfWindow, ltfWindow, initialCapital, leverage, tpRatios, slRatios)
			fmt.Printf("   🧮 ترکیب‌های جدید برای هر گروه محاسبه شد.\n")
			printGroupCombinations(currentWindowGroups)
		}
	}

	result := &walking.WalkForwardResult{
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

	fmt.Println("\n═════════════════════════════════════════════════════════")
	fmt.Println("📋 خلاصه نهایی پیمایش پنجره‌ای با گروه‌بندی:")
	fmt.Printf("   کل تریدها: %d\n", result.TotalTrades)
	fmt.Printf("   تریدهای برنده: %d\n", result.WinCount)
	fmt.Printf("   Win Rate: %.1f%%\n", result.WinRate)
	fmt.Printf("   سود خالص کل: $%.2f\n", result.TotalPnL)
	fmt.Printf("   سرمایه نهایی: $%.2f\n", initialCapital+result.TotalPnL)
	fmt.Println("═════════════════════════════════════════════════════════")

	return result
}

func filterCandlesByTime(candles []models.Candle, start, end int64) []models.Candle {
	// 🆕 نرمال‌سازی ورودی‌ها
	start = walking.NormalizeTimestamp(start)
	end = walking.NormalizeTimestamp(end)

	var filtered []models.Candle
	for _, c := range candles {
		cTime := walking.NormalizeTimestamp(c.Time)
		if cTime >= start && cTime < end {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func calculateBestCombinationsByGroup(
	htfWindow []models.Candle,
	ltfWindow []models.Candle,
	initialCapital float64,
	leverage float64,
	tpRange []float64,
	slRange []float64,
) WindowGroups {
	var groups WindowGroups

	for groupIdx := 0; groupIdx < 5; groupIdx++ {
		minPercent, maxPercent := GetGroupRange(groupIdx)

		groups.Groups[groupIdx] = BodyGroup{
			Index:      groupIdx,
			MinPercent: minPercent,
			MaxPercent: maxPercent,
		}

		// فیلتر کندل‌های HTF بر اساس گروه Body
		filteredHTF := filterByBodyGroup(htfWindow, groupIdx)

		if len(filteredHTF) == 0 {
			continue
		}

		// اجرای بک‌تست برای این گروه
		results := strategy.RunBacktest(filteredHTF, ltfWindow, initialCapital, leverage, tpRange, slRange)
		scored := results.ScoreResults()

		if len(scored) > 0 {
			best := scored[0]
			groups.Groups[groupIdx].BestTP = best.TP
			groups.Groups[groupIdx].BestSL = best.SL
			groups.Groups[groupIdx].BestScore = best.TotalScore
		}
	}

	return groups
}

func printGroupCombinations(groups WindowGroups) {
	fmt.Println("   📊 ترکیب‌های بهترین برای هر گروه Body:")
	for _, g := range groups.Groups {
		if g.BestTP > 0 && g.BestSL > 0 {
			fmt.Printf("      گروه %.1f%%-%.1f%%: TP=$%.0f, SL=$%.0f (Score: %.3f)\n",
				g.MinPercent, g.MaxPercent, g.BestTP, g.BestSL, g.BestScore)
		} else {
			fmt.Printf("      گروه %.1f%%-%.1f%%: ترکیب معتبری پیدا نشد\n",
				g.MinPercent, g.MaxPercent)
		}
	}
}

func executeTradesWithBodyGroups(
	htfWindow []models.Candle,
	ltfAll []models.Candle,
	initialCapital float64,
	leverage float64,
	groups WindowGroups,
	windowEnd int64,
) []walking.WalkForwardTrade {

	var trades []walking.WalkForwardTrade
	currentCapital := initialCapital

	sort.Slice(htfWindow, func(i, j int) bool {
		return htfWindow[i].Time < htfWindow[j].Time
	})

	sort.Slice(ltfAll, func(i, j int) bool {
		return ltfAll[i].Time < ltfAll[j].Time
	})

	var lastCloseTime int64 = 0

	for _, htf := range htfWindow {
		if currentCapital <= 0 {
			break
		}

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

		// 🆕 تشخیص گروه Body کندل
		groupIdx := GetBodyGroupIndex(&htf)
		if groupIdx < 0 {
			continue // کمتر از 0.2%، وارد معامله نمی‌شود
		}

		// 🆕 دریافت TP/SL از گروه مناسب
		group := groups.Groups[groupIdx]
		if group.BestTP == 0 || group.BestSL == 0 {
			continue // ترکیب معتبری برای این گروه وجود ندارد
		}

		tpUSD := group.BestTP
		slUSD := group.BestSL

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

		trades = append(trades, walking.WalkForwardTrade{
			EntryTime:  entryTime,
			ExitTime:   exitTime,
			Type:       tradeType,
			EntryPrice: entryPrice,
			ExitPrice:  exitPrice,
			TP:         tpUSD,
			SL:         slUSD,
			PnL:        pnl,
			Status:     status,
			Comment:    fmt.Sprintf("tpPrice: %.2f slPrice: %.2f", tpPrice, slPrice),
		})
	}

	return trades
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
