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
