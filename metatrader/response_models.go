package metatrader

import (
	"bufio"
	"encoding/json"
	"helix/models"
	"log"
	"net"
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
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	log.Printf("RAW received (%d bytes): %q", len(line), line)

	return line[:len(line)-1], nil // trim \n
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
