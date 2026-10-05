package chart

import (
	"encoding/json"
	"fmt"
	"helix/models"
	"os"
)

// BodyGroupPoint is a single data point representing one (bodyGroup, TP, SL) combination.
type BodyGroupPoint struct {
	X         float64 `json:"x"` // Net gain ($)
	Y         int     `json:"y"` // Trade count
	TP        float64 `json:"tp"`
	SL        float64 `json:"sl"`
	WinRate   float64 `json:"winRate"`
	BodyGroup int     `json:"bodyGroup"` // 0–10
	BodyLabel string  `json:"bodyLabel"` // "0–10%", "10–20%", …
	RR        float64 `json:"rr"`        // TP / SL
}

// bodyGroupLabel returns a human-readable label for a body group index.
func bodyGroupLabel(g int) string {
	labels := [...]string{
		"0–10%", "10–20%", "20–30%", "30–40%", "40–50%",
		"50–60%", "60–70%", "70–80%", "80–90%", "90–100%", "100%+",
	}
	if g >= 0 && g < len(labels) {
		return labels[g]
	}
	return fmt.Sprintf("Group %d", g)
}

// bodyGroupOf maps a body-percentage value (0–1+) to a group index 0–10.
// This must mirror GetBodyGroupIndex in the walking package.
func bodyGroupOf(bp float64) int {
	switch {
	case bp < 0.10:
		return 0
	case bp < 0.20:
		return 1
	case bp < 0.30:
		return 2
	case bp < 0.40:
		return 3
	case bp < 0.50:
		return 4
	case bp < 0.60:
		return 5
	case bp < 0.70:
		return 6
	case bp < 0.80:
		return 7
	case bp < 0.90:
		return 8
	case bp < 1.00:
		return 9
	default:
		return 10
	}
}

type tradeResult struct {
	gain float64
	win  bool
	hit  bool
}

// simulateBodyGroupTrade simulates a single trade.
// Long  → entry on red candle  (Close < Open): TP above, SL below
// Short → entry on green candle (Close > Open): TP below, SL above
func simulateBodyGroupTrade(entry models.Candle, follow []models.Candle, tp, sl float64) tradeResult {
	isLong := entry.Close < entry.Open
	price := entry.Close

	var tpPrice, slPrice float64
	if isLong {
		tpPrice = price + tp
		slPrice = price - sl
	} else {
		tpPrice = price - tp
		slPrice = price + sl
	}

	for _, c := range follow {
		if isLong {
			if c.High >= tpPrice {
				return tradeResult{gain: tp, win: true, hit: true}
			}
			if c.Low <= slPrice {
				return tradeResult{gain: -sl, win: false, hit: true}
			}
		} else {
			if c.Low <= tpPrice {
				return tradeResult{gain: tp, win: true, hit: true}
			}
			if c.High >= slPrice {
				return tradeResult{gain: -sl, win: false, hit: true}
			}
		}
	}
	return tradeResult{hit: false}
}

// scanKey uniquely identifies a (bodyGroup, TP, SL) bucket.
type scanKey struct {
	group int
	tp    int // stored as int cents to avoid float map keys
	sl    int
}

type scanVal struct {
	totalGain float64
	wins      int
	total     int
}

// GenerateBodyGroupInteractiveChart scans all Marubozu candles in highTF (signal)
// and monitors lowTF candles for TP/SL hits.
// If lowTF is nil or empty, the remaining highTF candles are used for monitoring.
// TP ranges 1–100, SL ranges 1–70 (all integer values, dollar-denominated).
func GenerateBodyGroupInteractiveChart(highTF, lowTF []models.Candle, filename string) error {
	useLowTF := len(lowTF) > 0

	stats := make(map[scanKey]*scanVal)

	for i, candle := range highTF {
		if !candle.IsMarubozu() {
			continue
		}

		bp := candle.BodyPercentage()
		group := bodyGroupOf(bp)

		// Determine which candles to watch after entry.
		var follow []models.Candle
		if useLowTF {
			follow = lowTF // caller is responsible for temporal alignment
		} else {
			if i+1 >= len(highTF) {
				continue
			}
			follow = highTF[i+1:]
		}

		for tp := 1; tp <= 100; tp++ {
			for sl := 1; sl <= 70; sl++ {
				res := simulateBodyGroupTrade(candle, follow, float64(tp), float64(sl))
				if !res.hit {
					continue
				}

				k := scanKey{group: group, tp: tp, sl: sl}
				v, ok := stats[k]
				if !ok {
					v = &scanVal{}
					stats[k] = v
				}
				v.totalGain += res.gain
				v.total++
				if res.win {
					v.wins++
				}
			}
		}
	}

	if len(stats) == 0 {
		return fmt.Errorf("هیچ داده‌ای برای نمایش وجود ندارد: کندل Marubozu یافت نشد یا هیچ تریدی بسته نشد")
	}

	// Build data points.
	points := make([]BodyGroupPoint, 0, len(stats))
	for k, v := range stats {
		if v.total == 0 {
			continue
		}
		winRate := float64(v.wins) / float64(v.total) * 100.0
		tp := float64(k.tp)
		sl := float64(k.sl)
		points = append(points, BodyGroupPoint{
			X:         v.totalGain,
			Y:         v.total,
			TP:        tp,
			SL:        sl,
			WinRate:   winRate,
			BodyGroup: k.group,
			BodyLabel: bodyGroupLabel(k.group),
			RR:        tp / sl,
		})
	}

	pointsJSON, err := json.Marshal(points)
	if err != nil {
		return err
	}

	html := buildBodyGroupHTML(string(pointsJSON))

	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		return err
	}

	fmt.Printf("✅ نمودار Body-Group در '%s' ذخیره شد (%d نقطه).\n", filename, len(points))
	return nil
}

