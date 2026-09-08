package chart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"helix/strategy"
	"io"
	"net/http"
	"os"
)

type ChartPoint struct {
	X       float64 `json:"x"`
	Y       int     `json:"y"`
	TP      float64 `json:"tp"`
	SL      float64 `json:"sl"`
	WinRate float64 `json:"winRate"`
}

type ChartDataset struct {
	Label            string       `json:"label"`
	Data             []ChartPoint `json:"data"`
	BackgroundColor  string       `json:"backgroundColor"`
	PointRadius      int          `json:"pointRadius"`
	PointHoverRadius int          `json:"pointHoverRadius"`
}

type ChartData struct {
	Datasets []ChartDataset `json:"datasets"`
}

type ChartTitle struct {
	Display bool   `json:"display"`
	Text    string `json:"text"`
}

type ChartAxisTitle struct {
	Display bool   `json:"display"`
	Text    string `json:"text"`
}

type ChartScales struct {
	X map[string]interface{} `json:"x"`
	Y map[string]interface{} `json:"y"`
}

type ChartTooltip struct {
	Callbacks map[string]string `json:"callbacks"`
}

type ChartPlugins struct {
	Title   ChartTitle   `json:"title"`
	Tooltip ChartTooltip `json:"tooltip"`
}

type ChartOptions struct {
	Responsive bool         `json:"responsive"`
	Plugins    ChartPlugins `json:"plugins"`
	Scales     ChartScales  `json:"scales"`
}

type ChartConfig struct {
	Type    string       `json:"type"`
	Data    ChartData    `json:"data"`
	Options ChartOptions `json:"options"`
}

type QuickChartResponse struct {
	Success bool   `json:"success"`
	URL     string `json:"url"`
}

func GenerateShortChartURL(results *strategy.BacktestOutput) (string, error) {
	var points []ChartPoint

	for i, tp := range results.TPValues {
		for j, sl := range results.SLValues {
			points = append(points, ChartPoint{
				X:  results.GainMatrix[i][j],
				Y:  results.CountMatrix[i][j],
				TP: tp,
				SL: sl,
			})
		}
	}

	config := ChartConfig{
		Type: "scatter",
		Data: ChartData{
			Datasets: []ChartDataset{
				{
					Label:            "TP/SL Combinations",
					Data:             points,
					BackgroundColor:  "rgba(54, 162, 235, 0.6)",
					PointRadius:      5, // کمی کوچک‌تر برای جلوگیری از شلوغی
					PointHoverRadius: 9,
				},
			},
		},
		Options: ChartOptions{
			Responsive: true,
			Plugins: ChartPlugins{
				Title: ChartTitle{Display: true, Text: "Backtest Optimization: Net Gain vs Trade Count"},
				Tooltip: ChartTooltip{
					Callbacks: map[string]string{
						"label": `function(context) {
							var pt = context.raw;
							return 'TP: $' + pt.tp + ' | SL: $' + pt.sl + '  =>  Gain: $' + pt.x.toFixed(2) + ' | Trades: ' + pt.y;
						}`,
					},
				},
			},
			Scales: ChartScales{
				X: map[string]interface{}{"title": ChartAxisTitle{Display: true, Text: "Net Gain ($)"}, "type": "linear", "position": "bottom"},
				Y: map[string]interface{}{"title": ChartAxisTitle{Display: true, Text: "Number of Trades"}, "type": "linear", "position": "left"},
			},
		},
	}

	jsonBytes, err := json.Marshal(map[string]interface{}{
		"chart":           config,
		"width":           800,
		"height":          600,
		"format":          "png",
		"backgroundColor": "#ffffff",
	})
	if err != nil {
		return "", err
	}

	// ارسال درخواست POST به سرور QuickChart
	resp, err := http.Post("https://quickchart.io/chart/create", "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "post error", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "read all error", err
	}

	var result QuickChartResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "unmarshal error", err
	}

	if !result.Success {
		return "", fmt.Errorf("QuickChart API error: %s", string(body))
	}

	return result.URL, nil
}

