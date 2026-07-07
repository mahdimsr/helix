package metatrader

import (
	"bufio"
	"encoding/json"
	"helix/models"
	"log"
	"net"
	"time"
)

type MTClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

type OrderResult struct {
	SUCCESS bool    `json:"success"`
	Retcode int     `json:"retcode"`
	Price   float64 `json:"price"`
	Tp      float64 `json:"tp"`
	Sl      float64 `json:"sl"`
	Ticket  int64   `json:"ticket"`
	Comment string  `json:"comment"`
}

type SocketResult struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func NewMT5Client(conn net.Conn) *MTClient {
	return &MTClient{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, 1<<20),
	}
}

func (c *MTClient) SendCommand(cmd string) error {
	_, err := c.conn.Write([]byte(cmd + "\n"))
	return err
}

func (c *MTClient) ReadResponse() (string, error) {
	// timeout 30 ثانیه برای read
	if err := c.conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return "", err
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		// اگر timeout بود، خطای خاصی برنمی‌گردونه فقط err داره
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", nil // timeout معمولی، نه خطای واقعی
		}
		return "", err
	}
	return line, nil
}

func (socketResult SocketResult) fetchDataAsCandle() []models.Candle {

	var candles []models.Candle
	if err := json.Unmarshal(socketResult.Data, &candles); err != nil {
		log.Println("JSON parse error:", err)
	}

	return candles
}

func (socketResult SocketResult) fetchDataAsOrder() OrderResult {

	var orderResult OrderResult
	if err := json.Unmarshal(socketResult.Data, &orderResult); err != nil {
		log.Println("JSON parse error:", err)
	}

	return orderResult
}