// buildBodyGroupHTML returns the full HTML string for the interactive chart.
func buildBodyGroupHTML(pointsJSON string) string {
	// 11 body groups → 11 OKLCH-derived colours (hue steps of ~33°, L=0.62, C=0.14).
	// Using hex approximations that maintain perceptual equal-lightness.
	groupColors := [11]string{
		"#c0627e", // 0:  0–10%   (hue 0)
		"#b8703f", // 1: 10–20%   (hue 33)
		"#9e8224", // 2: 20–30%   (hue 66)
		"#768f24", // 3: 30–40%   (hue 99)
		"#3e993e", // 4: 40–50%   (hue 132)
		"#209980", // 5: 50–60%   (hue 165)
		"#2092a8", // 6: 60–70%   (hue 198)
		"#2a82c8", // 7: 70–80%   (hue 231)
		"#606ac8", // 8: 80–90%   (hue 264)
		"#9056bc", // 9: 90–100%  (hue 297)
		"#b84c96", // 10: 100%+   (hue 330)
	}

	colorArrayJS := "["
	for i, c := range groupColors {
		if i > 0 {
			colorArrayJS += ","
		}
		colorArrayJS += fmt.Sprintf("%q", c)
	}
	colorArrayJS += "]"

	labelArrayJS := "["
	labels := [11]string{
		"0–10%", "10–20%", "20–30%", "30–40%", "40–50%",
		"50–60%", "60–70%", "70–80%", "80–90%", "90–100%", "100%+",
	}
	for i, l := range labels {
		if i > 0 {
			labelArrayJS += ","
		}
		labelArrayJS += fmt.Sprintf("%q", l)
	}
	labelArrayJS += "]"

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Body Group Backtest Chart</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<style>
:root {
  --bg:       #f4f5f7;
  --surface:  #ffffff;
  --surface2: #f0f1f4;
  --border:   #dde0e6;
  --text:     #1a1d24;
  --muted:    #6b7080;
  --accent:   #3b6fd4;
  --radius:   10px;
  --radius-sm:6px;
  color-scheme: light dark;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg:      #141519;
    --surface: #1e2028;
    --surface2:#262830;
    --border:  #32353f;
    --text:    #e8eaf0;
    --muted:   #888c9e;
  }
}
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: 'Segoe UI', system-ui, sans-serif;
  background: var(--bg);
  color: var(--text);
  padding: 24px;
  min-height: 100vh;
}
.page { max-width: 1440px; margin: 0 auto; display: flex; flex-direction: column; gap: 16px; }
h1 { font-size: 1.5rem; font-weight: 700; letter-spacing: -.02em; }
.sub { font-size: .85rem; color: var(--muted); margin-top: 4px; }

/* stats bar */
.stats-bar {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 12px;
}
.stat {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 14px 16px;
  display: flex; flex-direction: column; gap: 4px;
}
.stat-val { font-size: 1.5rem; font-weight: 700; color: var(--accent); font-variant-numeric: tabular-nums; }
.stat-lbl { font-size: .72rem; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }

/* panel */
.panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 20px;
}
.panel-title { font-size: .8rem; text-transform: uppercase; letter-spacing: .07em; color: var(--muted); margin-bottom: 14px; font-weight: 600; }

/* filters */
.filters { display: flex; flex-direction: column; gap: 12px; }
.filter-row { display: flex; align-items: center; gap: 12px; }
.filter-row label { width: 200px; font-size: .875rem; font-weight: 500; flex-shrink: 0; }
.filter-row input[type=range] {
  flex: 1; height: 6px; border-radius: 3px;
  accent-color: var(--accent); cursor: pointer;
}
.filter-val { width: 56px; text-align: right; font-size: .9rem; font-weight: 700; font-variant-numeric: tabular-nums; color: var(--accent); }
.filter-info { font-size: .82rem; color: var(--muted); text-align: center; padding-top: 8px; border-top: 1px solid var(--border); }
.filter-info strong { color: var(--accent); font-size: 1rem; }

