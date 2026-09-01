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

func GenerateInteractiveChart(results strategy.BacktestOutput, filename string) error {
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

	pointsJSON, err := json.Marshal(points)
	if err != nil {
		return err
	}

	// قالب HTML با قابلیت Tooltip کامل و طراحی زیبا
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Backtest Optimization Chart</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            padding: 30px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 15px;
            padding: 30px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
        }
        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 10px;
            font-size: 28px;
        }
        .subtitle {
            text-align: center;
            color: #666;
            margin-bottom: 25px;
            font-size: 14px;
        }
        .stats {
            display: flex;
            justify-content: space-around;
            margin-bottom: 25px;
            padding: 15px;
            background: #f8f9fa;
            border-radius: 10px;
        }
        .stat-item {
            text-align: center;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #667eea;
        }
        .stat-label {
            font-size: 12px;
            color: #666;
            text-transform: uppercase;
        }
        .chart-wrapper {
            position: relative;
            height: 600px;
        }
        .legend-info {
            margin-top: 20px;
            padding: 15px;
            background: #fff3cd;
            border-left: 4px solid #ffc107;
            border-radius: 5px;
            font-size: 13px;
            color: #856404;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>📊 Backtest Optimization Matrix</h1>
        <p class="subtitle">Hover over any point to see TP/SL details</p>
        
        <div class="stats">
            <div class="stat-item">
                <div class="stat-value" id="totalPoints">0</div>
                <div class="stat-label">Total Combinations</div>
            </div>
            <div class="stat-item">
                <div class="stat-value" id="bestGain">$0</div>
                <div class="stat-label">Best Gain</div>
            </div>
            <div class="stat-item">
                <div class="stat-value" id="bestCombo">-</div>
                <div class="stat-label">Best TP/SL</div>
            </div>
        </div>

        <div class="chart-wrapper">
            <canvas id="myChart"></canvas>
        </div>

        <div class="legend-info">
            💡 <strong>راهنما:</strong> موس را روی هر نقطه ببرید تا مقادیر TP، SL، سود و تعداد تریدها را ببینید.
            نقاط در ناحیه <strong>بالا-راست</strong> بهترین عملکرد را دارند (سود بالا + تعداد ترید زیاد).
        </div>
    </div>

    <script>
        const chartData = %s;

        // محاسبه آمار
        let bestGain = -Infinity;
        let bestPoint = null;
        chartData.forEach(pt => {
            if (pt.x > bestGain) {
                bestGain = pt.x;
                bestPoint = pt;
            }
        });

        document.getElementById('totalPoints').textContent = chartData.length;
        document.getElementById('bestGain').textContent = '$' + bestGain.toFixed(2);
        if (bestPoint) {
            document.getElementById('bestCombo').textContent = 'TP:' + bestPoint.tp + '/SL:' + bestPoint.sl;
        }

        // رنگ‌بندی نقاط بر اساس سود (سبز برای سود، قرمز برای ضرر)
        const colors = chartData.map(pt => {
            if (pt.x > 0) {
                const intensity = Math.min(pt.x / bestGain, 1);
                return 'rgba(' + Math.round(34 - intensity*20) + ', ' + Math.round(139 + intensity*50) + ', ' + Math.round(34 + intensity*50) + ', 0.7)';
            } else {
                return 'rgba(220, 53, 69, 0.6)';
            }
        });

        const ctx = document.getElementById('myChart').getContext('2d');
        new Chart(ctx, {
            type: 'scatter',
            data: {
                datasets: [{
                    label: 'TP/SL Combinations',
                    data: chartData,
                    backgroundColor: colors,
                    borderColor: colors.map(c => c.replace('0.7', '1').replace('0.6', '1')),
                    borderWidth: 1,
                    pointRadius: 7,
                    pointHoverRadius: 12,
                    pointHoverBorderWidth: 3,
                    pointHoverBorderColor: '#000'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'nearest',
                    intersect: true
                },
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
                            title: function(context) {
                                return '🎯 Trade Combination';
                            },
                            label: function(context) {
                                const pt = context.raw;
                                return [
                                    '💰 TP: $' + pt.tp + '  |  SL: $' + pt.sl,
                                    '📈 Net Gain: $' + pt.x.toFixed(2),
                                    '🔢 Trades: ' + pt.y,
                                    '📊 R/R Ratio: ' + (pt.tp / pt.sl).toFixed(2)
                                ];
                            },
                            afterLabel: function(context) {
                                const pt = context.raw;
                                if (pt.x > 0) {
                                    return '✅ Profitable';
                                }
                                return '❌ Loss';
                            }
                        }
                    }
                },
                scales: {
                    x: {
                        type: 'linear',
                        position: 'bottom',
                        title: {
                            display: true,
                            text: 'Net Gain ($)',
                            font: { size: 14, weight: 'bold' }
                        },
                        grid: { color: 'rgba(0,0,0,0.05)' }
                    },
                    y: {
                        type: 'linear',
                        title: {
                            display: true,
                            text: 'Number of Trades',
                            font: { size: 14, weight: 'bold' }
                        },
                        grid: { color: 'rgba(0,0,0,0.05)' }
                    }
                }
            }
        });
    </script>
</body>
</html>`

	finalHTML := fmt.Sprintf(htmlTemplate, string(pointsJSON))

	err = os.WriteFile(filename, []byte(finalHTML), 0644)
	if err != nil {
		return err
	}

	fmt.Printf("✅ فایل نمودار تعاملی در '%s' ذخیره شد.\n", filename)
	fmt.Println("💡 کافیست روی فایل دابل‌کلیک کنید تا در مرورگر باز شود.")
	return nil
}
