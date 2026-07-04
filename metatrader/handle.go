package metatrader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"helix/models"
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
	reader := bufio.NewReaderSize(conn, 1<<20)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	symbol := "XAUUSD.ecn"
	timeframe := "PERIOD_M1"
	candlesCount := 200

	requestCandles(conn, symbol, timeframe, candlesCount)

	for {
		select {
		case <-ticker.C:
			// هر 1 دقیقه درخواست کندل
			requestCandles(conn, symbol, timeframe, candlesCount)

		default:
			// خواندن پاسخ از EA
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Read error:", err)
				return
			}

			log.Printf("RAW received (%d bytes): %q", len(line), line)

			var candles []models.Candle
			if err := json.Unmarshal([]byte(line), &candles); err != nil {
				log.Println("JSON parse error:", err)
				continue
			}

			log.Printf("Received %d candles for XAUUSD.ecn", len(candles))
			if len(candles) > 0 {
				last := candles[len(candles)-1]
				log.Printf("Last candle: time=%d open=%.2f high=%.2f low=%.2f close=%.2f",
					last.Time, last.Open, last.High, last.Low, last.Close)
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

func requestCandles(conn net.Conn, symbol string, timeframe string, candlesCount int) {

	fmt.Printf("Fetch Candles symbol: %s | timeframe: %s | candlesCount: %d", symbol, timeframe, candlesCount)

	cmd := fmt.Sprintf("GET_CANDLES|%s|%s|%d\n", symbol, timeframe, candlesCount)

	fmt.Printf("sending CMD is: %s", cmd)

	_, err := conn.Write([]byte(cmd))
	if err != nil {
		log.Println("Write error:", err)
	} else {
		log.Printf("Requested %d candles for %s %s", candlesCount, symbol, timeframe)
	}
}
