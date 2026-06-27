package main

import (
	"fmt"
	"helix/exchange"
	"log"
	"time"
)

func main() {
	log.Print("Hello, I`m helix")
	//const dateTimeLayout = "2026"

	candles, err := exchange.GetCandles("BTCUSDT", "5m", 10)
	if err != nil {
		panic(err)
	}

	for _, candle := range candles {

		candleTime := time.UnixMilli(candle.Time).UTC()

		fmt.Printf("Time %s | Open: %s | High: %s | Close: %s | Low: %s \n",
			candleTime.Format("2006-01-02 15:04"),
			candle.Open,
			candle.High,
			candle.Close,
			candle.Low,
		)
	}
}
