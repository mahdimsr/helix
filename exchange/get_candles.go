package exchange

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"helix/database"
	"helix/models"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func GetCandles(symbol string, timeframe string, limit int) ([]models.Candle, error) {

	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		symbol,
		timeframe,
		limit,
	)

	// 2. Make the HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Binance API: %w", err)
	}
	defer resp.Body.Close()

	// 3. Check for successful response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance API returned non-200 status: %s, body: %s", resp.Status, string(body))
	}

	// 4. Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 5. Unmarshal the JSON response into a slice of Candle structs
	var candles []models.Candle
	if err := json.Unmarshal(body, &candles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return candles, nil
}

func GetCandlesByTimeRange(symbol string, timeframe string, startTime, endTime int64) ([]models.Candle, error) {

	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000",
		symbol,
		timeframe,
		startTime,
		endTime,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Binance API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance API returned non-200 status: %s, body: %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	candles, err := parseBinanceCandles(body)
	if err != nil {
		return nil, fmt.Errorf("parsing binance candles error: %s", err)
	}

	for i := range candles {
		candles[i].Symbol = symbol
		candles[i].Timeframe = timeframe
	}

	return candles, nil
}

func GetAllCandlesByDateString(symbol string, timeframe string, startDateStr string, endDateStr string) ([]models.Candle, error) {

	layout := "2006-01-02"

	startTimeObj, err := time.Parse(layout, startDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format, expected YYYY-MM-DD: %w", err)
	}

	endTimeObj, err := time.Parse(layout, endDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end date format, expected YYYY-MM-DD: %w", err)
	}

	currentStartTime := startTimeObj.UnixMilli()

	endTime := endTimeObj.UnixMilli()

	var allCandles []models.Candle

	for currentStartTime < endTime {
		candles, err := GetCandlesByTimeRange(symbol, timeframe, currentStartTime, endTime)
		if err != nil {
			return nil, fmt.Errorf("error fetching candles at timestamp %d: %w", currentStartTime, err)
		}

		if len(candles) == 0 {
			break
		}

		allCandles = append(allCandles, candles...)

		lastCandleTime := candles[len(candles)-1].Time

		currentStartTime = lastCandleTime + 1
	}

	return allCandles, nil
}

func parseBinanceCandles(jsonData []byte) ([]models.Candle, error) {
	var rawCandles [][]interface{}

	err := json.Unmarshal(jsonData, &rawCandles)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal array: %v", err)
	}

	var candles []models.Candle

	for _, raw := range rawCandles {
		if len(raw) < 6 {
			continue
		}

		//log.Fatalf("format raw: %s", jsonData)

		openTimeFloat, ok := raw[0].(float64)
		if !ok {
			continue
		}

		// قیمت‌ها در ریسپانس بایننس از نوع String هستند، پس اول به String و بعد به Float64 تبدیل می‌کنیم
		openStr, _ := raw[1].(string)
		open, _ := strconv.ParseFloat(openStr, 64)

		highStr, _ := raw[2].(string)
		high, _ := strconv.ParseFloat(highStr, 64)

		lowStr, _ := raw[3].(string)
		low, _ := strconv.ParseFloat(lowStr, 64)

		closeStr, _ := raw[4].(string)
		closePrice, _ := strconv.ParseFloat(closeStr, 64)

		// ۴. ساخت Struct کندل
		candle := models.Candle{
			Time:         int64(openTimeFloat),
			Open:         open,
			High:         high,
			Low:          low,
			Close:        closePrice,
			ReadableTime: time.UnixMilli(int64(openTimeFloat)).UTC(),
		}

		candles = append(candles, candle)
	}

	return candles, nil
}

func ImportMT4CSVToMongoDB(csvFilePath string, symbol string, timeframe string) error {

	// ─────────────────────────────────────────────
	// 1. خواندن فایل CSV
	// ─────────────────────────────────────────────
	file, err := os.Open(csvFilePath)
	if err != nil {
		return fmt.Errorf("failed to open csv file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t' // متاتریدر از Tab به عنوان جداکننده استفاده می‌کند
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read csv: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("csv file is empty or has only header")
	}

	// ─────────────────────────────────────────────
	// 2. تبدیل رکوردها به Candle
	// ─────────────────────────────────────────────
	var candles []models.Candle

	for i, record := range records[1:] { // رد کردن هدر
		if len(record) < 7 {
			log.Printf("skipping row %d: insufficient columns", i+2)
			continue
		}

		// ترکیب Date و Time: "2026.09.01" + "01:00:00"
		dateTimeStr := strings.TrimSpace(record[0]) + " " + strings.TrimSpace(record[1])

		t, err := time.Parse("2006.01.02 15:04:05", dateTimeStr)
		if err != nil {
			log.Printf("row %d: failed to parse time '%s': %v", i+2, dateTimeStr, err)
			continue
		}

		open, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
		if err != nil {
			log.Printf("row %d: failed to parse open: %v", i+2, err)
			continue
		}

		high, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
		if err != nil {
			log.Printf("row %d: failed to parse high: %v", i+2, err)
			continue
		}

		low, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
		if err != nil {
			log.Printf("row %d: failed to parse low: %v", i+2, err)
			continue
		}

		closePrice, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
		if err != nil {
			log.Printf("row %d: failed to parse close: %v", i+2, err)
			continue
		}

		tickVol, _ := strconv.ParseInt(strings.TrimSpace(record[6]), 10, 64)

		candle := models.Candle{
			Open:         open,
			High:         high,
			Close:        closePrice,
			Low:          low,
			Volume:       tickVol, // در فارکس Volume معمولاً 0 است، از TickVolume استفاده می‌کنیم
			Time:         t.UnixMilli(),
			ReadableTime: t,
			Symbol:       symbol,
			Timeframe:    timeframe,
		}

		candles = append(candles, candle)
	}

	if len(candles) == 0 {
		return fmt.Errorf("no valid candles found in csv file")
	}

	db := database.MongoConnect()
	candlesRepo := database.NewCandleRepository(&db)

	// درج دسته‌ای (Batch Insert)
	err = candlesRepo.CreateMany(context.Background(), candles)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✅ Successfully inserted %d candles\n", len(candles))

	return nil
}