/* group toggles */
.group-toggles { display: flex; flex-wrap: wrap; gap: 8px; }
.g-toggle {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 10px; border-radius: 20px; font-size: .78rem; font-weight: 600;
  border: 2px solid transparent; cursor: pointer;
  transition: opacity .15s, transform .1s;
  user-select: none;
}
.g-toggle.off { opacity: .35; }
.g-toggle:hover { transform: scale(1.04); }
.g-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }

/* chart */
.chart-wrap { position: relative; height: 580px; }

/* legend */
.legend-grid { display: flex; flex-wrap: wrap; gap: 10px; }
.legend-item { display: flex; align-items: center; gap: 6px; font-size: .78rem; }
.legend-dot { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }

@media (prefers-reduced-motion: reduce) { * { transition: none !important; } }
</style>
</head>
<body>
<div class="page">

  <div>
    <h1>📊 Body Group Backtest Matrix</h1>
    <p class="sub">All Marubozu candles · TP 1–100 · SL 1–70 · Hover a point for full details</p>
  </div>

  <div class="stats-bar">
    <div class="stat"><span class="stat-val" id="sTotalPts">–</span><span class="stat-lbl">Combinations</span></div>
    <div class="stat"><span class="stat-val" id="sBestGain">–</span><span class="stat-lbl">Best Gain ($)</span></div>
    <div class="stat"><span class="stat-val" id="sBestCombo">–</span><span class="stat-lbl">Best TP / SL</span></div>
    <div class="stat"><span class="stat-val" id="sBestGroup">–</span><span class="stat-lbl">Best Body Group</span></div>
    <div class="stat"><span class="stat-val" id="sBestWR">–</span><span class="stat-lbl">Best Win Rate</span></div>
    <div class="stat"><span class="stat-val" id="sBestRR">–</span><span class="stat-lbl">Best R/R</span></div>
  </div>

  <div class="panel">
    <p class="panel-title">Body Groups</p>
    <div class="group-toggles" id="groupToggles"></div>
  </div>

  <div class="panel">
    <p class="panel-title">Filters</p>
    <div class="filters">
      <div class="filter-row">
        <label>🎯 Min Win Rate</label>
        <input type="range" id="slWR" min="0" max="100" value="0" step="5">
        <span class="filter-val" id="vWR">0%%</span>
      </div>
      <div class="filter-row">
        <label>🔢 Min Trades</label>
        <input type="range" id="slTrades" min="0" max="200" value="0" step="1">
        <span class="filter-val" id="vTrades">0</span>
      </div>
      <div class="filter-row">
        <label>📊 Min R/R Ratio</label>
        <input type="range" id="slRR" min="0.1" max="20" value="0.1" step="0.1">
        <span class="filter-val" id="vRR">0.1</span>
      </div>
      <div class="filter-info">
        Showing <strong id="visCount">0</strong> of <strong id="totCount">0</strong> combinations
      </div>
    </div>
  </div>

  <div class="panel">
    <div class="chart-wrap"><canvas id="chart"></canvas></div>
  </div>

  <div class="panel">
    <p class="panel-title">Legend</p>
    <div class="legend-grid" id="legendGrid"></div>
  </div>

</div>

<script>
const GROUP_COLORS = %s;
const GROUP_LABELS = %s;
const NUM_GROUPS   = 11;

const rawData = %s;

// Pre-compute rr, normalise types
const allData = rawData.map(p => ({
  x:         +p.x,
  y:         +p.y,
  tp:        +p.tp,
  sl:        +p.sl,
  winRate:   +p.winRate,
  bodyGroup: +p.bodyGroup,
  bodyLabel: p.bodyLabel,
  rr:        +p.tp / +p.sl
}));

// --- Group toggles ---
const activeGroups = new Set(Array.from({length: NUM_GROUPS}, (_, i) => i));
const togglesEl = document.getElementById('groupToggles');
for (let g = 0; g < NUM_GROUPS; g++) {
  const el = document.createElement('div');
  el.className = 'g-toggle';
  el.dataset.group = g;
  el.style.background = GROUP_COLORS[g] + '22';
  el.style.borderColor = GROUP_COLORS[g];
  el.innerHTML = '<span class="g-dot" style="background:' + GROUP_COLORS[g] + '"></span>' + GROUP_LABELS[g];
  el.addEventListener('click', () => {
    if (activeGroups.has(g)) { activeGroups.delete(g); el.classList.add('off'); }
    else { activeGroups.add(g); el.classList.remove('off'); }
    render();
  });
  togglesEl.appendChild(el);
}

