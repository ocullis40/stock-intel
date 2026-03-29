package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oliver/stock-intel/internal/config"
	"github.com/oliver/stock-intel/internal/server"
)

func main() {
	// Find project root (where config.json lives)
	root := findRoot()

	loadEnv(filepath.Join(root, ".env"))
	config.Init(filepath.Join(root, "config.json"))

	port := 3847
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	dashboardDir := filepath.Join(root, "dashboard")
	srv := server.New(dashboardDir)

	if err := srv.Start(port); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

// loadEnv reads a .env file and sets any variables not already in the environment.
func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if _, exists := os.LookupEnv(key); !exists && val != "" {
			os.Setenv(key, val)
		}
	}
}

// findRoot walks up from cwd looking for config.json.
func findRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Fallback to cwd
			cwd, _ := os.Getwd()
			return cwd
		}
		dir = parent
	}
}
