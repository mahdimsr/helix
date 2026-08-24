package indicators

type Signal string

const (
	BuySignal  Signal = "BUY"
	SellSignal Signal = "SELL"
	NoneSignal Signal = "NONE"
)

type OptimizationResult struct {
	CondATR       int     `json:"cond_atr"`
	CondSens      float64 `json:"cond_sens"`
	SigATR        int     `json:"sig_atr"`
	SigSens       float64 `json:"sig_sens"`
	TotalTrades   int     `json:"total_trades"`
	WinTrades     int     `json:"win_trades"`
	WinRate       float64 `json:"win_rate"`
	TotalGainPerc float64 `json:"total_gain_perc"`
}

type UTBotParam struct {
	ATR  int
	Sens float64
}

type ExitMode int

const (
	ExitTP ExitMode = iota // خروج با حد سود ثابت
	ExitSignalReverse
)
