package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"dns-bench/api"
	"dns-bench/dashboard"
)

// ServeCmd handles serving the dashboard with API endpoints
func serveDashboard(resultsDir string, port string) error {
	// Ensure results directory exists
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	// Generate initial dashboard
	if err := dashboard.Generate(resultsDir); err != nil {
		log.Printf("Warning: failed to generate initial dashboard: %v", err)
	}

	// Serve static files from results directory
	fs := http.FileServer(http.Dir(resultsDir))

	// API endpoints
	http.HandleFunc("/api/clear-results", api.ClearResultsHandler(resultsDir))
	http.HandleFunc("/api/health", api.HealthHandler)

	// Serve static files (fallback)
	http.Handle("/", fs)

	addr := ":" + port
	log.Printf("Starting DNS Benchmark dashboard server on http://localhost%s", addr)
	return http.ListenAndServe(addr, nil)
}

// Add this to main() function
func init() {
	// Add serve command flag
	flag.StringVar(&servePort, "serve", "", "Port to serve dashboard on (e.g., 8080)")
}

var servePort string
