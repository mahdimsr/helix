package models

import (
	"fmt"
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

type BackTest struct {
	Trades      []Trade
	Wins        int
	Loss        int
	Winrate     float64
	GainPercent float64
}

func (candle *Candle) Body() float64 {
	openPrice := candle.Open
	closePrice := candle.Close

	return math.Abs(closePrice - openPrice)
}

func (candle *Candle) BodyPercentage() float64 {
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

	return shadowChangedPrice
}

func (candle *Candle) IsMarubozu() bool {

	if candle.Shadow() == 0 {
		return true
	}

	/*if candle.BodyPercentage() < 0.3 {
		return false
	}*/

	return candle.Body() > candle.Shadow()
}

func (candle *Candle) IsGreen() bool {

	return candle.Close > candle.Open
}

func (candle *Candle) IsRed() bool {

	return candle.Close < candle.Open
}

func (backtest *BackTest) Calculate() {

	for _, t := range backtest.Trades {
		if t.GainPercent > 0 {
			backtest.Wins++
		} else {
			backtest.Loss++
		}
		backtest.GainPercent += t.GainPercent
	}
	if len(backtest.Trades) > 0 {
		backtest.Winrate = float64(backtest.Wins) / float64(len(backtest.Trades)) * 100
	}
}

func (trade *Trade) ReadableTime(timeUnixMilli int64) string {
	openTime := time.UnixMilli(timeUnixMilli).UTC()

	return openTime.Format("2006-01-02 15:04")
}

func (backtest BackTest) PrintBacktest() {
	fmt.Println("========== Backtest Result ==========")
	fmt.Printf("Total Trades : %d\n", len(backtest.Trades))
	fmt.Printf("Wins         : %d\n", backtest.Wins)
	fmt.Printf("Losses       : %d\n", backtest.Loss)
	fmt.Printf("Win Rate     : %.2f%%\n", backtest.Winrate)
	fmt.Printf("Total Gain   : %.2f%%\n", backtest.GainPercent)
	fmt.Println("-------------------------------------")
	for idx, t := range backtest.Trades {
		status := "LOSS"
		if t.GainPercent > 0 {
			status = "WIN"
		}
		fmt.Printf("#%d %-5s | entry=%.2f exit=%.2f percentage=%.3f status=%s openTime=%s closeTime=%s \n",
			idx+1, t.Type, t.OpenPrice, t.ClosePrice, t.GainPercent, status, t.ReadableTime(t.OpenTime), t.ReadableTime(t.CloseTime))
	}
	fmt.Println("=====================================")
}
