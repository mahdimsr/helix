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

	symbol := "BTCUSD"
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

					// check profit to handle order
					/*if order.Profit > 0 {
						closeOrder(*client, order.Ticket)
						err = updateOrder(*client, order.Ticket, newSl, newTp)

					}*/

					buffer := 8.0
					isBuy := order.Side == "BUY"
					progressPercent := calculateTPProgress(order.EntryPrice, order.Tp, order.Price)

					log.Printf("Progress of Tp: %.2f", progressPercent)
					log.Printf("Pnl : %.2f", order.Profit)

					if progressPercent >= 20.0 {

						newSl := riskFree(order, buffer)

						needsUpdate := false
						if isBuy && order.Sl < newSl {
							// SL هنوز خیلی پایین است، باید بالا بیاید
							needsUpdate = true
						} else if !isBuy && order.Sl > newSl {
							needsUpdate = true // قیمت پایین آمده، SL هم باید پایین‌تر بیاید
						}

						if needsUpdate {
							log.Println("Need to update SL in 20 percent")
							err = updateOrder(*client, order.Ticket, newSl, order.Tp)
							if err != nil {
								log.Printf("❌ Update TP/SL error for ticket %d: %s", order.Ticket, err)
							} else {
								log.Printf("✅ Successfully updated TP/SL for ticket %d", order.Ticket)
							}
						} else {
							log.Println("Noooo Need to update SL in 20 percent")
						}
					}

					if progressPercent >= 50 {

						newSl := CalculatePriceAtPercent(order.EntryPrice, order.Tp, 20)

						needsUpdate := false
						if isBuy && order.Sl < newSl {
							needsUpdate = true
						} else if !isBuy && order.Sl > newSl {
							needsUpdate = true
						}

						if needsUpdate {

							log.Println("Need to update SL in 50 percent")

							err = updateOrder(*client, order.Ticket, newSl, order.Tp)
							if err != nil {
								log.Printf("❌ Update TP/SL error for ticket %d: %s", order.Ticket, err)
							} else {
								log.Printf("✅ Successfully updated TP/SL for ticket %d", order.Ticket)
							}
						} else {
							log.Println("Noooo Need to update SL in 50 percent")
						}
					}

					if progressPercent >= 80 {

						newSl := CalculatePriceAtPercent(order.EntryPrice, order.Tp, 60)

						totalDistance := math.Abs(order.Tp - order.EntryPrice)
						extensionAmount := totalDistance * 0.10

						var newTp float64
						if isBuy {
							newTp = order.Tp + extensionAmount
						} else {
							newTp = order.Tp - extensionAmount
						}

						needsUpdate := false
						if isBuy && newTp > order.Tp {
							needsUpdate = true
						} else if !isBuy && newTp < order.Tp {
							needsUpdate = true
						}

						if needsUpdate {
							err = updateOrder(*client, order.Ticket, newSl, newTp)
							if err == nil {
								order.Tp = newTp
								log.Printf("🎯 TP Extended to: %.5f", newTp)
							}
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

func calculateTPProgress(entryPrice, tpPrice, currentPrice float64) float64 {
	// فاصله کل تا هدف (این مقدار برای BUY مثبت و برای SELL منفی است)
	totalDistance := tpPrice - entryPrice
	if totalDistance == 0 {
		return 0
	}

	// فاصله طی شده از نقطه ورود
	// (اگر در جهت سود باشیم هم‌علامت با totalDistance است، اگر در ضرر باشیم خلاف علامت آن است)
	currentDistance := currentPrice - entryPrice

	// محاسبه درصد پیشرفت
	progress := (currentDistance / totalDistance) * 100.0

	// محدود کردن حداکثر به 100 درصد (اگر قیمت از TP هم رد شد)
	if progress > 100.0 {
		return 100.0
	}

	// محدود کردن حداقل به -100 درصد (اختیاری، برای جلوگیری از اعداد خیلی بزرگ منفی در ضرر سنگین)
	if progress < -100.0 {
		return -100.0
	}

	return progress
}

func riskFree(order OrderResult, buffer float64) (newSl float64) {

	newSl = CalculatePriceAtPercent(order.EntryPrice, order.Tp, 0)

	if order.Side == "BUY" {
		return newSl + buffer
	}

	return newSl - buffer
}

func CalculatePriceAtPercent(entryPrice, tpPrice, percent float64) float64 {
	return entryPrice + (tpPrice-entryPrice)*(percent/100.0)
}
