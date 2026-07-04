package models

import (
	"math"
)

type Candle struct {
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Close  float64 `json:"close"`
	Low    float64 `json:"low"`
	Volume int64   `json:"volume"`
	Time   int64   `json:"time"`
	Symbol string  `json:"symbol"`
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
	openPrice := candle.Open
	closePrice := candle.Close

	return math.Abs((closePrice-openPrice)/openPrice) * 100
}

func (candle *Candle) Shadow() float64 {
	highPrice := candle.High
	lowPrice := candle.Low
	openPrice := candle.Open
	closePrice := candle.Close

	shadowChangedPrice := math.Abs(highPrice-lowPrice) - math.Abs(closePrice-openPrice)

	return (shadowChangedPrice / openPrice) * 100
}

func (candle *Candle) IsMarubozu() bool {

	if candle.Shadow() == 0 {
		return true
	}

	return candle.Body() > candle.Shadow()
}
