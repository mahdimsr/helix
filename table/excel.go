package table

import (
	"bytes"
	"fmt"
	"helix/walking"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/xuri/excelize/v2"
)

func GenerateExcelFile(backtestTade []walking.WalkForwardTrade) ([]byte, error) {

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "BackTest"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)
	// شیت پیش‌فرض Sheet1 را حذف می‌کنیم
	f.DeleteSheet("Sheet1")

	// ── ۱. هدرها ──
	headers := []string{
		"Open Timestamp", "Close Timestamp", "Type",
		"Entry", "Exit", "TP", "SL",
		"PnL",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// ── ۲. استایل هدر (پس‌زمینه آبی تیره، متن سفید، Bold) ──
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#2F5496"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "#FFFFFF", Style: 1},
			{Type: "right", Color: "#FFFFFF", Style: 1},
			{Type: "top", Color: "#FFFFFF", Style: 1},
			{Type: "bottom", Color: "#FFFFFF", Style: 1},
		},
	})
	for i := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// ── ۳. استایل‌های داده ──
	// استایل سود (سبز)
	profitStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#006100"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C6EFCE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	// استایل ضرر (قرمز)
	lossStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "#9C0006"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FFC7CE"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	// استایل عادی
	normalStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	// استایل Long (آبی)
	longStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#2F5496"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	// استایل Short (نارنجی)
	shortStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#C55A11"},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// ── ۴. پر کردن داده‌ها ──
	for r, row := range backtestTade {
		rowNum := r + 2 // ردیف ۱ هدر است

		values := []interface{}{
			row.EntryTime,
			row.ExitTime,
			row.Type,
			row.EntryPrice,
			row.ExitPrice,
			row.TP,
			row.SL,
			row.PnL,
		}

		for c, val := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			f.SetCellValue(sheetName, cell, val)

			// اعمال استایل بر اساس محتوا
			switch c {
			case 2: // Type
				if row.Type == "Long" {
					f.SetCellStyle(sheetName, cell, cell, longStyle)
				} else {
					f.SetCellStyle(sheetName, cell, cell, shortStyle)
				}
			case 7, 10: // PnL و ROI
				if row.PnL > 0 {
					f.SetCellStyle(sheetName, cell, cell, profitStyle)
				} else if row.PnL < 0 {
					f.SetCellStyle(sheetName, cell, cell, lossStyle)
				} else {
					f.SetCellStyle(sheetName, cell, cell, normalStyle)
				}
			default:
				f.SetCellStyle(sheetName, cell, cell, normalStyle)
			}
		}
	}

	// ── ۵. تنظیم عرض ستون‌ها ──
	colWidths := map[string]float64{
		"A": 18, "B": 18, "C": 8,
		"D": 10, "E": 10, "F": 8, "G": 8,
		"H": 10, "I": 12, "J": 12, "K": 10,
	}
	for col, w := range colWidths {
		f.SetColWidth(sheetName, col, col, w)
	}

	// ── ۶. فیلتر خودکار (Auto Filter) ──
	lastRow := len(backtestTade) + 1
	f.AutoFilter(sheetName, fmt.Sprintf("A1:K%d", lastRow), nil)

	// ── ۷. خروجی به []byte ──
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

	// فیلد فایل
	part, err := writer.CreateFormFile("document", "trades_report.xlsx")
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
