package steps

import (
	"fmt"
	"time"

	"github.com/oliver/stock-intel/internal/client"
	"github.com/oliver/stock-intel/internal/types"
	"github.com/oliver/stock-intel/internal/usage"
)

type combinedResult struct {
	Price              *float64 `json:"price"`
	Change             *float64 `json:"change"`
	ChangePct          *float64 `json:"changePct"`
	RSI                *float64 `json:"rsi"`
	MA50               *float64 `json:"ma50"`
	MA200              *float64 `json:"ma200"`
	Source             string   `json:"source"`
	Headline           string   `json:"headline"`
	Bullets            []string `json:"bullets"`
	Risk               string   `json:"risk"`
	Catalyst           string   `json:"catalyst"`
	SentimentShortTerm string   `json:"sentimentShortTerm"`
	SentimentMedTerm   string   `json:"sentimentMedTerm"`
}

// FetchAll performs a single API call to get both technicals and news for a ticker.
func FetchAll(ticker, model string, tracker *usage.Tracker) (types.TechnicalData, *types.NewsData, types.AgentStep) {
	start := time.Now()
	emptyTech := types.TechnicalData{Source: "error"}

	if err := tracker.PreCallCheck(); err != nil {
		return emptyTech, nil, types.AgentStep{
			Step: "research", Action: fmt.Sprintf("Researched %s technicals and news", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: err.Error(),
		}
	}

	prompt := fmt.Sprintf(`Search for current technical indicators AND recent news for stock/ETF "%s".

TECHNICALS — search for "%s technical analysis RSI moving average" and "%s stock price today":
1. Current price and today's change/change%%
2. RSI (14-day relative strength index) — try Barchart, TradingView, or StockAnalysis
3. 50-day simple moving average (SMA)
4. 200-day simple moving average (SMA)

NEWS — search for "%s stock news this week" and "%s analyst outlook":
1. The single most important recent development
2. Key insights about performance, news, or sector trends (2-3 bullets)
3. The primary risk or concern right now
4. Any upcoming catalyst or event to watch
5. Short-term sentiment (next days/weeks) vs medium-term sentiment (next months)

Return ONLY valid JSON (no markdown, no explanation):
{
  "price": 123.45,
  "change": -1.23,
  "changePct": -0.98,
  "rsi": 65.4,
  "ma50": 110.25,
  "ma200": 98.50,
  "source": "where you found the technicals",
  "headline": "Most important recent development in 1 clear sentence",
  "bullets": ["Key insight 1", "Key insight 2", "Key insight 3"],
  "risk": "Primary risk or concern right now",
  "catalyst": "Upcoming catalyst or event to watch",
  "sentimentShortTerm": "bullish|bearish|neutral|mixed",
  "sentimentMedTerm": "bullish|bearish|neutral|mixed"
}

Use null for any technical value you genuinely cannot find. Return ONLY the JSON.`, ticker, ticker, ticker, ticker, ticker)

	raw, u, err := client.SearchAndExtract(prompt, model)
	tracker.RecordUsage(u)
	if err != nil {
		return emptyTech, nil, types.AgentStep{
			Step: "research", Action: fmt.Sprintf("Researched %s technicals and news", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: fmt.Sprintf("API error: %v", err),
		}
	}

	parsed, err := client.ParseJSON[combinedResult](raw)
	if err != nil {
		return emptyTech, nil, types.AgentStep{
			Step: "research", Action: fmt.Sprintf("Researched %s technicals and news", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: fmt.Sprintf("Parse error: %v", err),
		}
	}

	tech := types.TechnicalData{
		Price: parsed.Price, Change: parsed.Change, ChangePct: parsed.ChangePct,
		RSI: parsed.RSI, MA50: parsed.MA50, MA200: parsed.MA200, Source: parsed.Source,
	}

	news := &types.NewsData{
		Headline:           parsed.Headline,
		Bullets:            parsed.Bullets,
		Risk:               parsed.Risk,
		Catalyst:           parsed.Catalyst,
		SentimentShortTerm: parsed.SentimentShortTerm,
		SentimentMedTerm:   parsed.SentimentMedTerm,
	}
	if news.Bullets == nil {
		news.Bullets = []string{}
	}

	nullCount := countNils(parsed.Price, parsed.RSI, parsed.MA50, parsed.MA200)
	result := "success"
	detail := fmt.Sprintf("Found technicals and news from %s", parsed.Source)
	if nullCount > 0 {
		result = "partial"
		detail = fmt.Sprintf("Missing %d technical value(s), source: %s", nullCount, parsed.Source)
	}

	return tech, news, types.AgentStep{
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
