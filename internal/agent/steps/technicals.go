package steps

import (
	"fmt"
	"time"

	"github.com/oliver/stock-intel/internal/client"
	"github.com/oliver/stock-intel/internal/types"
)

type technicalsRaw struct {
	Price     *float64 `json:"price"`
	Change    *float64 `json:"change"`
	ChangePct *float64 `json:"changePct"`
	RSI       *float64 `json:"rsi"`
	MA50      *float64 `json:"ma50"`
	MA200     *float64 `json:"ma200"`
	Source    string   `json:"source"`
}

// FetchTechnicals searches for current price and technical indicators.
func FetchTechnicals(ticker, model string) (types.TechnicalData, types.AgentStep) {
	start := time.Now()
	empty := types.TechnicalData{Source: "error"}

	prompt := fmt.Sprintf(`Search for current technical indicators for stock/ETF "%s".
I need these SPECIFIC values:
1. Current price and today's change/change%%
2. RSI (14-day relative strength index) — look on Barchart, TradingView, or StockAnalysis
3. 50-day simple moving average (SMA)
4. 200-day simple moving average (SMA)

Search for "%s technical analysis RSI moving average" and "%s stock price today".

Return ONLY valid JSON (no markdown, no explanation):
{"price":123.45,"change":-1.23,"changePct":-0.98,"rsi":65.4,"ma50":110.25,"ma200":98.50,"source":"where you found the technicals"}

Use null for any value you genuinely cannot find. Return ONLY the JSON.`, ticker, ticker, ticker)

	raw, err := client.SearchAndExtract(prompt, model)
	if err != nil {
		return empty, types.AgentStep{
			Step: "technicals", Action: fmt.Sprintf("Searched for %s technicals", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: fmt.Sprintf("API error: %v", err),
		}
	}

	parsed, err := client.ParseJSON[technicalsRaw](raw)
	if err != nil {
		return empty, types.AgentStep{
			Step: "technicals", Action: fmt.Sprintf("Searched for %s technicals", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: "failed", Detail: fmt.Sprintf("Parse error: %v", err),
		}
	}

	data := types.TechnicalData{
		Price: parsed.Price, Change: parsed.Change, ChangePct: parsed.ChangePct,
		RSI: parsed.RSI, MA50: parsed.MA50, MA200: parsed.MA200, Source: parsed.Source,
	}

	nullCount := countNils(parsed.Price, parsed.RSI, parsed.MA50, parsed.MA200)
	result := "success"
	detail := fmt.Sprintf("Found all technicals from %s", parsed.Source)
	if nullCount > 0 {
		result = "partial"
		detail = fmt.Sprintf("Missing %d value(s), source: %s", nullCount, parsed.Source)
	}

	return data, types.AgentStep{
		Step: "technicals", Action: fmt.Sprintf("Searched for %s technicals", ticker),
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