func GenerateInteractiveChart(results *strategy.BacktestOutput, filename string) error {
	var points []ChartPoint
	for i, tp := range results.TPValues {
		for j, sl := range results.SLValues {
			points = append(points, ChartPoint{
				X:       results.GainMatrix[i][j],
				Y:       results.CountMatrix[i][j],
				TP:      tp,
				SL:      sl,
				WinRate: results.WinRateMatrix[i][j],
			})
		}
	}

	if len(points) == 0 {
		return fmt.Errorf("هیچ داده‌ای برای نمایش وجود ندارد")
	}

	pointsJSON, err := json.Marshal(points)
	if err != nil {
		return err
	}

	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Backtest Optimization Chart</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, sans-serif; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); min-height: 100vh; padding: 30px; }
        .container { max-width: 1400px; margin: 0 auto; background: white; border-radius: 15px; padding: 30px; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
        h1 { text-align: center; color: #333; margin-bottom: 10px; font-size: 28px; }
        .subtitle { text-align: center; color: #666; margin-bottom: 25px; font-size: 14px; }
        .stats { display: flex; justify-content: space-around; margin-bottom: 25px; padding: 15px; background: #f8f9fa; border-radius: 10px; flex-wrap: wrap; }
        .stat-item { text-align: center; padding: 10px; min-width: 120px; }
        .stat-value { font-size: 24px; font-weight: bold; color: #667eea; }
        .stat-label { font-size: 12px; color: #666; text-transform: uppercase; margin-top: 5px; }
        
        .filter-controls { background: #f8f9fa; border-radius: 10px; padding: 20px; margin-bottom: 20px; border: 2px solid #e9ecef; }
        .filter-row { display: flex; align-items: center; gap: 15px; margin-bottom: 15px; }
        .filter-row label { min-width: 180px; font-weight: 600; color: #333; }
        .filter-row input[type="range"] { flex: 1; height: 8px; border-radius: 4px; background: #ddd; outline: none; -webkit-appearance: none; }
        .filter-row input[type="range"]::-webkit-slider-thumb { -webkit-appearance: none; width: 22px; height: 22px; border-radius: 50%%; background: #667eea; cursor: pointer; border: 2px solid white; box-shadow: 0 2px 4px rgba(0,0,0,0.2); }
        .filter-value { min-width: 60px; font-weight: bold; color: #dc3545; font-size: 18px; text-align: right; }
        .filter-info { text-align: center; padding: 10px; background: white; border-radius: 5px; margin-top: 10px; color: #555; font-size: 14px; }
        
        .chart-wrapper { position: relative; height: 600px; }
        .legend-info { margin-top: 20px; padding: 15px; background: #fff3cd; border-left: 4px solid #ffc107; border-radius: 5px; font-size: 13px; color: #856404; }
        .color-legend { display: flex; justify-content: center; gap: 20px; margin-top: 15px; flex-wrap: wrap; }
        .color-item { display: flex; align-items: center; gap: 8px; font-size: 13px; }
        .color-box { width: 20px; height: 20px; border-radius: 4px; border: 1px solid #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Backtest Optimization Matrix</h1>
        <p class="subtitle">Hover over any point to see TP/SL details</p>
        
        <div class="stats">
            <div class="stat-item"><div class="stat-value" id="totalPoints">0</div><div class="stat-label">Total Combinations</div></div>
            <div class="stat-item"><div class="stat-value" id="bestGain">$0</div><div class="stat-label">Best Gain</div></div>
            <div class="stat-item"><div class="stat-value" id="bestCombo">-</div><div class="stat-label">Best TP/SL</div></div>
            <div class="stat-item"><div class="stat-value" id="bestWinRate">0%%</div><div class="stat-label">Best Win Rate</div></div>
            <div class="stat-item"><div class="stat-value" id="bestRR">0</div><div class="stat-label">Best R/R</div></div>
        </div>

        <div class="filter-controls">
            <div class="filter-row">
                <label for="winRateSlider">🎯 Minimum Win Rate:</label>
                <input type="range" id="winRateSlider" min="0" max="100" value="0" step="5">
                <span id="winRateValue" class="filter-value">0%%</span>
            </div>
            <div class="filter-row">
                <label for="minTradesSlider">🔢 Minimum Trades:</label>
                <input type="range" id="minTradesSlider" min="0" max="100" value="0" step="1">
                <span id="minTradesValue" class="filter-value">0</span>
            </div>
            <div class="filter-row">
                <label for="rrSlider">📊 Minimum R/R Ratio:</label>
                <input type="range" id="rrSlider" min="1" max="20" value="1" step="0.5">
                <span id="rrValue" class="filter-value">1.0</span>
            </div>
            <div class="filter-info">
                Showing <strong id="visibleCount" style="color:#667eea; font-size:18px;">0</strong> of <strong id="totalCount">0</strong> combinations
            </div>
        </div>

        <div class="chart-wrapper">
            <canvas id="myChart"></canvas>
        </div>

        <div class="color-legend">
            <div class="color-item"><div class="color-box" style="background: rgba(34, 189, 34, 0.8);"></div><span>High Win Rate (≥70%%)</span></div>
            <div class="color-item"><div class="color-box" style="background: rgba(100, 180, 100, 0.7);"></div><span>Medium Win Rate (50-70%%)</span></div>
            <div class="color-item"><div class="color-box" style="background: rgba(220, 53, 69, 0.7);"></div><span>Low Win Rate (<50%%)</span></div>
        </div>

        <div class="legend-info">
            💡 <strong>راهنما:</strong> از اسلایدرها برای فیلتر کردن نتایج استفاده کنید. R/R Ratio = TP / SL
        </div>
    </div>

    <script>
        const rawData = %s;
        
        if (rawData.length === 0) {
            document.body.innerHTML = '<h1 style="color: red; text-align: center; padding: 50px;">❌ No data to display</h1>';
        }

        const allData = rawData.map(pt => ({
            x: Number(pt.x),
            y: Number(pt.y),
            tp: Number(pt.tp),
            sl: Number(pt.sl),
            winRate: Number(pt.winRate),
            rr: Number(pt.tp) / Number(pt.sl) // محاسبه R/R
        }));

        let currentChart = null;

        function applyFilters() {
            const minWinRate = Number(document.getElementById('winRateSlider').value);
            const minTrades = Number(document.getElementById('minTradesSlider').value);
            const minRR = Number(document.getElementById('rrSlider').value);
            
            document.getElementById('winRateValue').textContent = minWinRate + '%%';
            document.getElementById('minTradesValue').textContent = minTrades;
            document.getElementById('rrValue').textContent = minRR.toFixed(1);
            
            const filteredData = allData.filter(pt => {
                return pt.winRate >= minWinRate && pt.y >= minTrades && pt.rr >= minRR;
            });
            
            document.getElementById('visibleCount').textContent = filteredData.length;
            document.getElementById('totalCount').textContent = allData.length;
            
            let bestGain = -Infinity;
            let bestPoint = null;
            let bestWinRate = 0;
            let bestRR = 0;
            
            filteredData.forEach(pt => {
                if (pt.x > bestGain) { bestGain = pt.x; bestPoint = pt; }
                if (pt.winRate > bestWinRate) bestWinRate = pt.winRate;
                if (pt.rr > bestRR) bestRR = pt.rr;
            });
            
            document.getElementById('bestGain').textContent = bestGain === -Infinity ? '$0' : '$' + bestGain.toFixed(2);
            document.getElementById('bestCombo').textContent = bestPoint ? 'TP:' + bestPoint.tp + '/SL:' + bestPoint.sl : '-';
            document.getElementById('bestWinRate').textContent = bestWinRate.toFixed(1) + '%%';
            document.getElementById('bestRR').textContent = bestRR.toFixed(1);
            
            const colors = filteredData.map(pt => {
                if (pt.winRate >= 70) {
                    const intensity = Math.min((pt.winRate - 70) / 30, 1);
                    return 'rgba(34, ' + Math.round(139 + intensity*50) + ', 34, 0.8)';
                } else if (pt.winRate >= 50) {
                    return 'rgba(100, 180, 100, 0.7)';
                } else {
                    const intensity = Math.min((50 - pt.winRate) / 50, 1);
                    return 'rgba(220, ' + Math.round(53 + (1-intensity)*50) + ', 69, 0.7)';
                }
            });
            
            if (currentChart) currentChart.destroy();
            
            const ctx = document.getElementById('myChart').getContext('2d');
            currentChart = new Chart(ctx, {
                type: 'scatter',
                data: {
                    datasets: [{
                        label: 'TP/SL Combinations',
                        data: filteredData,
                        backgroundColor: colors,
                        borderColor: colors.map(c => c.replace('0.7', '1').replace('0.8', '1')),
                        borderWidth: 1,
                        pointRadius: 8,
                        pointHoverRadius: 12,
                        pointHoverBorderWidth: 3,
                        pointHoverBorderColor: '#000'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    animation: { duration: 300 },
                    interaction: { mode: 'nearest', intersect: true },
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            backgroundColor: 'rgba(0, 0, 0, 0.9)',
                            titleFont: { size: 14, weight: 'bold' },
                            bodyFont: { size: 13 },
                            padding: 15,
                            cornerRadius: 8,
                            displayColors: false,
                            callbacks: {
                                title: () => '🎯 Trade Combination',
                                label: function(context) {
                                    const pt = context.raw;
                                    return [
                                        '💰 TP: $' + pt.tp + '  |  SL: $' + pt.sl,
                                        '📈 Net Gain: $' + pt.x.toFixed(2),
                                        '🔢 Trades: ' + pt.y,
                                        '🎯 Win Rate: ' + pt.winRate.toFixed(1) + '%%',
                                        '📊 R/R Ratio: ' + pt.rr.toFixed(2)
                                    ];
                                },
                                afterLabel: function(context) {
                                    const pt = context.raw;
                                    if (pt.x > 0 && pt.winRate >= 70) return '🏆 Excellent (WR ≥ 70%%)';
                                    if (pt.x > 0 && pt.winRate >= 50) return '✅ Good';
                                    if (pt.x > 0) return '⚠️ Risky';
                                    return '❌ Loss';
                                }
                            }
                        }
                    },
                    scales: {
                        x: { type: 'linear', position: 'bottom', title: { display: true, text: 'Net Gain ($)', font: { size: 14, weight: 'bold' } }, grid: { color: 'rgba(0,0,0,0.05)' } },
                        y: { type: 'linear', title: { display: true, text: 'Number of Trades', font: { size: 14, weight: 'bold' } }, grid: { color: 'rgba(0,0,0,0.05)' } }
                    }
                }
            });
        }

        document.getElementById('winRateSlider').addEventListener('input', applyFilters);
        document.getElementById('minTradesSlider').addEventListener('input', applyFilters);
        document.getElementById('rrSlider').addEventListener('input', applyFilters);

        applyFilters();
    </script>
</body>
</html>`

	finalHTML := fmt.Sprintf(htmlTemplate, string(pointsJSON))

	err = os.WriteFile(filename, []byte(finalHTML), 0644)
	if err != nil {
		return err
	}

	fmt.Printf("✅ فایل نمودار تعاملی با فیلتر R/R در '%s' ذخیره شد.\n", filename)
	fmt.Println("💡 فایل را در مرورگر باز کنید و از اسلایدر R/R برای فیلتر کردن استفاده کنید.")
	return nil
}