// --- Legend ---
const legendEl = document.getElementById('legendGrid');
for (let g = 0; g < NUM_GROUPS; g++) {
  const item = document.createElement('div');
  item.className = 'legend-item';
  item.innerHTML = '<span class="legend-dot" style="background:' + GROUP_COLORS[g] + '"></span>' + GROUP_LABELS[g];
  legendEl.appendChild(item);
}

// --- Chart ---
let chart = null;

function render() {
  const minWR     = +document.getElementById('slWR').value;
  const minTrades = +document.getElementById('slTrades').value;
  const minRR     = +document.getElementById('slRR').value;

  document.getElementById('vWR').textContent     = minWR + '%%';
  document.getElementById('vTrades').textContent = minTrades;
  document.getElementById('vRR').textContent     = minRR.toFixed(1);

  const filtered = allData.filter(p =>
    activeGroups.has(p.bodyGroup) &&
    p.winRate >= minWR &&
    p.y >= minTrades &&
    p.rr >= minRR
  );

  document.getElementById('visCount').textContent = filtered.length;
  document.getElementById('totCount').textContent = allData.length;

  // Stats
  let bestGain = -Infinity, bestPt = null, bestWR = 0, bestRR = 0;
  filtered.forEach(p => {
    if (p.x > bestGain)       { bestGain = p.x; bestPt = p; }
    if (p.winRate > bestWR)    bestWR = p.winRate;
    if (p.rr > bestRR)         bestRR = p.rr;
  });
  document.getElementById('sTotalPts').textContent  = filtered.length;
  document.getElementById('sBestGain').textContent  = bestGain === -Infinity ? '–' : '$' + bestGain.toFixed(2);
  document.getElementById('sBestCombo').textContent = bestPt ? bestPt.tp + ' / ' + bestPt.sl : '–';
  document.getElementById('sBestGroup').textContent = bestPt ? bestPt.bodyLabel : '–';
  document.getElementById('sBestWR').textContent    = bestWR > 0 ? bestWR.toFixed(1) + '%%' : '–';
  document.getElementById('sBestRR').textContent    = bestRR > 0 ? bestRR.toFixed(2) : '–';

  // Split into per-group datasets (Chart.js colours each dataset differently)
  const byGroup = {};
  filtered.forEach(p => {
    if (!byGroup[p.bodyGroup]) byGroup[p.bodyGroup] = [];
    byGroup[p.bodyGroup].push(p);
  });

  const datasets = Object.entries(byGroup).map(([g, pts]) => {
    const gi = +g;
    const col = GROUP_COLORS[gi];
    return {
      label: GROUP_LABELS[gi],
      data: pts,
      backgroundColor: col + 'bb',
      borderColor: col,
      borderWidth: 1,
      pointRadius: 7,
      pointHoverRadius: 11,
      pointHoverBorderWidth: 2,
      pointHoverBorderColor: '#000'
    };
  });

  if (chart) chart.destroy();

  const ctx = document.getElementById('chart').getContext('2d');
  chart = new Chart(ctx, {
    type: 'scatter',
    data: { datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 200 },
      interaction: { mode: 'nearest', intersect: true },
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: 'rgba(15,16,20,.92)',
          titleFont: { size: 13, weight: '700' },
          bodyFont: { size: 12 },
          padding: 14,
          cornerRadius: 8,
          displayColors: false,
          callbacks: {
            title: ctx => '📦 ' + ctx[0].raw.bodyLabel,
            label: ctx => {
              const p = ctx.raw;
              return [
                '  TP: $' + p.tp + '   SL: $' + p.sl,
                '  Net Gain : $' + p.x.toFixed(2),
                '  Trades   : ' + p.y,
                '  Win Rate : ' + p.winRate.toFixed(1) + '%%',
                '  R/R      : ' + p.rr.toFixed(2)
              ];
            },
            afterLabel: ctx => {
              const p = ctx.raw;
              if (p.x > 0 && p.winRate >= 70) return '  🏆 Excellent';
              if (p.x > 0 && p.winRate >= 50) return '  ✅ Good';
              if (p.x > 0)                    return '  ⚠️ Marginal';
              return '  ❌ Loss';
            }
          }
        }
      },
      scales: {
        x: {
          type: 'linear',
          title: { display: true, text: 'Net Gain ($)', font: { size: 13, weight: '600' } },
          grid: { color: 'rgba(128,128,128,.1)' }
        },
        y: {
          type: 'linear',
          title: { display: true, text: 'Number of Trades', font: { size: 13, weight: '600' } },
          grid: { color: 'rgba(128,128,128,.1)' }
        }
      }
    }
  });
}

document.getElementById('slWR').addEventListener('input', render);
document.getElementById('slTrades').addEventListener('input', render);
document.getElementById('slRR').addEventListener('input', render);

render();
</script>
</body>
</html>`, colorArrayJS, labelArrayJS, pointsJSON)
}
