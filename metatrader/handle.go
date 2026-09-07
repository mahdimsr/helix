package metatrader

import (
	"encoding/json"
	"fmt"
	"helix/database"
	"helix/indicators"
	"helix/notification"
	"helix/strategy"
	"io"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func Handle(conn net.Conn) {

	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)
	client := NewMT5Client(conn)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	tickerSec := time.NewTicker(3 * time.Second)
	defer tickerSec.Stop()

	_ = godotenv.Load()

	symbol := "BTCUSD.ecn"
	timeframe := "PERIOD_M15"
	candlesCount := 500

	ticketRepo, err := database.NewFileRepository("tickets.json")
	if err != nil {
		log.Fatal("Failed to initialize ticket repository:", err)
	}

	requestCandles(*client, symbol, timeframe, candlesCount)
	time.Sleep(50 * time.Millisecond)
	inquiryOpenOrders(*client, ticketRepo)

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
		case <-tickerSec.C:
			if ticketRepo.Count() > 0 {
				time.Sleep(50 * time.Millisecond)
				inquiryOpenOrders(*client, ticketRepo)
			}
		case <-ticker.C:

			fmt.Println("Requesting Candle")
			requestCandles(*client, symbol, timeframe, candlesCount)

			time.Sleep(50 * time.Millisecond)

			fmt.Println("Inquiry Order")
			inquiryOpenOrders(*client, ticketRepo)
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

				for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
					candles[i], candles[j] = candles[j], candles[i]
				}

				lastCandle := candles[len(candles)-1]
				signal, tp, sl := strategy.PulseStrategy(candles)

				fmt.Printf("signal: %s | tp: %.2f | sl: %.2f \n", signal, tp, sl)

				if signal != indicators.NoneSignal {

					fmt.Printf("signal detected: %s", string(signal))

					//amount, _, _ := strategy.CalculateOrderUtils(lastCandle.Close, string(signal))
					amount := 0.2
					placeOrder(*client, symbol, string(signal), amount, lastCandle.Close, tp, sl)
				} else {
					fmt.Println("signal not detected")
				}
			}

			if result.Type == "ORDER" {
				orderResult := result.fetchDataAsOrder()

				err = ticketRepo.SaveTicket(orderResult.Ticket)
				if err != nil {
					fmt.Println("Save Ticket error: ", err)
				}
			}

			if result.Type == "UPDATE_ORDER" {
				println("update sl received")
			}

			if result.Type == "INQUIRY" {

				order := result.fetchDataAsOrder()

				if !order.SUCCESS {
					log.Println("Order/Ticket not found:", order.Ticket)
					continue
				}

				// تفسیر Retcode به عنوان وضعیت
				switch order.Retcode {
				case 1000:
					log.Printf("✅ Position OPEN. Current Price: %.5f | Info: %s", order.Price, order.Comment)

					totalDistance := math.Abs(order.Tp - order.EntryPrice)

					if totalDistance > 0 { // جلوگیری از تقسیم بر صفر

						tpProgress := calculateTPProgress(order.EntryPrice, order.Tp, order.Price)

						log.Printf("📊 Progress: %.2f towards TP (Ticket: %d)", tpProgress, order.Ticket)

						// TODO: handle moving tp/sl
						//priceAt20Percent := order.EntryPrice + (order.Tp-order.EntryPrice)*0.20

						// اگر بین 70 تا 90 درصد طی شده بود tp و sl رو تکون بده
						/*if (tpProgress >= 70.0 && tpProgress < 80) || (tpProgress >= 50.0 && tpProgress < 55) {
							log.Printf("🛑 SL updated to price: %.2f", priceAt20Percent)

							shiftAmount := math.Abs(priceAt20Percent - order.EntryPrice)
							newTp := order.Tp + (order.Tp-order.EntryPrice)*0.20
							newSl := order.Sl + shiftAmount

							if math.Abs(order.Tp-newTp) > 50 {

								log.Printf("📊 Progress: %.2f%% | Shift Amount: %.5f | New TP Target: %.5f (Ticket: %d)", tpProgress, shiftAmount, newTp, order.Ticket)

								err = updateOrder(*client, order.Ticket, newSl, newTp)

								fmt.Printf("Update TP/SL error: %s", err)
							}
						}*/

						// ✅ اگر ۹۰ درصد یا بیشتر مسیر طی شده بود، ببند
						if tpProgress >= 90.0 {
							log.Printf("🚨 EARLY EXIT TRIGGERED! Reached %.1f%% of TP. Closing ticket %d immediately.", tpProgress, order.Ticket)
							closeOrder(*client, order.Ticket)
						}
					}

				case 2000:
					log.Printf("🔒 Position CLOSED. Close Price: %.5f | Info: %s", order.Price, order.Comment)

					comment := order.ParsComment()

					profitString := comment["PRF"]
					profit, _ := strconv.ParseFloat(profitString, 64)
					balance := order.Balance

					telegramApiKey := os.Getenv("TELEGRAM_API_KEY")
					telegramChatId := os.Getenv("TELEGRAM_CHAT_ID")
					appName := os.Getenv("APP_NAME")

					telegramService := notification.NewTelegramService(telegramApiKey)

					var orderResult string

					if profit > 0 {
						log.Println("🎯 Closed by Take Profit!")
						orderResult = "TP"
					} else {
						log.Println("🛑 Closed by Stop Loss!")
						orderResult = "SL"
					}

					text := fmt.Sprintf(
						"<b>Close</b> \n"+
							"🔣 Symbol: %s \n"+
							"📱 App: %s\n"+
							"🎯 Target: %s\n"+
							"💵 Gain(dollar): %.3f\n"+
							"💳 Balance: %0.2f"+
							"⏲️ Time: %s\n",
						symbol, appName, orderResult, profit, balance, time.Now().Format("2006-01-02 15:04"))

					telegramService.SendMessage(telegramChatId, text, "HTML")

					err := ticketRepo.RemoveTicket(order.Ticket)
					if err != nil {
						fmt.Println("Remove Ticket error: ", err)
					}

				case 3000:
					log.Println("❌ Ticket not found in history or active positions.")

					err := ticketRepo.RemoveTicket(order.Ticket)
					if err != nil {
						fmt.Println("Remove Ticket error: ", err)
					}
				}
			}
		}
	}
}

