package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oliver/stock-intel/internal/agent"
	"github.com/oliver/stock-intel/internal/config"
)

func main() {
	root := findRoot()
	config.Init(filepath.Join(root, "config.json"))

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nStock Intel — Analyzing %d tickers\n\n", len(cfg.Tickers))
	fmt.Printf("  Tickers:     %s\n", strings.Join(cfg.Tickers, ", "))
	fmt.Printf("  Model:       %s\n", cfg.Model)
	fmt.Printf("  Concurrency: %d\n\n", cfg.Concurrency)

	results := agent.AnalyzeAll(cfg, func(update agent.ProgressUpdate) {
		fmt.Printf("  [%s] Step %d/%d: %s\n", update.Ticker, update.StepIndex, update.TotalSteps, update.Step)
	})

	fmt.Printf("\n%s\n\n", strings.Repeat("─", 60))

	for _, ticker := range cfg.Tickers {
		intel, ok := results[ticker]
		if !ok {
			continue
		}

		t := intel.Technicals
		n := intel.News
		confidence := strings.ToUpper(intel.Validation.Confidence)

		fmt.Printf("%s [%s confidence]\n", ticker, confidence)

		// Price
		priceStr := "—"
		if t.Price != nil {
			priceStr = fmt.Sprintf("$%.2f", *t.Price)
		}
		changeStr := "—"
		if t.Change != nil && t.ChangePct != nil {
			sign := "+"
			if *t.Change < 0 {
				sign = ""
			}
			changeStr = fmt.Sprintf("%s%.2f (%s%.2f%%)", sign, *t.Change, sign, *t.ChangePct)
		}
		fmt.Printf("  Price: %s  Change: %s\n", priceStr, changeStr)

		// Technicals
		rsiStr := "—"
		if t.RSI != nil {
			rsiStr = fmt.Sprintf("%.1f", *t.RSI)
		}
		ma50Str := "—"
		if t.MA50 != nil {
			ma50Str = fmt.Sprintf("$%.2f", *t.MA50)
		}
		ma200Str := "—"
		if t.MA200 != nil {
			ma200Str = fmt.Sprintf("$%.2f", *t.MA200)
		}
		fmt.Printf("  RSI(14): %s  50MA: %s  200MA: %s\n", rsiStr, ma50Str, ma200Str)
		fmt.Printf("  Signal: %s\n", intel.MASignal)

		// Sentiment
		fmt.Printf("  Near-term: %s  |  Med-term: %s\n", n.SentimentShortTerm, n.SentimentMedTerm)

		// News
		fmt.Printf("  Headline: %s\n", n.Headline)
		for _, b := range n.Bullets {
			fmt.Printf("    • %s\n", b)
		}
		fmt.Printf("  Risk: %s\n", n.Risk)
		fmt.Printf("  Catalyst: %s\n", n.Catalyst)

		// Agent stats
		successCount := 0
		failedCount := 0
		for _, s := range intel.AgentLog {
			if s.Result == "success" {
				successCount++
			}
			if s.Result == "failed" {
				failedCount++
			}
		}
		fmt.Printf("  Agent steps: %d (%d success, %d failed)\n\n", len(intel.AgentLog), successCount, failedCount)
	}
}

func findRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			cwd, _ := os.Getwd()
			return cwd
		}
		dir = parent
	}
}
