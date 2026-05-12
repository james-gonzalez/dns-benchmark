package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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

	// Shared mutex for all write handlers
	var historyMu sync.Mutex

	// Serve static files from results directory
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(resultsDir))

	// API endpoints
	mux.HandleFunc("/api/clear-results", api.ClearResultsHandler(resultsDir))
	mux.HandleFunc("/api/health", api.HealthHandler)
	mux.HandleFunc("/api/submit-results", api.AuthMiddleware(api.SubmitResultsHandler(resultsDir, &historyMu, dashboard.Generate)))

	// Serve static files (fallback)
	mux.Handle("/", fs)

	addr := ":" + port
	log.Printf("Starting DNS Benchmark dashboard server on http://localhost%s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server.ListenAndServe()
}

// Add this to main() function
func init() {
	// Add serve command flag
	flag.StringVar(&servePort, "serve", "", "Port to serve dashboard on (e.g., 8080)")
}

var servePort string
