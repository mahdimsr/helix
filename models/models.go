package models

import (
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Candle struct {
	Open         float64   `json:"open" bson:"open"`
	High         float64   `json:"high" bson:"high"`
	Close        float64   `json:"close" bson:"close"`
	Low          float64   `json:"low" bson:"low"`
	Volume       int64     `json:"volume" bson:"volume"`
	Time         int64     `json:"time" bson:"time"`
	ReadableTime time.Time `json:"readableTime" bson:"readableTime"`
	Symbol       string    `json:"symbol" bson:"symbol"`
	Timeframe    string    `json:"timeframe" bson:"timeframe"`
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
	GainPercent   float64
}

type Order struct {
	Id     primitive.ObjectID `bson:"id,omitempty"`
	Symbol string             `bson:"symbol"`
	Price  float64            `bson:"price"`
	Tp     float64            `bson:"tp"`
	Sl     float64            `bson:"sl"`
	Ticket int64              `bson:"ticket"`
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

func CalcGainPercent(t *Trade) {
	if t.Type == "Long" {
		t.GainPercent = (t.ClosePrice - t.OpenPrice) / t.OpenPrice * 100
	} else {
		t.GainPercent = (t.OpenPrice - t.ClosePrice) / t.OpenPrice * 100
	}
}