func placeOrder(client MTClient, symbol string, signal string, lot, price, tp, sl float64) {

	// PLACE_ORDER|symbol|side|lot|tp|sl
	cmd := fmt.Sprintf("PLACE_ORDER|%s|%s|%.3f|%.3f|%.3f", symbol, signal, lot, tp, sl)
	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("place order failed: ", err)
	}

	telegramApiKey := os.Getenv("TELEGRAM_API_KEY")
	telegramChatId := os.Getenv("TELEGRAM_CHAT_ID")
	appName := os.Getenv("APP_NAME")

	var sideDisplay string
	switch signal {
	case "BUY":
		sideDisplay = "⬆️ BUY"
	case "SELL":
		sideDisplay = "⬇️ SELL"
	}

	telegramService := notification.NewTelegramService(telegramApiKey)
	text := fmt.Sprintf(""+
		"📖 <b>Open</b> \n"+
		"%s \n"+
		"🔣 Symbol: %s \n"+
		"📱 App: %s\n"+
		"💲 Price: %f\n"+
		"🟢 Tp: %f\n"+
		"🔴 Sl: %f\n"+
		"⏲️ Time: %s\n",
		sideDisplay, symbol, appName, price, tp, sl, time.Now().Format("2006-01-02 15:04"))
	telegramService.SendMessage(telegramChatId, text, "HTML")
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

func inquiryOpenOrders(client MTClient, repo *database.FileRepository) {

	allTickets := repo.GetAllTickets()

	if len(allTickets) > 0 {

		for _, ticket := range allTickets {

			if ticket > 0 {

				cmd := fmt.Sprintf("INQUIRY|%d\n", ticket)

				fmt.Printf("sending CMD is: %s", cmd)

				err := client.SendCommand(cmd)
				if err != nil {
					log.Println("Write error:", err)
				}
			}
		}
	}
}

func closeOrder(client MTClient, ticket int64) {
	// CLOSE_ORDER|ticket
	cmd := fmt.Sprintf("CLOSE_ORDER|%d\n", ticket)

	err := client.SendCommand(cmd)
	if err != nil {
		log.Println("❌ close order command failed: ", err)
	} else {
		log.Printf("📤 Sent CLOSE_ORDER command for ticket: %d", ticket)
	}
}

func calculateTPProgress(orderPrice, tpPrice, currentPrice float64) float64 {

	log.Printf("Tp: %.2f Entry: %.2f Current: %.2f", tpPrice, orderPrice, currentPrice)

	totalDistance := math.Abs(tpPrice - orderPrice)
	direction := 0.0
	if tpPrice > orderPrice {
		direction = 1
	} else {
		direction = -1
	}

	coveredDistance := (currentPrice - orderPrice) * direction
	progress := (coveredDistance / totalDistance) * 100

	return progress
}
