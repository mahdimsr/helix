package table

import (
	"bytes"
	"fmt"
	"helix/walking"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
)

// UnifiedTrade ساختار یکپارچه برای هر دو نوع ترید
type UnifiedTrade struct {
	OpenTime  time.Time
	CloseTime time.Time
	Type      string
	Entry     float64
	Exit      float64
	TP        float64
	SL        float64
	PnL       float64
	ROI       float64
	Source    string // "LIVE" یا "BACKTEST"
}

// MatchedRow نتیجه تطبیق یک ردیف
type MatchedRow struct {
	OpenTime time.Time
	Live     *UnifiedTrade // nil اگر ترید لایو وجود نداشته باشد
	Backtest *UnifiedTrade // nil اگر ترید بک‌تست وجود نداشته باشد
}

// calculateSimilarity درصد شباهت بین دو عدد را محاسبه می‌کند
// (این تابع به‌طور خودکار اعداد منفی مثل PnL را هم به‌درستی مدیریت می‌کند)
func calculateSimilarity(liveVal, btVal float64) float64 {
	if liveVal == 0 {
		if btVal == 0 {
			return 100.0 // هر دو صفر هستند، پس کاملاً شبیه‌اند
		}
		return 0.0 // یکی صفر و دیگری غیرصفر است، شباهتی ندارند
	}

	diff := math.Abs(liveVal - btVal)
	// استفاده از math.Abs برای مخرج، باعث می‌شود اگر PnL منفی باشد، تقسیم بر عدد منفی انجام نشود
	diffPercent := (diff / math.Abs(liveVal)) * 100.0
	similarity := 100.0 - diffPercent

	// جلوگیری از منفی شدن درصد در صورت اختلاف بسیار فاحش
	if similarity < 0 {
		return 0.0
	}
	return similarity
}

// calculateRowSimilarity میانگین شباهت ۵ پارامتر کلیدی را محاسبه می‌کند
func calculateRowSimilarity(live, bt *UnifiedTrade) float64 {
	if live == nil || bt == nil {
		return 0.0
	}

	simEntry := calculateSimilarity(live.Entry, bt.Entry)
	simExit := calculateSimilarity(live.Exit, bt.Exit)
	simTP := calculateSimilarity(live.TP, bt.TP)
	simSL := calculateSimilarity(live.SL, bt.SL)
	simPnL := calculateSimilarity(live.PnL, bt.PnL) // ✅ پارامتر جدید اضافه شد

	// ✅ میانگین‌گیری بر روی ۵ پارامتر
	return (simEntry + simExit + simTP + simSL + simPnL) / 5.0
}

// ---------------------------------------------------------

