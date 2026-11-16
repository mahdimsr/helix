package metatrader

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type TradingGate struct {
	url    string
	active atomic.Bool
	client *http.Client
}

func NewTradingGate(url string) *TradingGate {
	g := &TradingGate{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	g.active.Store(false) // تا اولین چک موفق، ترید نکن (ایمن‌تر)
	return g
}

func (g *TradingGate) Refresh() {
	resp, err := g.client.Get(g.url)
	if err != nil {
		log.Println("❌ gate check failed:", err)
		return // مقدار قبلی می‌ماند
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("❌ gate status:", resp.Status)
		return
	}

	var out struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Println("❌ gate decode:", err)
		return
	}

	if old := g.active.Swap(out.Active); old != out.Active {
		log.Printf("🔁 trading active changed: %v -> %v", old, out.Active)
	}
}

func (g *TradingGate) IsActive() bool { return g.active.Load() }
