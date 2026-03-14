package steps

import (
	"fmt"
	"time"

	"github.com/oliver/stock-intel/internal/agent"
)

// FetchNews searches for recent news, sentiment, risk, and catalysts.
func FetchNews(ticker, model string) (*agent.NewsData, agent.AgentStep) {
	start := time.Now()

	prompt := fmt.Sprintf(`Research recent news and analyst sentiment for %s. Search for "%s stock news this week" and "%s analyst outlook".

Focus on:
- The single most important recent development
- Key insights about performance, news, or sector trends
- The primary risk or concern right now
- Any upcoming catalyst or event to watch
- Short-term sentiment (next days/weeks) vs medium-term sentiment (next months)

Return ONLY valid JSON (no markdown, no explanation):
{
  "headline": "Most important recent development in 1 clear sentence",
  "bullets": ["Key insight 1", "Key insight 2", "Key insight 3"],
  "risk": "Primary risk or concern right now",
  "catalyst": "Upcoming catalyst or event to watch",
  "sentimentShortTerm": "bullish|bearish|neutral|mixed",
  "sentimentMedTerm": "bullish|bearish|neutral|mixed"
}

Return ONLY the JSON.`, ticker, ticker, ticker)

	raw, err := agent.SearchAndExtract(prompt, model)
	if err != nil {
		return nil, agent.AgentStep{
			Step:       "news",
			Action:     fmt.Sprintf("Searched for %s news and sentiment", ticker),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: time.Since(start).Milliseconds(),
			Result:     "failed",
			Detail:     fmt.Sprintf("API error: %v", err),
		}
	}

	parsed, err := agent.ParseJSON[agent.NewsData](raw)
	if err != nil {
		return nil, agent.AgentStep{
			Step:       "news",
			Action:     fmt.Sprintf("Searched for %s news and sentiment", ticker),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: time.Since(start).Milliseconds(),
			Result:     "failed",
			Detail:     fmt.Sprintf("Parse error: %v", err),
		}
	}

	headline := parsed.Headline
	if len(headline) > 80 {
		headline = headline[:80] + "..."
	}

	return parsed, agent.AgentStep{
		Step:       "news",
		Action:     fmt.Sprintf("Searched for %s news and sentiment", ticker),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMs: time.Since(start).Milliseconds(),
		Result:     "success",
		Detail:     fmt.Sprintf("Headline: %s", headline),
	}
}
