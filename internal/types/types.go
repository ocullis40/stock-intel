package types

// TechnicalData holds price and indicator values from search.
type TechnicalData struct {
	Price     *float64 `json:"price"`
	Change    *float64 `json:"change"`
	ChangePct *float64 `json:"changePct"`
	RSI       *float64 `json:"rsi"`
	MA50      *float64 `json:"ma50"`
	MA200     *float64 `json:"ma200"`
	Source    string   `json:"source"`
}

// ValidationResult flags gaps and suspicious values.
type ValidationResult struct {
	Missing    []string `json:"missing"`
	Suspicious []string `json:"suspicious"`
	Confidence string   `json:"confidence"` // "high", "medium", "low"
}

// NewsData holds qualitative analysis.
type NewsData struct {
	Headline           string   `json:"headline"`
	Bullets            []string `json:"bullets"`
	Risk               string   `json:"risk"`
	Catalyst           string   `json:"catalyst"`
	SentimentShortTerm string   `json:"sentimentShortTerm"`
	SentimentMedTerm   string   `json:"sentimentMedTerm"`
}

// AgentStep records a single step in the agent's execution trace.
type AgentStep struct {
	Step       string `json:"step"`
	Action     string `json:"action"`
	Timestamp  string `json:"timestamp"`
	DurationMs int64  `json:"durationMs"`
	Result     string `json:"result"` // "success", "partial", "failed"
	Detail     string `json:"detail"`
}

// TickerIntel is the final output for one ticker.
type TickerIntel struct {
	Ticker     string           `json:"ticker"`
	Name       string           `json:"name"`
	Timestamp  string           `json:"timestamp"`
	Technicals TechnicalData    `json:"technicals"`
	News       NewsData         `json:"news"`
	MASignal   string           `json:"maSignal"`
	Validation ValidationResult `json:"validation"`
	AgentLog   []AgentStep      `json:"agentLog"`
}

// ProgressUpdate is sent during analysis.
type ProgressUpdate struct {
	Ticker     string `json:"ticker"`
	Step       string `json:"step"`
	StepIndex  int    `json:"stepIndex"`
	TotalSteps int    `json:"totalSteps"`
}

// Usage holds token counts from a single API response.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// UsageSummary holds aggregate usage stats for a run.
type UsageSummary struct {
	TotalInputTokens  int     `json:"totalInputTokens"`
	TotalOutputTokens int     `json:"totalOutputTokens"`
	TotalTokens       int     `json:"totalTokens"`
	APICalls          int     `json:"apiCalls"`
	EstimatedCost     float64 `json:"estimatedCost"`
	BudgetUsedPct     float64 `json:"budgetUsedPct"`
}

// Config holds agent runtime configuration.
type Config struct {
	Tickers        []string `json:"tickers"`
	Model          string   `json:"model"`
	Concurrency    int      `json:"concurrency"`
	MaxTokensPerRun int     `json:"maxTokensPerRun,omitempty"`
	MaxTickers     int      `json:"maxTickers,omitempty"`
	RequestDelayMs int      `json:"requestDelayMs,omitempty"`
	Agent struct{} `json:"agent,omitempty"`
}
