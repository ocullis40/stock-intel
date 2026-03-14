package steps

import (
	"fmt"
	"time"

	"github.com/oliver/stock-intel/internal/agent"
)

// Synthesize computes the MA signal from technical data.
// Pure logic — no API calls needed.
func Synthesize(ticker string, tech agent.TechnicalData) (string, agent.AgentStep) {
	start := time.Now()

	if tech.Price == nil || tech.MA50 == nil || tech.MA200 == nil {
		// Partial signal
		var parts []string
		if tech.Price != nil {
			parts = append(parts, fmt.Sprintf("Price: $%.2f", *tech.Price))
		}
		if tech.MA50 != nil {
			parts = append(parts, fmt.Sprintf("50MA: $%.2f", *tech.MA50))
		}
		if tech.MA200 != nil {
			parts = append(parts, fmt.Sprintf("200MA: $%.2f", *tech.MA200))
		}
		if tech.RSI != nil {
			parts = append(parts, fmt.Sprintf("RSI: %.1f", *tech.RSI))
		}

		signal := "Insufficient technical data to compute signal."
		if len(parts) > 0 {
			signal = fmt.Sprintf("Incomplete data — %s. Cannot compute full MA signal.", joinComma(parts))
		}

		return signal, agent.AgentStep{
			Step:       "synthesize",
			Action:     fmt.Sprintf("Attempted MA signal for %s", ticker),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: time.Since(start).Milliseconds(),
			Result:     "partial",
			Detail:     "Missing MA values — computed partial signal",
		}
	}

	price := *tech.Price
	ma50 := *tech.MA50
	ma200 := *tech.MA200
	above50 := price > ma50
	above200 := price > ma200
	goldenCross := ma50 > ma200

	rsiNote := ""
	if tech.RSI != nil {
		rsi := *tech.RSI
		switch {
		case rsi >= 70:
			rsiNote = fmt.Sprintf(" RSI at %.1f signals overbought conditions.", rsi)
		case rsi <= 30:
			rsiNote = fmt.Sprintf(" RSI at %.1f signals oversold conditions.", rsi)
		default:
			rsiNote = fmt.Sprintf(" RSI at %.1f.", rsi)
		}
	}

	var signal string
	switch {
	case above50 && above200 && goldenCross:
		signal = fmt.Sprintf("Strong uptrend — price above both MAs with golden cross (50MA > 200MA).%s", rsiNote)
	case above50 && above200:
		signal = fmt.Sprintf("Uptrend — price above both MAs, but 50MA still below 200MA.%s", rsiNote)
	case !above50 && above200:
		signal = fmt.Sprintf("Caution — price slipped below 50MA but still above 200MA. Watch for further weakness.%s", rsiNote)
	case above50 && !above200:
		signal = fmt.Sprintf("Recovery attempt — price reclaimed 50MA but still below 200MA.%s", rsiNote)
	case !above50 && !above200 && !goldenCross:
		signal = fmt.Sprintf("Downtrend — price below both MAs with death cross (50MA < 200MA).%s", rsiNote)
	default:
		signal = fmt.Sprintf("Price below both MAs.%s", rsiNote)
	}

	return signal, agent.AgentStep{
		Step:       "synthesize",
		Action:     fmt.Sprintf("Computed MA signal for %s", ticker),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMs: time.Since(start).Milliseconds(),
		Result:     "success",
		Detail:     signal,
	}
}

func joinComma(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
