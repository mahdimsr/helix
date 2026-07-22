package metatrader

import (
	"context"
	"encoding/json"
	"fmt"
	"helix/database"
	"helix/indicators"
	"helix/models"
	"helix/strategy"
	"io"
	"log"
	"net"
	"strings"
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

	db := database.MongoConnect()

	symbol := "BTCUSD"
	timeframe := "PERIOD_M15"
	Sensitivity := 3
	ATR := 20
	candlesCount := 200

	requestCandles(*client, symbol, timeframe, candlesCount)

	lines := make(chan string)
	readErr := make(chan error)
	go func() {
		for {
			line, err := client.ReadResponse()
			if err != nil {
				readErr <- err
				return
			}
			lines <- line
		}
	}()

	for {
		select {
		case <-ticker.C:
			// هر 1 دقیقه درخواست کندل
			fmt.Println("Requesting Candle")
			requestCandles(*client, symbol, timeframe, candlesCount)
		case err := <-readErr:
			if err == io.EOF {
				log.Println("connection closed by EA (EOF)")
				return
			}
			log.Println("read error:", err)
			return

		case line := <-lines:

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var result SocketResult
			if err := json.Unmarshal([]byte(line), &result); err != nil {
				log.Println("JSON parse error:", err)
				continue
			}

			log.Printf("Result type is %s", result.Type)

			if result.Type == "CANDLES" {
				candles := result.fetchDataAsCandle()

				fmt.Printf("Fetch %d candles \n", len(candles))

				lastCandle := candles[0]

				for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
					candles[i], candles[j] = candles[j], candles[i]
				}

				signal := strategy.CalculateSignal(candles, ATR, Sensitivity)

				if signal != indicators.NoneSignal {

					fmt.Printf("signal detected: %s", string(signal))

					amount, tp, sl := strategy.CalculateOrderUtils(lastCandle.Close, string(signal))
					placeOrder(*client, symbol, string(signal), amount, tp, sl)
				} else {
					fmt.Println("signal not detected")
				}
			}

			if result.Type == "ORDER" {
				orderResult := result.fetchDataAsOrder()
				order := &models.Order{
					Symbol: symbol,
					Tp:     orderResult.Tp,
					Sl:     orderResult.Sl,
					Ticket: orderResult.Ticket,
				}

				orderRepo := database.NewOrderRepository(&db)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				insertResult, err := orderRepo.Create(ctx, order)
				cancel()
				if err != nil {
					fmt.Println("Create Order error: ", err)
					continue
				}

				fmt.Println("Order Inserted Id By: \n", insertResult.InsertedID)
				fmt.Printf("OrderResult parameters are retCode: %d | ticket: %d \n", orderResult.Retcode, orderResult.Ticket)
			}

			if result.Type == "UPDATE_ORDER" {
				println("update sl received")
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

func updateOrder(client MTClient, ticket int64, stopLoss float64, takeProfit float64) error {

	// UPDATE_ORDER|ticket|sl|tp
	cmd := fmt.Sprintf("UPDATE_ORDER|%d|%0.5f|%0.5f\n", ticket, stopLoss, takeProfit)

	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("place order failed: ", err)
	}

	return nil
}
