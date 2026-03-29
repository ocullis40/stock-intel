package agent

import (
	"fmt"
	"sync"
	"time"

	"github.com/oliver/stock-intel/internal/agent/steps"
	"github.com/oliver/stock-intel/internal/types"
	"github.com/oliver/stock-intel/internal/usage"
)

// ProgressFunc is called when an agent step starts.
type ProgressFunc func(update types.ProgressUpdate)

// AnalyzeTicker runs the full agent pipeline for one ticker.
func AnalyzeTicker(ticker string, cfg types.Config, onProgress ProgressFunc, tracker *usage.Tracker) types.TickerIntel {
	var log []types.AgentStep
	totalSteps := 3

	report := func(step string, idx int) {
		if onProgress != nil {
			onProgress(types.ProgressUpdate{
				Ticker:     ticker,
				Step:       step,
				StepIndex:  idx,
				TotalSteps: totalSteps,
			})
		}
	}

	// Step 1: Research (parallel API calls for technicals + news)
	report("Researching technicals & news", 1)
	technicals, news, sources, step1 := steps.FetchAll(ticker, cfg.Model, tracker)
	log = append(log, step1)

	// Step 2: Validate (local, no API call)
	report("Validating data", 2)
	validation, step2 := steps.Validate(ticker, technicals)
	log = append(log, step2)

	// Step 3: Synthesize (local, no API call)
	report("Synthesizing analysis", 3)
	maSignal, step3 := steps.Synthesize(ticker, technicals)
	log = append(log, step3)

	newsData := types.NewsData{
		Headline:           "Unable to fetch news",
		Bullets:            []string{},
		Risk:               "Unknown",
		Catalyst:           "Unknown",
		SentimentShortTerm: "neutral",
		SentimentMedTerm:   "neutral",
	}
	if news != nil {
		newsData = *news
	}

	return types.TickerIntel{
		Ticker:     ticker,
		Name:       "",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Technicals: technicals,
		News:       newsData,
		MASignal:   maSignal,
		Validation: validation,
		AgentLog:   log,
		Sources:    sources,
	}
}

// AnalyzeAll runs the agent pipeline for all tickers with controlled concurrency.
// Returns results and a usage summary for the run.
func AnalyzeAll(cfg types.Config, onProgress ProgressFunc) (map[string]types.TickerIntel, types.UsageSummary) {
	tracker := usage.New(cfg.MaxTokensPerRun, cfg.RequestDelayMs)

	tickers := cfg.Tickers
	if cfg.MaxTickers > 0 && len(tickers) > cfg.MaxTickers {
		if onProgress != nil {
			onProgress(types.ProgressUpdate{
				Ticker: "SYSTEM",
				Step:   fmt.Sprintf("Capping tickers from %d to %d (maxTickers limit)", len(tickers), cfg.MaxTickers),
			})
		}
		tickers = tickers[:cfg.MaxTickers]
	}

	results := make(map[string]types.TickerIntel)
	var mu sync.Mutex

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, ticker := range tickers {
		wg.Add(1)
		sem <- struct{}{}

		go func(t string) {
			defer wg.Done()
			defer func() { <-sem }()

			intel := AnalyzeTicker(t, cfg, onProgress, tracker)

			mu.Lock()
			results[t] = intel
			mu.Unlock()
		}(ticker)
	}

	wg.Wait()
	return results, tracker.Summary()
}
