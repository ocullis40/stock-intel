package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/oliver/stock-intel/internal/agent"
	"github.com/oliver/stock-intel/internal/config"
	"github.com/oliver/stock-intel/internal/types"
	"github.com/oliver/stock-intel/internal/usage"
)

// Server holds the HTTP server state.
type Server struct {
	results          map[string]types.TickerIntel
	progress         []types.ProgressUpdate
	analysisRunning  bool
	lastUsageSummary *types.UsageSummary
	mu               sync.RWMutex
	dashboardDir     string
}

// New creates a new server.
func New(dashboardDir string) *Server {
	return &Server{
		results:      make(map[string]types.TickerIntel),
		dashboardDir: dashboardDir,
	}
}

// Start launches the HTTP server.
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/tickers", s.handleTickers)
	mux.HandleFunc("/api/tickers/", s.handleTickerDelete)
	mux.HandleFunc("/api/results", s.handleResults)
	mux.HandleFunc("/api/analyze", s.handleAnalyze)
	mux.HandleFunc("/api/analyze/", s.handleAnalyzeSingle)
	mux.HandleFunc("/api/progress", s.handleProgress)

	fs := http.FileServer(http.Dir(s.dashboardDir))
	mux.Handle("/", fs)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("\n  Stock Intel Dashboard\n")
	fmt.Printf("  ─────────────────────\n")
	fmt.Printf("  http://localhost:%d\n\n", port)

	return http.ListenAndServe(addr, corsMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) handleTickers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"tickers": cfg.Tickers})

	case http.MethodPost:
		var body struct {
			Ticker string `json:"ticker"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Ticker == "" {
			writeError(w, 400, "ticker is required")
			return
		}
		cfg, err := config.AddTicker(body.Ticker)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"tickers": cfg.Tickers})

	default:
		writeError(w, 405, "method not allowed")
	}
}

func (s *Server) handleTickerDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, 405, "method not allowed")
		return
	}

	ticker := strings.TrimPrefix(r.URL.Path, "/api/tickers/")
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		writeError(w, 400, "ticker is required")
		return
	}

	cfg, err := config.RemoveTicker(ticker)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	s.mu.Lock()
	delete(s.results, ticker)
	s.mu.Unlock()

	writeJSON(w, map[string]any{"tickers": cfg.Tickers})
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	writeJSON(w, map[string]any{
		"results":  s.results,
		"running":  s.analysisRunning,
		"progress": s.progress,
		"usage":    s.lastUsageSummary,
	})
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}

	s.mu.RLock()
	if s.analysisRunning {
		s.mu.RUnlock()
		writeError(w, 409, "analysis already in progress")
		return
	}
	s.mu.RUnlock()

	cfg, err := config.Load()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	s.mu.Lock()
	s.analysisRunning = true
	s.progress = nil
	s.mu.Unlock()

	writeJSON(w, map[string]any{"status": "started", "tickers": cfg.Tickers})

	go func() {
		results, usageSummary := agent.AnalyzeAll(cfg, func(update types.ProgressUpdate) {
			s.mu.Lock()
			found := false
			for i, p := range s.progress {
				if p.Ticker == update.Ticker {
					s.progress[i] = update
					found = true
					break
				}
			}
			if !found {
				s.progress = append(s.progress, update)
			}
			s.mu.Unlock()
		})

		s.mu.Lock()
		s.results = results
		s.lastUsageSummary = &usageSummary
		s.analysisRunning = false
		s.mu.Unlock()
	}()
}

func (s *Server) handleAnalyzeSingle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}

	ticker := strings.TrimPrefix(r.URL.Path, "/api/analyze/")
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		writeError(w, 400, "ticker is required")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	tracker := usage.New(cfg.MaxTokensPerRun, cfg.RequestDelayMs)
	intel := agent.AnalyzeTicker(ticker, cfg, func(update types.ProgressUpdate) {
		fmt.Printf("  [%s] %s\n", update.Ticker, update.Step)
	}, tracker)

	summary := tracker.Summary()

	s.mu.Lock()
	s.results[ticker] = intel
	s.lastUsageSummary = &summary
	s.mu.Unlock()

	writeJSON(w, map[string]any{
		"result": intel,
		"usage":  summary,
	})
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method not allowed")
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	writeJSON(w, map[string]any{
		"running":  s.analysisRunning,
		"progress": s.progress,
	})
}
