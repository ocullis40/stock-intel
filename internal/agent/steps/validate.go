package steps

import (
	"fmt"
	"math"
	"time"

	"github.com/oliver/stock-intel/internal/types"
)

// Validate inspects technical data for gaps and suspicious values.
func Validate(ticker string, data types.TechnicalData) (types.ValidationResult, types.AgentStep) {
	start := time.Now()
	var missing, suspicious []string

	if data.Price == nil {
		missing = append(missing, "price")
	}
	if data.RSI == nil {
		missing = append(missing, "rsi")
	}
	if data.MA50 == nil {
		missing = append(missing, "ma50")
	}
	if data.MA200 == nil {
		missing = append(missing, "ma200")
	}

	if data.RSI != nil && (*data.RSI < 0 || *data.RSI > 100) {
		suspicious = append(suspicious, fmt.Sprintf("RSI value %.1f is out of range [0-100]", *data.RSI))
	}
	if data.Price != nil && data.MA50 != nil {
		pctDiff := math.Abs((*data.Price - *data.MA50) / *data.MA50 * 100)
		if pctDiff > 50 {
			suspicious = append(suspicious, fmt.Sprintf("Price is %.0f%% from 50MA — may be stale", pctDiff))
		}
	}
	if data.Price != nil && data.MA200 != nil {
		pctDiff := math.Abs((*data.Price - *data.MA200) / *data.MA200 * 100)
		if pctDiff > 80 {
			suspicious = append(suspicious, fmt.Sprintf("Price is %.0f%% from 200MA — may be stale", pctDiff))
		}
	}
	if data.MA50 != nil && data.Price != nil {
		if *data.MA50 > *data.Price*3 || *data.MA50 < *data.Price*0.2 {
			suspicious = append(suspicious, "50MA seems implausible relative to current price")
		}
	}

	confidence := "high"
	if len(missing) > 0 || len(suspicious) > 0 {
		confidence = "medium"
	}
	if len(missing) > 1 || len(suspicious) > 1 {
		confidence = "low"
	}

	detail := "All values present and plausible"
	result := "success"
	if len(missing) > 0 || len(suspicious) > 0 {
		result = "partial"
		detail = fmt.Sprintf("Missing: %v, Suspicious: %v", missing, suspicious)
	}

	return types.ValidationResult{
			Missing: missing, Suspicious: suspicious, Confidence: confidence,
		}, types.AgentStep{
			Step: "validate", Action: fmt.Sprintf("Validated %s technicals", ticker),
			Timestamp: time.Now().UTC().Format(time.RFC3339), DurationMs: time.Since(start).Milliseconds(),
			Result: result, Detail: detail,
		}
}
