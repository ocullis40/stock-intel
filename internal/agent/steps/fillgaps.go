package steps

import (
	"fmt"
	"strings"
	"time"

	"github.com/oliver/stock-intel/internal/agent"
)

type gapResult struct {
	Price  *float64 `json:"price"`
	RSI    *float64 `json:"rsi"`
	MA50   *float64 `json:"ma50"`
	MA200  *float64 `json:"ma200"`
	Source string   `json:"source"`
}

// FillGaps runs a targeted search for specific missing values.
// Only called when validation found gaps.
func FillGaps(ticker string, current agent.TechnicalData, validation agent.ValidationResult, model string) (agent.TechnicalData, agent.AgentStep) {
	start := time.Now()

	if len(validation.Missing) == 0 && len(validation.Suspicious) == 0 {
		return current, agent.AgentStep{
			Step:       "fill_gaps",
			Action:     "Skipped — no gaps to fill",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: 0,
			Result:     "success",
			Detail:     "No gaps detected",
		}
	}

	// Build targeted prompt
	var needs []string
	for _, m := range validation.Missing {
		switch m {
		case "rsi":
			needs = append(needs, fmt.Sprintf(`RSI (14-day) for %s — try "barchart %s technical" or "tradingview %s technicals"`, ticker, ticker, ticker))
		case "ma50":
			needs = append(needs, fmt.Sprintf("50-day simple moving average for %s", ticker))
		case "ma200":
			needs = append(needs, fmt.Sprintf("200-day simple moving average for %s", ticker))
		case "price":
			needs = append(needs, fmt.Sprintf("Current price for %s", ticker))
		}
	}

	// Format existing values for the prompt
	fmtVal := func(v *float64) string {
		if v == nil {
			return "null"
		}
		return fmt.Sprintf("%.2f", *v)
	}

	prompt := fmt.Sprintf(`I need SPECIFIC technical values for %s that I couldn't find before. Search specifically for:

%s

Try different sources: Barchart.com, StockAnalysis.com, TradingView, MarketWatch, Yahoo Finance.

Return ONLY JSON with the values you find:
{"price":%s,"rsi":%s,"ma50":%s,"ma200":%s,"source":"where you found it"}

Keep any existing non-null values I provided. Only replace null values with what you find. Return ONLY the JSON.`,
		ticker,
		strings.Join(needs, "\n"),
		fmtVal(current.Price),
		fmtVal(current.RSI),
		fmtVal(current.MA50),
		fmtVal(current.MA200),
	)

	raw, err := agent.SearchAndExtract(prompt, model)
	if err != nil {
		return current, agent.AgentStep{
			Step:       "fill_gaps",
			Action:     fmt.Sprintf("Retry search for %s gaps: %s", ticker, strings.Join(validation.Missing, ", ")),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: time.Since(start).Milliseconds(),
			Result:     "failed",
			Detail:     fmt.Sprintf("API error: %v", err),
		}
	}

	parsed, err := agent.ParseJSON[gapResult](raw)
	if err != nil {
		return current, agent.AgentStep{
			Step:       "fill_gaps",
			Action:     fmt.Sprintf("Retry search for %s gaps", ticker),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: time.Since(start).Milliseconds(),
			Result:     "failed",
			Detail:     fmt.Sprintf("Parse error: %v", err),
		}
	}

	// Merge: only fill in values that were null
	merged := current
	if merged.Price == nil && parsed.Price != nil {
		merged.Price = parsed.Price
	}
	if merged.RSI == nil && parsed.RSI != nil {
		merged.RSI = parsed.RSI
	}
	if merged.MA50 == nil && parsed.MA50 != nil {
		merged.MA50 = parsed.MA50
	}
	if merged.MA200 == nil && parsed.MA200 != nil {
		merged.MA200 = parsed.MA200
	}
	merged.Source = fmt.Sprintf("%s + %s", current.Source, parsed.Source)

	filled := 0
	for _, m := range validation.Missing {
		switch m {
		case "price":
			if merged.Price != nil {
				filled++
			}
		case "rsi":
			if merged.RSI != nil {
				filled++
			}
		case "ma50":
			if merged.MA50 != nil {
				filled++
			}
		case "ma200":
			if merged.MA200 != nil {
				filled++
			}
		}
	}

	result := "partial"
	if filled == len(validation.Missing) {
		result = "success"
	}

	return merged, agent.AgentStep{
		Step:       "fill_gaps",
		Action:     fmt.Sprintf("Retry search for %s gaps: %s", ticker, strings.Join(validation.Missing, ", ")),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMs: time.Since(start).Milliseconds(),
		Result:     result,
		Detail:     fmt.Sprintf("Filled %d/%d gaps from %s", filled, len(validation.Missing), parsed.Source),
	}
}