func GenerateExcelFile(liveTrades []walking.WalkForwardTrade, backtestTrades []walking.WalkForwardTrade) ([]byte, error) {

	// ۱. تبدیل هر دو لیست به فرمت یکپارچه
	unifiedLive := convertLiveTrades(liveTrades)
	unifiedBacktest := convertBacktestTrades(backtestTrades)

	// ۲. مرتب‌سازی بر اساس OpenTime
	sort.Slice(unifiedLive, func(i, j int) bool {
		return unifiedLive[i].OpenTime.Before(unifiedLive[j].OpenTime)
	})
	sort.Slice(unifiedBacktest, func(i, j int) bool {
		return unifiedBacktest[i].OpenTime.Before(unifiedBacktest[j].OpenTime)
	})

	// ۳. تطبیق تریدها با اختلاف حداکثر ۳ ثانیه
	matchedRows := matchTrades(unifiedLive, unifiedBacktest, 3*time.Second)

	// ۴. ساخت فایل اکسل
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Comparison"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// ۵. هدرها (اکنون ۱۶ ستون)
	headers := []string{
		"Live_Open_Time", "Live_Type", "Live_Entry", "Live_Exit", "Live_TP", "Live_SL", "Live_PnL",
		"BT_Open_Time", "BT_Type", "BT_Entry", "BT_Exit", "BT_TP", "BT_SL", "BT_PnL",
		"Match", "Similarity (%)", // ✅ ستون جدید اضافه شد
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// ۶. استایل هدرها
	headerStyleLive, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#2F5496"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyleBT, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#548235"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyleNeutral, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#404040"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	for i := 0; i < len(headers); i++ {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		// ستون‌های ۱۵ و ۱۶ (Match و Similarity) خاکستری
		if i >= 14 {
			f.SetCellStyle(sheetName, cell, cell, headerStyleNeutral)
		} else if i < 7 { // ستون‌های ۱ تا ۷ (Live)
			f.SetCellStyle(sheetName, cell, cell, headerStyleLive)
		} else { // ستون‌های ۸ تا ۱۴ (Backtest)
			f.SetCellStyle(sheetName, cell, cell, headerStyleBT)
		}
	}

	// ۷. استایل‌های داده
	profitStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#006100"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C6EFCE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	lossStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#9C0006"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFC7CE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	normalStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	matchStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#006100"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C6EFCE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	mismatchStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#9C0006"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFC7CE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// استایل‌های ستون‌های نظیر
	entryStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#404040"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#F2F2F2"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	exitStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#7F6000"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFF2CC"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	tpStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#1F4E79"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#DEEAF6"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	slStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#C65911"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FCE4D6"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// ✅ استایل‌های جدید برای ستون درصد شباهت (رنگ‌بندی شرطی)
	highSimStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#006100"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C6EFCE"}}, // سبز
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	medSimStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#7F6000"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFF2CC"}}, // زرد
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	lowSimStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#9C0006"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFC7CE"}}, // قرمز
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// ۸. پر کردن داده‌ها
	for r, row := range matchedRows {
		rowNum := r + 2

		// --- بخش Live (ستون‌های ۱ تا ۷) ---
		if row.Live != nil {
			cell, _ := excelize.CoordinatesToCellName(1, rowNum)
			f.SetCellValue(sheetName, cell, row.Live.OpenTime.Format("2006-01-02 15:04:05"))
			f.SetCellStyle(sheetName, cell, cell, normalStyle)

			writeTradeDataAdjusted(f, sheetName, rowNum, 2, row.Live, profitStyle, lossStyle, normalStyle, entryStyle, exitStyle, tpStyle, slStyle)
		} else {
			for c := 1; c <= 7; c++ {
				cell, _ := excelize.CoordinatesToCellName(c, rowNum)
				f.SetCellValue(sheetName, cell, "-")
				f.SetCellStyle(sheetName, cell, cell, normalStyle)
			}
		}

		// --- بخش Backtest (ستون‌های ۸ تا ۱۴) ---
		if row.Backtest != nil {
			cell, _ := excelize.CoordinatesToCellName(8, rowNum)
			f.SetCellValue(sheetName, cell, row.Backtest.OpenTime.Format("2006-01-02 15:04:05"))
			f.SetCellStyle(sheetName, cell, cell, normalStyle)

			writeTradeDataAdjusted(f, sheetName, rowNum, 9, row.Backtest, profitStyle, lossStyle, normalStyle, entryStyle, exitStyle, tpStyle, slStyle)
		} else {
			for c := 8; c <= 14; c++ {
				cell, _ := excelize.CoordinatesToCellName(c, rowNum)
				f.SetCellValue(sheetName, cell, "-")
				f.SetCellStyle(sheetName, cell, cell, normalStyle)
			}
		}

		// --- بخش Match (ستون ۱۵) ---
		cellMatch, _ := excelize.CoordinatesToCellName(15, rowNum)
		if row.Live != nil && row.Backtest != nil {
			f.SetCellValue(sheetName, cellMatch, "✅")
			f.SetCellStyle(sheetName, cellMatch, cellMatch, matchStyle)
		} else {
			f.SetCellValue(sheetName, cellMatch, "❌")
			f.SetCellStyle(sheetName, cellMatch, cellMatch, mismatchStyle)
		}

		// --- ✅ بخش درصد شباهت (ستون ۱۶) ---
		cellSim, _ := excelize.CoordinatesToCellName(16, rowNum)
		if row.Live != nil && row.Backtest != nil {
			similarity := calculateRowSimilarity(row.Live, row.Backtest)
			f.SetCellValue(sheetName, cellSim, fmt.Sprintf("%.1f%%", similarity))

			// اعمال رنگ‌بندی شرطی بر اساس مقدار شباهت
			var simStyle int
			if similarity >= 95.0 {
				simStyle = highSimStyle
			} else if similarity >= 80.0 {
				simStyle = medSimStyle
			} else {
				simStyle = lowSimStyle
			}
			f.SetCellStyle(sheetName, cellSim, cellSim, simStyle)
		} else {
			f.SetCellValue(sheetName, cellSim, "-")
			f.SetCellStyle(sheetName, cellSim, cellSim, normalStyle)
		}
	}

	// ۹. تنظیم عرض ستون‌ها (۱۶ ستون از A تا P)
	colWidths := map[string]float64{
		"A": 20, "B": 10, "C": 10, "D": 10, "E": 10, "F": 10, "G": 10,
		"H": 20, "I": 10, "J": 10, "K": 10, "L": 10, "M": 10, "N": 10,
		"O": 8, "P": 14, // ✅ عرض ستون Similarity اضافه شد
	}
	for col, w := range colWidths {
		f.SetColWidth(sheetName, col, col, w)
	}

	// ۱۰. فیلتر خودکار (از A1 تا P)
	lastRow := len(matchedRows) + 1
	f.AutoFilter(sheetName, fmt.Sprintf("A1:P%d", lastRow), nil)

	// ۱۱. خروجی به []byte
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeTradeDataAdjusted (بدون تغییر نسبت به نسخه قبلی شما)
func writeTradeDataAdjusted(f *excelize.File, sheetName string, rowNum, startCol int, trade *UnifiedTrade, profitStyle, lossStyle, normalStyle, entryStyle, exitStyle, tpStyle, slStyle int) {
	values := []interface{}{
		trade.Type,
		trade.Entry,
		trade.Exit,
		trade.TP,
		trade.SL,
		trade.PnL,
	}

	for c, val := range values {
		cell, _ := excelize.CoordinatesToCellName(startCol+c, rowNum)
		f.SetCellValue(sheetName, cell, val)

		var styleID int
		switch c {
		case 0: // Type
			styleID = normalStyle
		case 1: // Entry
			styleID = entryStyle
		case 2: // Exit
			styleID = exitStyle
		case 3: // TP
			styleID = tpStyle
		case 4: // SL
			styleID = slStyle
		case 5: // PnL
			if trade.PnL > 0 {
				styleID = profitStyle
			} else if trade.PnL < 0 {
				styleID = lossStyle
			} else {
				styleID = normalStyle
			}
		}
		f.SetCellStyle(sheetName, cell, cell, styleID)
	}
}

func SendExcelToTelegram(botToken, chatID string, fileData []byte, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", botToken)

	// ساخت multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// فیلد chat_id
	writer.WriteField("chat_id", chatID)

	// فیلد caption
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	_ = godotenv.Load()
	appName := os.Getenv("APP_NAME")
	// فیلد فایل
	part, err := writer.CreateFormFile("document", fmt.Sprintf("%s_report.xlsx", appName))
	if err != nil {
		return err
	}
	part.Write(fileData)
	writer.Close()

	// ارسال درخواست
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(respBody))
	}

	return nil
}

