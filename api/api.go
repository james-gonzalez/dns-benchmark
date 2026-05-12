package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SubmitRequest is the payload accepted by SubmitResultsHandler.
type SubmitRequest struct {
	Source  string       `json:"source"`
	Results []JSONResult `json:"results"`
}

// JSONResult mirrors the -json output format of dns-bench.
type JSONResult struct {
	Server     string  `json:"server"`
	Domain     string  `json:"domain"`
	DurationMs float64 `json:"duration_ms"`
	Error      string  `json:"error"`
}

// AuthMiddleware validates the X-API-Key header against the DNS_BENCH_API_KEY
// environment variable. If the variable is unset, all requests are allowed.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("DNS_BENCH_API_KEY")
		if secret != "" && r.Header.Get("X-API-Key") != secret {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// SubmitResultsHandler accepts benchmark results pushed from remote devices and
// appends them to history.csv, writes a per-submission results CSV, then
// regenerates the dashboard.
func SubmitResultsHandler(resultsDir string, mu *sync.Mutex, regenerate func(string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %v", err)})
			return
		}

		if req.Source == "" {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "source is required"})
			return
		}
		if len(req.Results) == 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "results must not be empty"})
			return
		}
		if len(req.Results) > 10000 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "results exceeds maximum of 10000"})
			return
		}

		timestamp := time.Now().UTC()
		tsStr := timestamp.Format(time.RFC3339)
		fileTS := timestamp.Format("2006-01-02T15-04-05Z")

		mu.Lock()
		err := writeSubmission(resultsDir, req, tsStr, fileTS)
		mu.Unlock()

		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Regenerate dashboard outside the lock — it only reads files.
		if regenerate != nil {
			if err := regenerate(resultsDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to regenerate dashboard: %v\n", err)
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"accepted": len(req.Results),
			"source":   req.Source,
		})
	}
}

// writeSubmission appends to history.csv and writes a per-submission results CSV.
// Caller must hold the mutex.
func writeSubmission(resultsDir string, req SubmitRequest, tsStr, fileTS string) error {
	historyPath := filepath.Join(resultsDir, "history.csv")
	historyExists := true
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		historyExists = false
	}

	hf, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open history.csv: %w", err)
	}
	defer hf.Close()

	hw := csv.NewWriter(hf)
	if !historyExists {
		if err := hw.Write([]string{"Timestamp", "Server", "Domain", "Duration_ms", "Error", "Source"}); err != nil {
			return fmt.Errorf("failed to write history header: %w", err)
		}
	}
	for _, res := range req.Results {
		if err := hw.Write([]string{
			tsStr,
			res.Server,
			res.Domain,
			fmt.Sprintf("%.4f", res.DurationMs),
			res.Error,
			req.Source,
		}); err != nil {
			return fmt.Errorf("failed to write history row: %w", err)
		}
	}
	hw.Flush()
	if err := hw.Error(); err != nil {
		return fmt.Errorf("failed to flush history.csv: %w", err)
	}

	// Per-submission results CSV: results-<timestamp>-<source>.csv
	runPath := filepath.Join(resultsDir, fmt.Sprintf("results-%s-%s.csv", fileTS, req.Source))
	rf, err := os.Create(runPath)
	if err != nil {
		return fmt.Errorf("failed to create run CSV: %w", err)
	}
	defer rf.Close()

	rw := csv.NewWriter(rf)
	if err := rw.Write([]string{"Server", "Domain", "Duration_ms", "Error"}); err != nil {
		return fmt.Errorf("failed to write run CSV header: %w", err)
	}
	for _, res := range req.Results {
		if err := rw.Write([]string{
			res.Server,
			res.Domain,
			fmt.Sprintf("%.4f", res.DurationMs),
			res.Error,
		}); err != nil {
			return fmt.Errorf("failed to write run CSV row: %w", err)
		}
	}
	rw.Flush()
	return rw.Error()
}

// ClearResultsHandler deletes all result files from the results directory
func ClearResultsHandler(resultsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// List all result files
		files, err := filepath.Glob(filepath.Join(resultsDir, "results-*.csv"))
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to list files: %v", err),
			})
			return
		}

		deleted := 0
		var errors []string

		// Delete CSV files
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", filepath.Base(f), err))
			} else {
				deleted++
			}
		}

		// Delete HTML files
		htmlFiles, err := filepath.Glob(filepath.Join(resultsDir, "results-*.html"))
		if err == nil {
			for _, f := range htmlFiles {
				if err := os.Remove(f); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", filepath.Base(f), err))
				} else {
					deleted++
				}
			}
		}

		// Delete history.csv
		historyPath := filepath.Join(resultsDir, "history.csv")
		if err := os.Remove(historyPath); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("Failed to delete history.csv: %v", err))
		} else if err == nil {
			deleted++
		}

		// Delete index.html
		indexPath := filepath.Join(resultsDir, "index.html")
		if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("Failed to delete index.html: %v", err))
		} else if err == nil {
			deleted++
		}

		response := map[string]interface{}{
			"deleted": deleted,
		}
		if len(errors) > 0 {
			response["errors"] = errors
		}

		respondJSON(w, http.StatusOK, response)
	}
}

// HealthHandler returns the health status of the API
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n"))
}
