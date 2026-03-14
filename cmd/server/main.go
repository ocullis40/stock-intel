package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/oliver/stock-intel/internal/config"
	"github.com/oliver/stock-intel/internal/server"
)

func main() {
	// Find project root (where config.json lives)
	root := findRoot()

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