func convertLiveTrades(liveTrades []walking.WalkForwardTrade) []UnifiedTrade {

	var trades []UnifiedTrade

	for _, liveTrade := range liveTrades {
		trades = append(trades, UnifiedTrade{
			OpenTime:  time.Unix(liveTrade.EntryTime, 0),
			CloseTime: time.Unix(liveTrade.ExitTime, 0),
			Type:      liveTrade.Type,
			Entry:     liveTrade.EntryPrice,
			Exit:      liveTrade.ExitPrice,
			TP:        liveTrade.TP,
			SL:        liveTrade.SL,
			PnL:       liveTrade.PnL,
			Source:    "Live",
		})
	}

	return trades
}

func convertBacktestTrades(backtestTrades []walking.WalkForwardTrade) []UnifiedTrade {

	var trades []UnifiedTrade

	for _, liveTrade := range backtestTrades {

		entry := liveTrade.EntryPrice - 0.1
		volume := 10000.0

		var tpPrice, slPrice float64

		tpPnl := 8.0
		slPnl := 4.0 + 0.3

		if liveTrade.Type == "BUY" {

			tpPrice = calculateTargetPrice(entry, volume, tpPnl, "XAU", "BUY")
			slPrice = calculateTargetPrice(entry, volume, slPnl, "XAU", "SELL")

		} else {

			tpPrice = calculateTargetPrice(entry, volume, tpPnl, "XAU", "SELL")
			slPrice = calculateTargetPrice(entry, volume, slPnl, "XAU", "BUY")
		}

		var pnl, exitPrice float64
		if liveTrade.PnL > 0 {
			pnl = tpPnl - 0.3
			exitPrice = tpPrice
		} else {
			pnl = 0 - slPnl
			exitPrice = slPrice
		}

		trades = append(trades, UnifiedTrade{
			OpenTime:  time.Unix(liveTrade.EntryTime, 0).Truncate(time.Minute),
			CloseTime: time.Unix(liveTrade.ExitTime, 0).Truncate(time.Minute),
			Type:      liveTrade.Type,
			Entry:     liveTrade.EntryPrice - 0.1,
			Exit:      exitPrice,
			TP:        tpPrice,
			SL:        slPrice,
			PnL:       pnl,
			Source:    "BackTest",
		})
	}

	return trades
}

