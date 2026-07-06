package metatrader

import (
	"encoding/json"
	"fmt"
	"helix/strategy"
	"log"
	"net"
	"time"
)

func Handle(conn net.Conn) {

	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := NewMT5Client(conn)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	symbol := "BTCUSD"
	timeframe := "PERIOD_H1"
	candlesCount := 200

	requestCandles(*client, symbol, timeframe, candlesCount)

	for {
		select {
		case <-ticker.C:
			// هر 1 دقیقه درخواست کندل
			requestCandles(*client, symbol, timeframe, candlesCount)

		default:
			// خواندن پاسخ از EA
			line, _ := client.ReadResponse()

			var result SocketResult
			if err := json.Unmarshal([]byte(line), &result); err != nil {
				log.Println("JSON parse error:", err)
				continue
			}

			log.Printf("Result type is %s", result.Type)

			if result.Type == "CANDLES" {
				candles := result.fetchDataAsCandle()

				fmt.Printf("Fetch %d candles", len(candles))

				lastCandle := candles[0]

				signal := strategy.CalculateSignal(candles)
				amount, tp, sl := strategy.CalculateOrderUtils(lastCandle.Close, signal)
				placeOrder(*client, symbol, signal, amount, tp, sl)
			}

		}
	}
}

func placeOrder(client MTClient, symbol string, signal string, lot float64, tp float64, sl float64) {

	// PLACE_ORDER|symbol|side|lot|tp|sl
	cmd := fmt.Sprintf("PLACE_ORDER|%s|%s|%.3f|%.3f|%.3f", symbol, signal, lot, tp, sl)
	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("place order failed: ", err)
	}
}

func requestCandles(client MTClient, symbol string, timeframe string, candlesCount int) {

	fmt.Printf("Fetch Candles symbol: %s | timeframe: %s | candlesCount: %d \n", symbol, timeframe, candlesCount)

	cmd := fmt.Sprintf("GET_CANDLES|%s|%s|%d\n", symbol, timeframe, candlesCount)

	fmt.Printf("sending CMD is: %s", cmd)

	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("Write error:", err)
	} else {
		log.Printf("Requested %d candles for %s %s", candlesCount, symbol, timeframe)
	}
}
