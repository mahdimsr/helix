package metatrader

import (
	"bufio"
	"encoding/json"
	"fmt"
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

	symbol := "XAUUSD.ecn"
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
			}

		}
	}
}

func placeOrder(conn net.Conn, side string) error {

	if _, err := conn.Write([]byte(side + "\n")); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// خواندن پاسخ JSON
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	fmt.Printf("response form metatrader: %s", line)

	/*var r OrderResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return fmt.Errorf("bad json %q: %w", line, err)
	}*/
	return nil
}

func requestCandles(client MTClient, symbol string, timeframe string, candlesCount int) {

	fmt.Printf("Fetch Candles symbol: %s | timeframe: %s | candlesCount: %d", symbol, timeframe, candlesCount)

	cmd := fmt.Sprintf("GET_CANDLES|%s|%s|%d\n", symbol, timeframe, candlesCount)

	fmt.Printf("sending CMD is: %s", cmd)

	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("Write error:", err)
	} else {
		log.Printf("Requested %d candles for %s %s", candlesCount, symbol, timeframe)
	}
}