// matchTrades تریدهای لایو و بک‌تست را با اختلاف زمانی مشخص جفت می‌کند
func matchTrades(live, backtest []UnifiedTrade, maxDiff time.Duration) []MatchedRow {
	var result []MatchedRow

	i, j := 0, 0
	usedBT := make(map[int]bool)

	for i < len(live) {
		liveTrade := live[i]
		matched := false

		// جستجو در بک‌تست برای پیدا کردن نزدیک‌ترین ترید
		for j < len(backtest) {
			btTrade := backtest[j]
			diff := liveTrade.OpenTime.Sub(btTrade.OpenTime)
			if diff < 0 {
				diff = -diff
			}

			if diff <= maxDiff && !usedBT[j] {
				// تطبیق پیدا شد
				result = append(result, MatchedRow{
					OpenTime: liveTrade.OpenTime,
					Live:     &live[i],
					Backtest: &backtest[j],
				})
				usedBT[j] = true
				matched = true
				j++
				break
			} else if btTrade.OpenTime.Before(liveTrade.OpenTime.Add(-maxDiff)) {
				// ترید بک‌تست خیلی قدیمی است، دیگر تطبیق نمی‌یابد
				if !usedBT[j] {
					result = append(result, MatchedRow{
						OpenTime: btTrade.OpenTime,
						Live:     nil,
						Backtest: &backtest[j],
					})
					usedBT[j] = true
				}
				j++
			} else {
				break
			}
		}

		if !matched {
			// ترید لایو بدون تطبیق
			result = append(result, MatchedRow{
				OpenTime: liveTrade.OpenTime,
				Live:     &live[i],
				Backtest: nil,
			})
		}
		i++
	}

	// اضافه کردن تریدهای بک‌تست باقی‌مانده
	for j < len(backtest) {
		if !usedBT[j] {
			result = append(result, MatchedRow{
				OpenTime: backtest[j].OpenTime,
				Live:     nil,
				Backtest: &backtest[j],
			})
		}
		j++
	}

	// مرتب‌سازی نهایی بر اساس OpenTime
	sort.Slice(result, func(i, j int) bool {
		return result[i].OpenTime.Before(result[j].OpenTime)
	})

	return result
}

// writeTradeData داده‌های یک ترید را در ستون‌های مشخص می‌نویسد
func writeTradeData(f *excelize.File, sheetName string, rowNum, startCol int, trade *UnifiedTrade, profitStyle, lossStyle, normalStyle int) {
	values := []interface{}{
		trade.Type,
		trade.Entry,
		trade.Exit,
		trade.TP,
		trade.SL,
		trade.PnL,
		trade.ROI,
	}

	for c, val := range values {
		cell, _ := excelize.CoordinatesToCellName(startCol+c, rowNum)
		f.SetCellValue(sheetName, cell, val)

		// استایل بر اساس PnL
		if c == 5 { // ستون PnL
			if trade.PnL > 0 {
				f.SetCellStyle(sheetName, cell, cell, profitStyle)
			} else if trade.PnL < 0 {
				f.SetCellStyle(sheetName, cell, cell, lossStyle)
			} else {
				f.SetCellStyle(sheetName, cell, cell, normalStyle)
			}
		} else {
			f.SetCellStyle(sheetName, cell, cell, normalStyle)
		}
	}
}

func calculateTargetPrice(entryPrice, volumeUSD, targetUSD float64, symbol string, side string) float64 {
	if volumeUSD <= 0 {
		return 0 // حجم دلاری باید عددی مثبت باشد
	}

	upperSymbol := strings.ToUpper(symbol)
	var targetPrice float64

	// تشخیص اینکه آیا ارز پایه (Base Currency) دلار است یا خیر
	// مثال: USDJPY, USDCHF (ارز پایه USD است)
	// مثال: EURUSD, XAUUSD, BTCUSD (ارز مظنه (Quote) USD است)
	isBaseUSD := strings.HasPrefix(upperSymbol, "USD") && len(upperSymbol) == 6

	if !isBaseUSD {
		// حالت اول: Quote = USD (مثل EURUSD, GBPUSD, XAUUSD, BTCUSD)
		// تعداد واحد ارز پایه = volumeUSD / entryPrice
		// فرمول سود: Profit = (TargetPrice - EntryPrice) * (volumeUSD / entryPrice)
		// با بازآرایی فرمول برای TargetPrice:

		priceDiff := (targetUSD * entryPrice) / volumeUSD

		if side == "BUY" {
			targetPrice = entryPrice + priceDiff
		} else {
			targetPrice = entryPrice - priceDiff
		}
	} else {
		// حالت دوم: Base = USD (مثل USDJPY, USDCHF)
		// تعداد واحد ارز پایه (که خود دلار است) = volumeUSD
		// فرمول سود: Profit = (TargetPrice - EntryPrice) * volumeUSD / TargetPrice
		// با حل معادله ریاضی برای TargetPrice:

		if side == "BUY" {
			denominator := volumeUSD - targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // جلوگیری از تقسیم بر صفر
			}
			targetPrice = (entryPrice * volumeUSD) / denominator
		} else {
			denominator := volumeUSD + targetUSD
			if math.Abs(denominator) < 0.0001 {
				return 0 // جلوگیری از تقسیم بر صفر
			}
			targetPrice = (entryPrice * volumeUSD) / denominator
		}
	}

	return targetPrice
}
