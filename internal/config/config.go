package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oliver/stock-intel/internal/types"
)

var (
	mu      sync.RWMutex
	cfgPath string
)

// Init sets the config file path. Call once at startup.
func Init(path string) {
	cfgPath = path
}

// Load reads config from disk.
func Load() (types.Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return types.Config{}, err
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return types.Config{}, err
	}

	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 2
	}
	if cfg.Agent.MaxRetries == 0 {
		cfg.Agent.MaxRetries = 2
	}

	return cfg, nil
}

// Save writes config to disk.
func Save(cfg types.Config) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, append(data, '\n'), 0644)
}

// AddTicker adds a ticker if not already present.
func AddTicker(ticker string) (types.Config, error) {
	cfg, err := Load()
	if err != nil {
		return cfg, err
	}

	t := strings.ToUpper(strings.TrimSpace(ticker))
	if t == "" {
		return cfg, nil
	}

	for _, existing := range cfg.Tickers {
		if existing == t {
			return cfg, nil
		}
	}

	cfg.Tickers = append(cfg.Tickers, t)
	return cfg, Save(cfg)
}

// RemoveTicker removes a ticker.
func RemoveTicker(ticker string) (types.Config, error) {
	cfg, err := Load()
	if err != nil {
		return cfg, err
	}

	t := strings.ToUpper(strings.TrimSpace(ticker))
	var filtered []string
	for _, existing := range cfg.Tickers {
		if existing != t {
			filtered = append(filtered, existing)
		}
	}

	cfg.Tickers = filtered
	return cfg, Save(cfg)
}

// DefaultPath returns the default config path relative to the executable.
func DefaultPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}
