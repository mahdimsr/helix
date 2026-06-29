package models

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type Candle struct {
	Open   string
	High   string
	Close  string
	Low    string
	Volume string
	Time   int64
	Symbol string
}

type Trade struct {
	Type          string // "Long" or "Short"
	OpenPrice     float64
	ClosePrice    float64
	Duration      int64
	OpenTime      int64
	CloseTime     int64
	RunupPrice    float64
	RunupPercent  float64
	RunupTime     int64
	RunupDuration int64
	RiskPrice     float64
	RiskPercent   float64
	RiskTime      int64
	RiskDuration  int64
	ATR           int
	Sensitivity   float64
}

func (candle *Candle) Body() float64 {
	openPrice, _ := strconv.ParseFloat(candle.Open, 64)
	closePrice, _ := strconv.ParseFloat(candle.Close, 64)

	return math.Abs((closePrice-openPrice)/openPrice) * 100
}

func (candle *Candle) Shadow() float64 {
	highPrice, _ := strconv.ParseFloat(candle.High, 64)
	lowPrice, _ := strconv.ParseFloat(candle.Low, 64)
	openPrice, _ := strconv.ParseFloat(candle.Open, 64)
	closePrice, _ := strconv.ParseFloat(candle.Close, 64)

	shadowChangedPrice := math.Abs(highPrice-lowPrice) - math.Abs(closePrice-openPrice)

	return (shadowChangedPrice / openPrice) * 100
}

func (candle *Candle) IsMarubozu() bool {

	if candle.Shadow() == 0 {
		return true
	}

	return candle.Body() > candle.Shadow()
}

func (candle *Candle) UnmarshalJSON(data []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal raw candle data: %w", err)
	}

	if len(raw) < 11 {
		return fmt.Errorf("invalid candle data length: expected at least 11 elements, got %d", len(raw))
	}

	// Helper to convert interface{} to string, returns empty string on failure
	getString := func(v interface{}) string {
		s, _ := v.(string)
		return s
	}

	getInt64 := func(v interface{}) int64 {
		f, _ := v.(float64)
		return int64(f)
	}

	candle.Open = getString(raw[1])
	candle.High = getString(raw[2])
	candle.Low = getString(raw[3])
	candle.Close = getString(raw[4])
	candle.Volume = getString(raw[5])
	candle.Time = getInt64(raw[6])

	return nil
}
