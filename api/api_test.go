package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestClearResultsHandlerMethodsAndMissingFiles(t *testing.T) {
	resultsDir := t.TempDir()
	handler := ClearResultsHandler(resultsDir)

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/clear", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("missing files", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/clear", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got := body["deleted"]; got != float64(0) {
			t.Fatalf("deleted = %v, want 0", got)
		}
	})
}

func TestClearResultsHandlerDeletesExistingFiles(t *testing.T) {
	resultsDir := t.TempDir()
	paths := []string{
		filepath.Join(resultsDir, "results-1.csv"),
		filepath.Join(resultsDir, "results-1.html"),
		filepath.Join(resultsDir, "history.csv"),
		filepath.Join(resultsDir, "index.html"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/clear", nil)
	rec := httptest.NewRecorder()

	ClearResultsHandler(resultsDir)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, stat err = %v", p, err)
		}
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body["deleted"]; got != float64(len(paths)) {
		t.Fatalf("deleted = %v, want %d", got, len(paths))
	}
}
