package indicators

import (
	"helix/models"
	"math"
	"sort"
)

func OptimizeDualUTBot(candles []models.Candle, exitMode ExitMode) []OptimizationResult {

	n := len(candles)
	if n == 0 {
		return nil
	}

	// 2. ساخت تمام ترکیبات ممکن برای پارامترها (ATR 5-80, Sens 1-10)
	var params []UTBotParam
	for atr := 5; atr <= 80; atr += 5 {
		for sens := 1.0; sens <= 10.0; sens += 1.0 {
			params = append(params, UTBotParam{ATR: atr, Sens: sens})
		}
	}

	// 3. پیش‌محاسبه سیگنال‌ها برای جلوگیری از تکرار محاسبات ریاضی در حلقه‌های تو در تو
	statesMap := make([][]int8, len(params))
	for i, p := range params {
		statesMap[i] = precalcUTBotStates(candles, p.Sens, p.ATR)
	}

	var results []OptimizationResult
	tpPercent := 3.0 // درصد (برای حالت ExitTP)
	leverage := 20.0

	// 4. تست تمامی ترکیبات Condition و Signal
	for cIdx, condParam := range params {
		condStates := statesMap[cIdx]

		for sIdx, sigParam := range params {
			sigStates := statesMap[sIdx]

			trades := 0
			wins := 0
			gain := 0.0

			inTrade := false
			tradeType := int8(0) // 1: Long, -1: Short
			var entryPrice, tpPrice, liqPrice float64

			for i := 1; i < n; i++ {
				// الف: بررسی خروج اگر داخل معامله هستیم
				if inTrade {
					// --- 1. چک لیکوئید شدن (اولویت اول) ---
					liquidated := (tradeType == 1 && candles[i].Low <= liqPrice) ||
						(tradeType == -1 && candles[i].High >= liqPrice)
					if liquidated {
						trades++
						gain += -100.0
						inTrade = false
						continue
					}

					// --- 2. شرط خروج بر اساس exitMode ---
					if exitMode == ExitTP {
						// حالت TP ثابت
						if tradeType == 1 && candles[i].High >= tpPrice {
							trades++
							wins++
							gain += tpPercent * leverage
							inTrade = false
							continue
						}
						if tradeType == -1 && candles[i].Low <= tpPrice {
							trades++
							wins++
							gain += tpPercent * leverage
							inTrade = false
							continue
						}

					} else if exitMode == ExitSignalReverse {
						// حالت خروج با سیگنال معکوس اندیکاتور سیگنال
						if tradeType == 1 && sigStates[i] == -1 && sigStates[i-1] != -1 {
							// سیگنال sell شد → خروج
							trades++
							pnl := ((candles[i].Close - entryPrice) / entryPrice) * 100 * leverage
							if pnl < -100.0 {
								pnl = -100.0
							}
							gain += pnl
							if pnl > 0 {
								wins++
							}
							inTrade = false
							continue
						}
						if tradeType == -1 && sigStates[i] == 1 && sigStates[i-1] != 1 {
							// سیگنال buy شد → خروج
							trades++
							pnl := ((entryPrice - candles[i].Close) / entryPrice) * 100 * leverage
							if pnl < -100.0 {
								pnl = -100.0
							}
							gain += pnl
							if pnl > 0 {
								wins++
							}
							inTrade = false
							continue
						}
					}

					// --- 3. خروج اضطراری با تغییر روند اندیکاتور شرط ---
					if (tradeType == 1 && condStates[i] == -1) ||
						(tradeType == -1 && condStates[i] == 1) {
						trades++
						var pnl float64
						if tradeType == 1 {
							pnl = ((candles[i].Close - entryPrice) / entryPrice) * 100 * leverage
						} else {
							pnl = ((entryPrice - candles[i].Close) / entryPrice) * 100 * leverage
						}
						if pnl < -100.0 {
							pnl = -100.0
						}
						gain += pnl
						if pnl > 0 {
							wins++
						}
						inTrade = false
					}
				}

				// ب: بررسی ورود اگر داخل معامله نیستیم
				if !inTrade {
					// شرط ورود Long: اندیکاتور شرط Long باشد + سیگنال تازه Long شده باشد
					if condStates[i] == 1 && sigStates[i] == 1 && sigStates[i-1] != 1 {
						inTrade = true
						tradeType = 1
						entryPrice = candles[i].Close
						tpPrice = entryPrice * (1 + tpPercent/100)
						liqPrice = entryPrice * (1.0 - 1.0/leverage)
					} else
					// شرط ورود Short: اندیکاتور شرط Short باشد + سیگنال تازه Short شده باشد
					if condStates[i] == -1 && sigStates[i] == -1 && sigStates[i-1] != -1 {
						inTrade = true
						tradeType = -1
						entryPrice = candles[i].Close
						tpPrice = entryPrice * (1 - tpPercent/100)
						liqPrice = entryPrice * (1.0 + 1.0/leverage)
					}
				}
			}

			// بستن معامله باز در انتهای داده‌ها
			if inTrade {
				trades++
				var pnl float64
				if tradeType == 1 {
					pnl = ((candles[n-1].Close - entryPrice) / entryPrice) * 100 * leverage
				} else {
					pnl = ((entryPrice - candles[n-1].Close) / entryPrice) * 100 * leverage
				}
				if pnl < -100.0 {
					pnl = -100.0
				}
				gain += pnl
				if pnl > 0 {
					wins++
				}
			}

			// 5. ذخیره نتایج در صورت وجود معامله
			if trades > 0 {
				winRate := (float64(wins) / float64(trades)) * 100
				results = append(results, OptimizationResult{
					CondATR:       condParam.ATR,
					CondSens:      condParam.Sens,
					SigATR:        sigParam.ATR,
					SigSens:       sigParam.Sens,
					TotalTrades:   trades,
					WinTrades:     wins,
					WinRate:       winRate,
					TotalGainPerc: gain,
				})
			}
		}
	}

	// 6. مرتب‌سازی نتایج: اولویت با WinRate بالا، سپس Gain بالا
	sort.Slice(results, func(i, j int) bool {
		if results[i].WinRate == results[j].WinRate {
			return results[i].TotalGainPerc > results[j].TotalGainPerc
		}
		return results[i].WinRate > results[j].WinRate
	})

	// برگرداندن 10 نتیجه برتر
	if len(results) > 10 {
		return results[:10]
	}
	return results
}

