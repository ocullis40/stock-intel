package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oliver/stock-intel/internal/agent"
	"github.com/oliver/stock-intel/internal/config"
	"github.com/oliver/stock-intel/internal/types"
)

func main() {
	root := findRoot()
	loadEnv(filepath.Join(root, ".env"))
	config.Init(filepath.Join(root, "config.json"))

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nStock Intel — Analyzing %d tickers\n\n", len(cfg.Tickers))
	fmt.Printf("  Tickers:     %s\n", strings.Join(cfg.Tickers, ", "))
	fmt.Printf("  Model:       %s\n", cfg.Model)
	fmt.Printf("  Concurrency: %d\n", cfg.Concurrency)
	budgetStr := "unlimited"
	if cfg.MaxTokensPerRun > 0 {
		budgetStr = fmt.Sprintf("%d tokens", cfg.MaxTokensPerRun)
	}
	fmt.Printf("  Token budget: %s\n", budgetStr)
	fmt.Printf("  Max tickers: %d\n", cfg.MaxTickers)
	if cfg.RequestDelayMs > 0 {
		fmt.Printf("  Request delay: %dms\n", cfg.RequestDelayMs)
	}
	fmt.Println()

	results, usageSummary := agent.AnalyzeAll(cfg, func(update types.ProgressUpdate) {
		if update.TotalSteps > 0 {
			fmt.Printf("  [%s] Step %d/%d: %s\n", update.Ticker, update.StepIndex, update.TotalSteps, update.Step)
		} else {
			fmt.Printf("  [%s] %s\n", update.Ticker, update.Step)
		}
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
		fmt.Printf("  Near-term: %s  |  Med-term: %s\n", n.SentimentShortTerm, n.SentimentMedTerm)
		fmt.Printf("  Headline: %s\n", n.Headline)
		for _, b := range n.Bullets {
			fmt.Printf("    • %s\n", b)
		}
		fmt.Printf("  Risk: %s\n", n.Risk)
		fmt.Printf("  Catalyst: %s\n", n.Catalyst)

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

	fmt.Printf("%s\n", strings.Repeat("─", 60))
	fmt.Printf("  Usage Summary\n")
	fmt.Printf("  API calls:      %d\n", usageSummary.APICalls)
	fmt.Printf("  Input tokens:   %d\n", usageSummary.TotalInputTokens)
	fmt.Printf("  Output tokens:  %d\n", usageSummary.TotalOutputTokens)
	fmt.Printf("  Total tokens:   %d\n", usageSummary.TotalTokens)
	fmt.Printf("  Est. cost:      $%.4f\n", usageSummary.EstimatedCost)
	if usageSummary.BudgetUsedPct > 0 {
		fmt.Printf("  Budget used:    %.1f%%\n", usageSummary.BudgetUsedPct)
	}
	fmt.Println()
}

func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if _, exists := os.LookupEnv(key); !exists && val != "" {
			os.Setenv(key, val)
		}
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
