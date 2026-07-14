package models

type Position string

const (
	BuyPosition  Position = "BUY"
	SellPosition Position = "SELL"
	NonePosition Position = "NONE"
)

func CalcGainPercent(entry, exit float64, leverage int, fee float64, isLong bool) float64 {
	var raw float64
	if isLong {
		raw = (exit - entry) / entry * 100
	} else {
		raw = (entry - exit) / entry * 100
	}

	lev := float64(leverage)

	totalFee := 2 * (fee / 100) * lev

	return raw*lev - totalFee
}
