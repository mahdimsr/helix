package walking

import "fmt"

func PrintLiveOptimizationResults(groups WindowGroups) {
	fmt.Println("\n┌──────────────┬──────────────┬────────────┬────────────┬───────────┐")
	fmt.Println("│    group     │    range     │     TP     │     SL     │   Score   │")
	fmt.Println("├──────────────┼──────────────┼────────────┼────────────┼───────────┤")

	for _, g := range groups.Groups {
		if g.BestTP > 0 && g.BestSL > 0 {
			fmt.Printf("│      %d       │ %-12s │ $%7.0f  │ $%7.0f  │  %6.3f  │\n",
				g.Index,
				fmt.Sprintf("%.1f%%-%.1f%%", g.MinPercent, g.MaxPercent),
				g.BestTP,
				g.BestSL,
				g.BestScore)
		} else {
			fmt.Printf("│      %d       │ %-12s │    ---     │    ---     │    ---    │\n",
				g.Index,
				fmt.Sprintf("%.1f%%-%.1f%%", g.MinPercent, g.MaxPercent))
		}
	}
	fmt.Println("└──────────────┴──────────────┴────────────┴────────────┴───────────┘")
}
