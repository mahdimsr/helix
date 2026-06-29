package main

import (
	"fmt"
	"helix/exchange"
	"helix/indicators"
	"log"
	"math"
	"time"
)

func main() {
	log.Print("Hello, I`m helix")
	const dateTimeLayout = "2006-01-02 15:04"

	candles, err := exchange.GetCandles("BTCUSDT", "15m", 100)
	if err != nil {
		panic(err)
	}

	trades := indicators.BacktestUTBot(candles, 4, 20)

	for _, trade := range trades {
		openTime := time.UnixMilli(trade.OpenTime).UTC()
		closeTime := time.UnixMilli(trade.CloseTime).UTC()
		pnl := math.Abs(trade.OpenPrice-trade.ClosePrice) / 100
		if trade.Type == "Long" {
			fmt.Printf("🟢 Buy | Time: %s | openprice: %s | closeTime: %s | closePrice: %s | pnl: %s \n",
				openTime.Format(dateTimeLayout),
				trade.OpenPrice,
				closeTime.Format(dateTimeLayout),
				trade.ClosePrice,
				pnl,
			)
		} else {
			fmt.Printf("🔴 Sell | Time: %s | openprice: %s | closeTime: %s | closePrice: %s | pnl: %s \n",
				openTime.Format(dateTimeLayout),
				trade.OpenPrice,
				closeTime.Format(dateTimeLayout),
				trade.ClosePrice,
				pnl,
			)
		}
	}
}
