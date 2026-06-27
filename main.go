package main

import (
	"fmt"
	"helix/exchange"
	"log"
	"time"
)

func main() {
	log.Print("Hello, I`m helix")
	const dateTimeLayout = "2006-01-02 15:04"

	candles, err := exchange.GetCandles("BTCUSDT", "5m", 10)
	if err != nil {
		panic(err)
	}

	lastCandle := candles[5]

	candleTime := time.UnixMilli(lastCandle.Time).UTC()

	fmt.Printf("Time %s | Open: %s | High: %s | Close: %s | Low: %s | body: %f | shadow: %f \n",
		candleTime.Format(dateTimeLayout),
		lastCandle.Open,
		lastCandle.High,
		lastCandle.Close,
		lastCandle.Low,
		lastCandle.Body(),
		lastCandle.Shadow(),
	)
}
