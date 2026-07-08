package exchange

import (
	"encoding/json"
	"fmt"
	"helix/models"
	"io"
	"net/http"
	"strconv"
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
