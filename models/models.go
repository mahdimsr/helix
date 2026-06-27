package models

import (
	"encoding/json"
	"fmt"
)

type Candle struct {
	Open   string
	High   string
	Close  string
	Low    string
	Volume string
	Time   int64
	Symbol string
}

func (c *Candle) UnmarshalJSON(data []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal raw candle data: %w", err)
	}

	if len(raw) < 11 {
		return fmt.Errorf("invalid candle data length: expected at least 11 elements, got %d", len(raw))
	}

	// Helper to convert interface{} to string, returns empty string on failure
	getString := func(v interface{}) string {
		s, _ := v.(string)
		return s
	}

	getInt64 := func(v interface{}) int64 {
		f, _ := v.(float64)
		return int64(f)
	}

	c.Open = getString(raw[1])
	c.High = getString(raw[2])
	c.Low = getString(raw[3])
	c.Close = getString(raw[4])
	c.Volume = getString(raw[5])
	c.Time = getInt64(raw[6])

	return nil
}
