package usage

import (
	"fmt"
	"sync"
	"time"

	"github.com/oliver/stock-intel/internal/types"
)

// Tracker accumulates token usage across API calls and enforces budgets.
type Tracker struct {
	mu             sync.Mutex
	inputTokens    int
	outputTokens   int
	apiCalls       int
	budget         int // 0 = unlimited
	requestDelayMs int
	lastCallTime   time.Time
}

// New creates a Tracker with the given budget and rate-limit delay.
func New(budget, requestDelayMs int) *Tracker {
	return &Tracker{
		budget:         budget,
		requestDelayMs: requestDelayMs,
	}
}

// PreCallCheck enforces the token budget and rate-limit delay.
// Returns an error if the budget has been exceeded.
func (t *Tracker) PreCallCheck() error {
	t.mu.Lock()
	if t.budget > 0 && (t.inputTokens+t.outputTokens) >= t.budget {
		t.mu.Unlock()
		return fmt.Errorf("token budget exceeded: used %d / %d tokens", t.inputTokens+t.outputTokens, t.budget)
	}

	var sleepDur time.Duration
	if t.requestDelayMs > 0 && !t.lastCallTime.IsZero() {
		elapsed := time.Since(t.lastCallTime)
		delay := time.Duration(t.requestDelayMs) * time.Millisecond
		if elapsed < delay {
			sleepDur = delay - elapsed
		}
	}
	t.lastCallTime = time.Now().Add(sleepDur)
	t.mu.Unlock()

	if sleepDur > 0 {
		time.Sleep(sleepDur)
	}

	return nil
}

// RecordUsage adds token counts from a single API call.
func (t *Tracker) RecordUsage(u types.Usage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inputTokens += u.InputTokens
	t.outputTokens += u.OutputTokens
	t.apiCalls++
}

// Summary returns the aggregate usage statistics.
func (t *Tracker) Summary() types.UsageSummary {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := t.inputTokens + t.outputTokens
	// Sonnet pricing: $3/MTok input, $15/MTok output
	cost := float64(t.inputTokens)/1_000_000*3.0 + float64(t.outputTokens)/1_000_000*15.0

	budgetPct := 0.0
	if t.budget > 0 {
		budgetPct = float64(total) / float64(t.budget) * 100
	}

	return types.UsageSummary{
		TotalInputTokens:  t.inputTokens,
		TotalOutputTokens: t.outputTokens,
		TotalTokens:       total,
		APICalls:          t.apiCalls,
		EstimatedCost:     cost,
		BudgetUsedPct:     budgetPct,
	}
}
