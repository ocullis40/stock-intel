package steps

import (
	"fmt"
	"sync"
	"time"

	"github.com/oliver/stock-intel/internal/client"
	"github.com/oliver/stock-intel/internal/types"
	"github.com/oliver/stock-intel/internal/usage"
)

type techResult struct {
	Price     *float64 `json:"price"`
	Change    *float64 `json:"change"`
	ChangePct *float64 `json:"changePct"`
	RSI       *float64 `json:"rsi"`
	MA50      *float64 `json:"ma50"`
	MA200     *float64 `json:"ma200"`
	Source    string   `json:"source"`
}

type newsResult struct {
	Headline           string   `json:"headline"`
	Bullets            []string `json:"bullets"`
	Risk               string   `json:"risk"`
	Catalyst           string   `json:"catalyst"`
	SentimentShortTerm string   `json:"sentimentShortTerm"`
	SentimentMedTerm   string   `json:"sentimentMedTerm"`
	BreakingNews       string   `json:"breakingNews"`
}

// FetchAll performs two parallel API calls: one for technicals, one for news.
func FetchAll(ticker, model string, tracker *usage.Tracker) (types.TechnicalData, *types.NewsData, []types.Source, types.AgentStep) {
	start := time.Now()
	emptyTech := types.TechnicalData{Source: "error"}

	if err := tracker.PreCallCheck(); err != nil {
		return emptyTech, nil, nil, types.AgentStep{
			Step: "research", Action: fmt.Sprintf("Researched %s technicals and news", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: err.Error(),
		}
	}

	var tech types.TechnicalData
	var news *types.NewsData
	var techErr, newsErr error
	var techUsage, newsUsage types.Usage
	var techSources, newsSources []types.Source

	var wg sync.WaitGroup
	wg.Add(2)

	// Call 1: Technicals
	go func() {
		defer wg.Done()
		techPrompt := fmt.Sprintf(`Search for "%s stock price RSI 50-day moving average 200-day moving average technical indicators".

Find these values for %s:
- Current stock price
- Today's dollar change and percent change
- 14-day RSI
- 50-day simple moving average (SMA)
- 200-day simple moving average (SMA)

These are standard indicators on TradingView, StockAnalysis, MarketWatch, Barchart, etc. If multiple sources show slightly different values, use the most recent one. NEVER return null if a value is available — approximate values from credible sources are acceptable.

Respond with ONLY this JSON object, no other text:
{"price":null,"change":null,"changePct":null,"rsi":null,"ma50":null,"ma200":null,"source":""}

All values must be numbers or null (only use null if truly unavailable). source = site(s) where you found the data.`, ticker, ticker)

		raw, srcs, u, err := client.SearchAndExtract(techPrompt, model)
		techUsage = u
		techSources = srcs
		if err != nil {
			techErr = err
			return
		}
		parsed, err := client.ParseJSON[techResult](raw)
		if err != nil {
			techErr = err
			return
		}
		tech = types.TechnicalData{
			Price: parsed.Price, Change: parsed.Change, ChangePct: parsed.ChangePct,
			RSI: parsed.RSI, MA50: parsed.MA50, MA200: parsed.MA200, Source: parsed.Source,
		}
	}()

	// Call 2: News + sector
	go func() {
		defer wg.Done()
		newsPrompt := fmt.Sprintf(`Search for all of the following:
- "%s stock news this week"
- "%s sector industry news this week"

Find company-specific news AND broader sector/industry developments that could impact this stock. Include competitor announcements, regulatory changes, and technology shifts.

Respond with ONLY this JSON object, no other text:
{"breakingNews":null,"headline":"","bullets":[],"risk":"","catalyst":"","sentimentShortTerm":"neutral","sentimentMedTerm":"neutral"}

Fields: breakingNews=most impactful news from the past week (company OR industry) or null, headline=most important recent development, bullets=3-5 key insights (MUST include relevant sector/industry news), risk=primary risk including sector-wide risks, catalyst=upcoming catalyst, sentimentShortTerm/sentimentMedTerm=bullish|bearish|neutral|mixed.`, ticker, ticker)

		raw, srcs, u, err := client.SearchAndExtract(newsPrompt, model)
		newsUsage = u
		newsSources = srcs
		if err != nil {
			newsErr = err
			return
		}
		parsed, err := client.ParseJSON[newsResult](raw)
		if err != nil {
			newsErr = err
			return
		}
		if parsed.Bullets == nil {
			parsed.Bullets = []string{}
		}
		news = &types.NewsData{
			BreakingNews:       parsed.BreakingNews,
			Headline:           parsed.Headline,
			Bullets:            parsed.Bullets,
			Risk:               parsed.Risk,
			Catalyst:           parsed.Catalyst,
			SentimentShortTerm: parsed.SentimentShortTerm,
			SentimentMedTerm:   parsed.SentimentMedTerm,
		}
	}()

	wg.Wait()

	// Merge usage from both calls
	tracker.RecordUsage(types.Usage{
		InputTokens:  techUsage.InputTokens + newsUsage.InputTokens,
		OutputTokens: techUsage.OutputTokens + newsUsage.OutputTokens,
	})

	// Build result
	details := []string{}
	result := "success"

	if techErr != nil {
		details = append(details, fmt.Sprintf("technicals failed: %v", techErr))
		result = "failed"
		tech = emptyTech
	}
	if newsErr != nil {
		details = append(details, fmt.Sprintf("news failed: %v", newsErr))
		if result != "failed" {
			result = "partial"
		}
	}

	if result == "success" {
		nullCount := countNils(tech.Price, tech.RSI, tech.MA50, tech.MA200)
		if nullCount > 0 {
			result = "partial"
			details = append(details, fmt.Sprintf("missing %d technical value(s)", nullCount))
		}
		details = append(details, fmt.Sprintf("source: %s", tech.Source))
	}

	// Deduplicate and merge sources
	seen := map[string]bool{}
	var allSources []types.Source
	for _, s := range append(techSources, newsSources...) {
		if !seen[s.URL] {
			seen[s.URL] = true
			allSources = append(allSources, s)
		}
	}

	detail := "OK"
	if len(details) > 0 {
		detail = joinDetails(details)
	}

	return tech, news, allSources, types.AgentStep{
		Step: "research", Action: fmt.Sprintf("Researched %s technicals and news", ticker),
		Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
		Result: result, Detail: detail,
	}
}

func countNils(ptrs ...*float64) int {
	n := 0
	for _, p := range ptrs {
		if p == nil {
			n++
		}
	}
	return n
}

func joinDetails(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += "; " + p
	}
	return out
}
