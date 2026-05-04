package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

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
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
