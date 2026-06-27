package exchange

import (
	"encoding/json"
	"fmt"
	"helix/models"
	"io"
	"net/http"
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
