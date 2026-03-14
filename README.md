# Stock Intel

Agentic stock research dashboard in Go. Multi-pass AI analysis with technical indicators, news synthesis, and full execution traces.

## What it does

For each ticker in your watchlist, an agent runs a multi-step research pipeline:

1. **Technicals search** — finds RSI (14-day), 50-day MA, 200-day MA, current price
2. **Validation** — checks for missing/suspicious values (RSI out of range, MAs implausibly far from price)
3. **Gap filling** — if validation found holes, runs targeted searches on different sources (up to 2 retries)
4. **News research** — separate pass for recent developments, analyst sentiment, risks, catalysts
5. **Synthesis** — computes MA signal (golden cross, death cross, trend position) with RSI context

Each step logs what it did, how long it took, and whether it succeeded — viewable in the agent trace on each card.

## Quick start

```bash
# Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# Build and run the dashboard
make run

# Open http://localhost:3847
```

Or run directly without building:

```bash
go run ./cmd/server
```

## Usage

### Web dashboard

- **Add/remove tickers** — type in the input field and press Enter, or click × on a tag
- **Analyze All** — kicks off the full agent pipeline for every ticker
- **Agent Trace** — click to expand the step-by-step execution log on each card
- **Confidence indicator** — HIGH (all data found), MEDIUM (minor gaps), LOW (significant missing data)

### CLI mode

```bash
# Build and run headless analysis
make analyze

# Or directly
go run ./cmd/cli
```

### API

```
GET    /api/tickers              # list tickers
POST   /api/tickers              # add ticker  { "ticker": "AAPL" }
DELETE /api/tickers/:ticker      # remove ticker
POST   /api/analyze              # run full analysis (background)
POST   /api/analyze/:ticker      # analyze single ticker (blocking)
GET    /api/results              # latest results + progress
GET    /api/progress             # poll analysis progress
```

## Configuration

`config.json` at the project root (also editable via the dashboard):

```json
{
  "tickers": ["QQQ", "USO", "IREN", "SNDK", "HOOD"],
  "model": "claude-sonnet-4-20250514",
  "concurrency": 2,
  "agent": {
    "maxRetries": 2,
    "validateTechnicals": true
  }
}
```

- **tickers** — managed via the UI or API, persisted to this file
- **model** — which Claude model to use for research
- **concurrency** — how many tickers to research in parallel (goroutines)
- **maxRetries** — how many gap-filling attempts per ticker
- **validateTechnicals** — whether to run validation + gap filling at all

## Project structure

```
stock-intel/
├── cmd/
│   ├── server/main.go        # dashboard server entry point
│   └── cli/main.go           # headless CLI entry point
├── internal/
│   ├── agent/
│   │   ├── types.go           # all data types
│   │   ├── client.go          # Anthropic API client (raw HTTP, no SDK)
│   │   ├── agent.go           # orchestrator — runs multi-step loop
│   │   └── steps/
│   │       ├── technicals.go  # Step 1: search for RSI, MAs, price
│   │       ├── validate.go    # Step 2: check data quality
│   │       ├── fillgaps.go    # Step 3: targeted retry for missing values
│   │       ├── news.go        # Step 4: news and sentiment research
│   │       └── synthesize.go  # Step 5: compute MA signal
│   ├── config/
│   │   └── config.go          # config.json read/write/ticker CRUD
│   └── server/
│       └── server.go          # HTTP server + API routes
├── dashboard/
│   └── index.html             # single-file web UI
├── config.json                # default watchlist and settings
├── Makefile
├── go.mod
└── README.md
```

## Zero external dependencies

The entire project uses only the Go standard library. The Anthropic API client is a plain `net/http` call — no SDK needed. This means `go build` just works with no `go mod tidy` dance.

## What makes it agentic

The key difference from a single-shot API call:

- **Validation loop** — the agent inspects its own output after the technicals search. If the RSI came back null or the 200MA looks stale, it decides to search again with different queries.
- **Step-by-step reasoning** — each step sees what the previous steps produced and acts accordingly. The gap-filler knows exactly which fields are missing. The synthesizer reads the actual MA values to compute the signal.
- **Observable execution** — every step is logged with timing, result status, and detail. You can see exactly what the agent did and where it struggled.

## Future improvements

- [ ] SSE streaming for real-time progress (replace polling)
- [ ] Historical results storage (SQLite — track changes over time)
- [ ] Scheduled runs via built-in cron
- [ ] Sector/correlation analysis across the watchlist
- [ ] Pluggable data sources (Yahoo Finance API for exact quotes)
- [ ] Confidence-weighted alerts (notify when high-confidence bearish signal)