func precalcUTBotStates(candles []models.Candle, sensitivity float64, atrPeriod int) []int8 {
	n := len(candles)
	states := make([]int8, n)
	if n == 0 {
		return states
	}

	trs := make([]float64, n)
	atrs := make([]float64, n)
	var prevStop float64
	alpha := 1.0 / float64(atrPeriod)
	sum := 0.0
	currentState := int8(0)

	for i := 0; i < n; i++ {
		high := candles[i].High
		low := candles[i].Low
		closePrice := candles[i].Close

		if i == 0 {
			trs[i] = high - low
			sum += trs[i]
			atrs[i] = sum
			prevStop = closePrice
			states[i] = 0
			continue
		}

		prevClose := candles[i-1].Close
		trs[i] = math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

		if i < atrPeriod {
			sum += trs[i]
			atrs[i] = sum / float64(i+1)
		} else {
			atrs[i] = alpha*trs[i] + (1.0-alpha)*atrs[i-1]
		}

		nLoss := sensitivity * atrs[i]
		var stop float64

		if prevClose > prevStop && closePrice > prevStop {
			stop = math.Max(prevStop, closePrice-nLoss)
		} else if prevClose < prevStop && closePrice < prevStop {
			stop = math.Min(prevStop, closePrice+nLoss)
		} else if closePrice > prevStop {
			stop = closePrice - nLoss
		} else {
			stop = closePrice + nLoss
		}

		above := prevClose <= prevStop && closePrice > stop
		below := prevStop <= prevClose && stop > closePrice

		buySignal := closePrice > stop && above
		sellSignal := closePrice < stop && below

		if buySignal {
			currentState = 1
		} else if sellSignal {
			currentState = -1
		}

		states[i] = currentState
		prevStop = stop
	}

	return states
}
