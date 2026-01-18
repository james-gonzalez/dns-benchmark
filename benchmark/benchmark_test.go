package benchmark

import (
	"testing"
	"time"
)

func TestClientMeasureUDP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	client := Client{Timeout: 2 * time.Second}
	result := client.Measure("8.8.8.8", "google.com")

	if result.Error != nil {
		t.Errorf("Expected no error for valid DNS query, got %v", result.Error)
	}
	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}
	if result.Server != "8.8.8.8" {
		t.Errorf("Expected server '8.8.8.8', got '%s'", result.Server)
	}
	if result.Domain != "google.com" {
		t.Errorf("Expected domain 'google.com', got '%s'", result.Domain)
	}
}

func TestClientMeasureDoT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	client := Client{Timeout: 3 * time.Second}
	result := client.Measure("tls://1.1.1.1", "google.com")

	if result.Error != nil {
		t.Errorf("Expected no error for valid DoT query, got %v", result.Error)
	}
	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}
}

func TestClientMeasureDoH(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	client := Client{Timeout: 3 * time.Second}
	result := client.Measure("https://dns.google/dns-query", "google.com")

	if result.Error != nil {
		t.Errorf("Expected no error for valid DoH query, got %v", result.Error)
	}
	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}
}

func TestClientMeasureInvalidDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	client := Client{Timeout: 2 * time.Second}
	result := client.Measure("8.8.8.8", "this-domain-definitely-does-not-exist-12345.com")

	// DNS query should complete but may return NXDOMAIN (not necessarily an error)
	// The important thing is we get a response
	if result.Duration == 0 {
		t.Error("Expected non-zero duration even for non-existent domain")
	}
}

func TestClientMeasureTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	client := Client{Timeout: 1 * time.Nanosecond} // Impossible timeout
	result := client.Measure("8.8.8.8", "google.com")

	if result.Error == nil {
		t.Error("Expected timeout error with impossible timeout")
	}
}

func TestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	config := Config{
		Servers:     []string{"8.8.8.8"},
		Domains:     []string{"google.com", "example.com"},
		Iterations:  1,
		Concurrency: 2,
		Timeout:     2 * time.Second,
		Verbose:     false,
	}

	results := Run(config)

	expectedResults := len(config.Servers) * len(config.Domains) * config.Iterations
	if len(results) != expectedResults {
		t.Errorf("Expected %d results, got %d", expectedResults, len(results))
	}

	for _, result := range results {
		if result.Server == "" {
			t.Error("Result missing server")
		}
		if result.Domain == "" {
			t.Error("Result missing domain")
		}
	}
}

func TestRunWithDuration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	config := Config{
		Servers:     []string{"8.8.8.8"},
		Domains:     []string{"google.com"},
		Concurrency: 5,
		Timeout:     2 * time.Second,
		Duration:    500 * time.Millisecond, // Run for 500ms
		Verbose:     false,
	}

	start := time.Now()
	results := Run(config)
	elapsed := time.Since(start)

	// Should complete around the duration time (with some overhead)
	if elapsed < config.Duration {
		t.Errorf("Benchmark completed too quickly: %v < %v", elapsed, config.Duration)
	}
	if elapsed > config.Duration+2*time.Second {
		t.Errorf("Benchmark took too long: %v > %v", elapsed, config.Duration+2*time.Second)
	}

	if len(results) == 0 {
		t.Error("Expected at least some results from duration-based benchmark")
	}
}

func TestRunEmptyConfig(t *testing.T) {
	// Empty config should not panic, but might produce no results
	config := Config{
		Servers:     []string{},
		Domains:     []string{"test.com"},
		Iterations:  1,
		Concurrency: 1,
		Timeout:     1 * time.Second,
	}

	results := Run(config)

	// With no servers, we expect 0 results
	if len(results) != 0 {
		t.Errorf("Expected 0 results with empty servers, got %d", len(results))
	}
}

// TestResultStructure tests the Result struct (no network required)
func TestResultStructure(t *testing.T) {
	result := Result{
		Server:   "8.8.8.8",
		Domain:   "example.com",
		Duration: 50 * time.Millisecond,
		Error:    nil,
	}

	if result.Server != "8.8.8.8" {
		t.Errorf("Expected server '8.8.8.8', got '%s'", result.Server)
	}
	if result.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", result.Domain)
	}
	if result.Duration != 50*time.Millisecond {
		t.Errorf("Expected duration 50ms, got %v", result.Duration)
	}
	if result.Error != nil {
		t.Errorf("Expected no error, got %v", result.Error)
	}
}

