package agent

import (
	"sync"
	"time"

	"github.com/oliver/stock-intel/internal/agent/steps"
	"github.com/oliver/stock-intel/internal/types"
)

// ProgressFunc is called when an agent step starts.
type ProgressFunc func(update types.ProgressUpdate)

// AnalyzeTicker runs the full agent pipeline for one ticker.
func AnalyzeTicker(ticker string, cfg types.Config, onProgress ProgressFunc) types.TickerIntel {
	var log []types.AgentStep
	totalSteps := 5

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

	// Step 1: Technicals
	report("Searching for technicals", 1)
	technicals, step1 := steps.FetchTechnicals(ticker, cfg.Model)
	log = append(log, step1)

	// Step 2: Validate
	report("Validating data", 2)
	validation, step2 := steps.Validate(ticker, technicals)
	log = append(log, step2)

	// Step 3: Fill gaps (with retries)
	if cfg.Agent.ValidateTechnicals && len(validation.Missing) > 0 {
		for attempt := 0; attempt < cfg.Agent.MaxRetries; attempt++ {
			report("Filling gaps", 3)
			var step3 types.AgentStep
			technicals, step3 = steps.FillGaps(ticker, technicals, validation, cfg.Model)
			log = append(log, step3)

			validation, step2 = steps.Validate(ticker, technicals)
			log = append(log, step2)

			if len(validation.Missing) == 0 {
				break
			}
		}
	} else {
		log = append(log, types.AgentStep{
			Step:       "fill_gaps",
			Action:     "Skipped — no gaps or validation disabled",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: 0,
			Result:     "success",
			Detail:     "No action needed",
		})
	}

	// Step 4: News
	report("Researching news & sentiment", 4)
	news, step4 := steps.FetchNews(ticker, cfg.Model)
	log = append(log, step4)

	// Step 5: Synthesize
	report("Synthesizing analysis", 5)
	maSignal, step5 := steps.Synthesize(ticker, technicals)
	log = append(log, step5)

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
	}
}

// AnalyzeAll runs the agent pipeline for all tickers with controlled concurrency.
func AnalyzeAll(cfg types.Config, onProgress ProgressFunc) map[string]types.TickerIntel {
	results := make(map[string]types.TickerIntel)
	var mu sync.Mutex

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, ticker := range cfg.Tickers {
		wg.Add(1)
		sem <- struct{}{}

		go func(t string) {
			defer wg.Done()
			defer func() { <-sem }()

			intel := AnalyzeTicker(t, cfg, onProgress)

			mu.Lock()
			results[t] = intel
			mu.Unlock()
		}(ticker)
	}

	wg.Wait()
	return results
}
