package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// --- AuthMiddleware tests ---

func TestAuthMiddlewareNoEnvVar(t *testing.T) {
	// Ensure env var is unset.
	os.Unsetenv("DNS_BENCH_API_KEY")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	AuthMiddleware(inner)(rec, req)

	if !called {
		t.Fatal("expected inner handler to be called when env var is unset")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareValidKey(t *testing.T) {
	t.Setenv("DNS_BENCH_API_KEY", "secret123")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret123")
	rec := httptest.NewRecorder()
	AuthMiddleware(inner)(rec, req)

	if !called {
		t.Fatal("expected inner handler to be called with correct key")
	}
}

func TestAuthMiddlewareWrongKey(t *testing.T) {
	t.Setenv("DNS_BENCH_API_KEY", "secret123")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrongkey")
	rec := httptest.NewRecorder()
	AuthMiddleware(inner)(rec, req)

	if called {
		t.Fatal("expected inner handler NOT to be called with wrong key")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareMissingKey(t *testing.T) {
	t.Setenv("DNS_BENCH_API_KEY", "secret123")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-API-Key header set.
	rec := httptest.NewRecorder()
	AuthMiddleware(inner)(rec, req)

	if called {
		t.Fatal("expected inner handler NOT to be called with missing key")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- SubmitResultsHandler tests ---

func newSubmitHandler(t *testing.T) (http.HandlerFunc, string, *bool) {
	t.Helper()
	resultsDir := t.TempDir()
	mu := &sync.Mutex{}
	regenerateCalled := false
	regenerate := func(_ string) error {
		regenerateCalled = true
		return nil
	}
	return SubmitResultsHandler(resultsDir, mu, regenerate), resultsDir, &regenerateCalled
}

func TestSubmitResultsHandlerMethodNotAllowed(t *testing.T) {
	handler, _, _ := newSubmitHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/submit-results", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestSubmitResultsHandlerInvalidJSON(t *testing.T) {
	handler, _, _ := newSubmitHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader("{bad json"))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if !strings.Contains(body["error"], "invalid JSON") {
		t.Fatalf("error = %q, want to contain 'invalid JSON'", body["error"])
	}
}

func TestSubmitResultsHandlerMissingSource(t *testing.T) {
	handler, _, _ := newSubmitHandler(t)
	body := `{"source":"","results":[{"server":"1.1.1.1","domain":"google.com","duration_ms":10,"error":""}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "source is required" {
		t.Fatalf("error = %q, want 'source is required'", resp["error"])
	}
}

func TestSubmitResultsHandlerEmptyResults(t *testing.T) {
	handler, _, _ := newSubmitHandler(t)
	body := `{"source":"rpi","results":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "results must not be empty" {
		t.Fatalf("error = %q, want 'results must not be empty'", resp["error"])
	}
}

func TestSubmitResultsHandlerTooManyResults(t *testing.T) {
	handler, _, _ := newSubmitHandler(t)
	// Build a payload with 10001 results.
	var sb strings.Builder
	sb.WriteString(`{"source":"rpi","results":[`)
	for i := 0; i < 10001; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"server":"1.1.1.1","domain":"google.com","duration_ms":1,"error":""}`)
	}
	sb.WriteString(`]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader(sb.String()))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "10000") {
		t.Fatalf("error = %q, want to mention 10000", resp["error"])
	}
}

func TestSubmitResultsHandlerValidRequest(t *testing.T) {
	resultsDir := t.TempDir()
	mu := &sync.Mutex{}
	regenerateCalled := false
	regenerate := func(_ string) error {
		regenerateCalled = true
		return nil
	}
	handler := SubmitResultsHandler(resultsDir, mu, regenerate)

	body := `{"source":"rpi-4b","results":[{"server":"1.1.1.1","domain":"google.com","duration_ms":12.34,"error":""},{"server":"8.8.8.8","domain":"github.com","duration_ms":5.0,"error":"timeout"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["accepted"] != float64(2) {
		t.Fatalf("accepted = %v, want 2", resp["accepted"])
	}
	if resp["source"] != "rpi-4b" {
		t.Fatalf("source = %v, want rpi-4b", resp["source"])
	}

	// history.csv should exist and have header + 2 data rows.
	histPath := filepath.Join(resultsDir, "history.csv")
	f, err := os.Open(histPath)
	if err != nil {
		t.Fatalf("history.csv not created: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read history.csv: %v", err)
	}
	if len(rows) != 3 { // header + 2 results
		t.Fatalf("history.csv rows = %d, want 3", len(rows))
	}
	if rows[0][5] != "Source" {
		t.Fatalf("header[5] = %q, want Source", rows[0][5])
	}
	if rows[1][5] != "rpi-4b" {
		t.Fatalf("row[1][5] = %q, want rpi-4b", rows[1][5])
	}

	// A per-submission results CSV should exist.
	matches, _ := filepath.Glob(filepath.Join(resultsDir, "results-*-rpi-4b.csv"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 per-submission CSV, got %d", len(matches))
	}

	// Regenerate should have been called.
	if !regenerateCalled {
		t.Fatal("expected regenerate to be called")
	}
}

func TestSubmitResultsHandlerAppendsToExistingHistory(t *testing.T) {
	resultsDir := t.TempDir()
	mu := &sync.Mutex{}
	handler := SubmitResultsHandler(resultsDir, mu, nil)

	submit := func(source, server string) {
		body := `{"source":"` + source + `","results":[{"server":"` + server + `","domain":"example.com","duration_ms":1,"error":""}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/submit-results", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("submit %s: status = %d, body = %s", source, rec.Code, rec.Body.String())
		}
	}

	submit("device-a", "1.1.1.1")
	submit("device-b", "8.8.8.8")

	histPath := filepath.Join(resultsDir, "history.csv")
	f, err := os.Open(histPath)
	if err != nil {
		t.Fatalf("history.csv not found: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read history.csv: %v", err)
	}
	// 1 header + 2 data rows; no duplicate header.
	if len(rows) != 3 {
		t.Fatalf("history.csv rows = %d, want 3 (1 header + 2 data)", len(rows))
	}
}