// TestClientStructure tests the Client struct (no network required)
func TestClientStructure(t *testing.T) {
	client := Client{
		Timeout: 5 * time.Second,
	}

	if client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", client.Timeout)
	}
	if client.httpClient != nil {
		t.Error("Expected httpClient to be nil initially")
	}
}

// TestConfigStructure tests the Config struct (no network required)
func TestConfigStructure(t *testing.T) {
	config := Config{
		Servers:      []string{"8.8.8.8", "1.1.1.1"},
		Domains:      []string{"google.com", "example.com"},
		Iterations:   10,
		Concurrency:  5,
		Timeout:      2 * time.Second,
		Duration:     30 * time.Second,
		Verbose:      true,
		ShowProgress: true,
	}

	if len(config.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(config.Servers))
	}
	if len(config.Domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(config.Domains))
	}
	if config.Iterations != 10 {
		t.Errorf("Expected 10 iterations, got %d", config.Iterations)
	}
	if config.Concurrency != 5 {
		t.Errorf("Expected concurrency 5, got %d", config.Concurrency)
	}
	if config.Timeout != 2*time.Second {
		t.Errorf("Expected timeout 2s, got %v", config.Timeout)
	}
	if config.Duration != 30*time.Second {
		t.Errorf("Expected duration 30s, got %v", config.Duration)
	}
	if !config.Verbose {
		t.Error("Expected verbose to be true")
	}
	if !config.ShowProgress {
		t.Error("Expected ShowProgress to be true")
	}
}

// TestProgressUpdateStructure tests the ProgressUpdate struct (no network required)
func TestProgressUpdateStructure(t *testing.T) {
	update := ProgressUpdate{
		Completed: 50,
		Total:     100,
		Elapsed:   5 * time.Second,
	}

	if update.Completed != 50 {
		t.Errorf("Expected 50 completed, got %d", update.Completed)
	}
	if update.Total != 100 {
		t.Errorf("Expected 100 total, got %d", update.Total)
	}
	if update.Elapsed != 5*time.Second {
		t.Errorf("Expected 5s elapsed, got %v", update.Elapsed)
	}
}

// TestJobStructure tests the Job struct (no network required)
func TestJobStructure(t *testing.T) {
	job := Job{
		Server: "8.8.8.8",
		Domain: "example.com",
	}

	if job.Server != "8.8.8.8" {
		t.Errorf("Expected server '8.8.8.8', got '%s'", job.Server)
	}
	if job.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", job.Domain)
	}
}

// TestRunEmptyDomains tests behavior with empty domains list
func TestRunEmptyDomains(t *testing.T) {
	config := Config{
		Servers:     []string{"8.8.8.8"},
		Domains:     []string{},
		Iterations:  1,
		Concurrency: 1,
		Timeout:     1 * time.Second,
	}

	results := Run(config)

	// With no domains, we expect 0 results
	if len(results) != 0 {
		t.Errorf("Expected 0 results with empty domains, got %d", len(results))
	}
}

// TestRunZeroIterations tests behavior with zero iterations
func TestRunZeroIterations(t *testing.T) {
	config := Config{
		Servers:     []string{"8.8.8.8"},
		Domains:     []string{"example.com"},
		Iterations:  0,
		Concurrency: 1,
		Timeout:     1 * time.Second,
	}

	results := Run(config)

	// With 0 iterations, we expect 0 results
	if len(results) != 0 {
		t.Errorf("Expected 0 results with 0 iterations, got %d", len(results))
	}
}

// TestRunMultipleServersAndDomains tests configuration without network
func TestRunMultipleServersAndDomains(t *testing.T) {
	config := Config{
		Servers:     []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"},
		Domains:     []string{"example.com", "test.com"},
		Iterations:  2,
		Concurrency: 5,
		Timeout:     100 * time.Millisecond,
	}

	expectedJobs := len(config.Servers) * len(config.Domains) * config.Iterations
	if expectedJobs != 12 {
		t.Errorf("Expected 12 total jobs (3*2*2), calculated %d", expectedJobs)
	}
}
