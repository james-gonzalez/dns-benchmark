package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dns-bench/benchmark"
)

func TestCalculateStats(t *testing.T) {
	results := []benchmark.Result{
		{Server: "8.8.8.8", Domain: "google.com", Duration: 10 * time.Millisecond, Error: nil},
		{Server: "8.8.8.8", Domain: "yahoo.com", Duration: 20 * time.Millisecond, Error: nil},
		{Server: "1.1.1.1", Domain: "google.com", Duration: 5 * time.Millisecond, Error: nil},
		{Server: "8.8.8.8", Domain: "error.com", Duration: 0, Error: os.ErrNotExist},
	}

	stats := calculateStats(results)

	if len(stats) != 2 {
		t.Errorf("Expected 2 servers in stats, got %d", len(stats))
	}

	// Find stats for 8.8.8.8
	var googleStats *ServerStats
	for _, s := range stats {
		if s.Server == "8.8.8.8" {
			googleStats = s
			break
		}
	}

	if googleStats == nil {
		t.Fatal("Expected to find stats for 8.8.8.8")
	}

	if googleStats.Total != 3 {
		t.Errorf("Expected 3 total queries for 8.8.8.8, got %d", googleStats.Total)
	}

	if googleStats.Success != 2 {
		t.Errorf("Expected 2 successful queries, got %d", googleStats.Success)
	}

	if googleStats.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", googleStats.Errors)
	}

	expectedAvg := 15 * time.Millisecond
	if googleStats.Avg != expectedAvg {
		t.Errorf("Expected avg %v, got %v", expectedAvg, googleStats.Avg)
	}

	if googleStats.Min != 10*time.Millisecond {
		t.Errorf("Expected min 10ms, got %v", googleStats.Min)
	}

	if googleStats.Max != 20*time.Millisecond {
		t.Errorf("Expected max 20ms, got %v", googleStats.Max)
	}

	// Check that 1.1.1.1 comes first (lower avg latency)
	if stats[0].Server != "1.1.1.1" {
		t.Errorf("Expected 1.1.1.1 to be ranked first, got %s", stats[0].Server)
	}
}

func TestCalculateStatsAllErrors(t *testing.T) {
	results := []benchmark.Result{
		{Server: "bad.server", Domain: "google.com", Duration: 0, Error: os.ErrNotExist},
		{Server: "bad.server", Domain: "yahoo.com", Duration: 0, Error: os.ErrNotExist},
	}

	stats := calculateStats(results)

	if len(stats) != 1 {
		t.Errorf("Expected 1 server in stats, got %d", len(stats))
	}

	if stats[0].Success != 0 {
		t.Errorf("Expected 0 successful queries, got %d", stats[0].Success)
	}

	if stats[0].LossPct != 100.0 {
		t.Errorf("Expected 100%% loss, got %.2f%%", stats[0].LossPct)
	}

	if stats[0].Min != 0 {
		t.Errorf("Expected min to be 0 when all errors, got %v", stats[0].Min)
	}
}

func TestReadLines(t *testing.T) {
	// Create a temp file
	tmpfile, err := os.CreateTemp("", "test-domains-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "google.com\nyahoo.com\n\nexample.com\n  "
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	lines, err := readLines(tmpfile.Name())
	if err != nil {
		t.Fatalf("readLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %v", len(lines), lines)
	}

	expected := []string{"google.com", "yahoo.com", "example.com"}
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestReadCSV(t *testing.T) {
	// Create a temp CSV file with header
	tmpfile, err := os.CreateTemp("", "test-domains-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "rank,domain,traffic\n1,google.com,high\n2,yahoo.com,medium\n"
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	domains, err := readCSV(tmpfile.Name())
	if err != nil {
		t.Fatalf("readCSV failed: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d: %v", len(domains), domains)
	}

	if domains[0] != "google.com" {
		t.Errorf("Expected first domain to be google.com, got %s", domains[0])
	}
}

func TestReadCSVNoHeader(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-domains-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "google.com\nyahoo.com\n"
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	domains, err := readCSV(tmpfile.Name())
	if err != nil {
		t.Fatalf("readCSV failed: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}
}

func TestReadServersYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-servers-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := `servers:
  - 8.8.8.8
  - tls://1.1.1.1
  - https://dns.google/dns-query
`
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	servers, err := readServers(tmpfile.Name())
	if err != nil {
		t.Fatalf("readServers failed: %v", err)
	}

	if len(servers) != 3 {
		t.Errorf("Expected 3 servers, got %d: %v", len(servers), servers)
	}

	if servers[0] != "8.8.8.8" {
		t.Errorf("Expected first server to be 8.8.8.8, got %s", servers[0])
	}
}

func TestReadServersTXT(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-servers-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := "8.8.8.8\n1.1.1.1\n"
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	servers, err := readServers(tmpfile.Name())
	if err != nil {
		t.Fatalf("readServers failed: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}
}

func TestExportCSV(t *testing.T) {
	results := []benchmark.Result{
		{Server: "8.8.8.8", Domain: "google.com", Duration: 10 * time.Millisecond, Error: nil},
		{Server: "8.8.8.8", Domain: "yahoo.com", Duration: 20 * time.Millisecond, Error: nil},
	}

	tmpfile := filepath.Join(os.TempDir(), "test-export.csv")
	defer os.Remove(tmpfile)

	err := exportCSV(results, tmpfile)
	if err != nil {
		t.Fatalf("exportCSV failed: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("Failed to read exported CSV: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Server") {
		t.Error("Expected CSV to contain header 'Server'")
	}
	if !strings.Contains(contentStr, "8.8.8.8") {
		t.Error("Expected CSV to contain server '8.8.8.8'")
	}
	if !strings.Contains(contentStr, "google.com") {
		t.Error("Expected CSV to contain domain 'google.com'")
	}
}

func TestGenerateHTML(t *testing.T) {
	stats := []*ServerStats{
		{
			Server:  "8.8.8.8",
			Total:   10,
			Success: 9,
			Errors:  1,
			Min:     5 * time.Millisecond,
			Max:     50 * time.Millisecond,
			Avg:     15 * time.Millisecond,
			LossPct: 10.0,
		},
	}

	tmpfile := filepath.Join(os.TempDir(), "test-report.html")
	defer os.Remove(tmpfile)

	err := generateHTML(stats, 5*time.Second, tmpfile)
	if err != nil {
		t.Fatalf("generateHTML failed: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(tmpfile)
	if err != nil {
		t.Fatalf("Failed to read generated HTML: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "<!DOCTYPE html>") {
		t.Error("Expected HTML to be valid HTML5")
	}
	if !strings.Contains(contentStr, "8.8.8.8") {
		t.Error("Expected HTML to contain server '8.8.8.8'")
	}
	if !strings.Contains(contentStr, "DNS Benchmark") {
		t.Error("Expected HTML to contain title")
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
servers:
  - 8.8.8.8
  - 1.1.1.1
domains:
  - google.com
concurrency: 100
iterations: 5
timeout: 2s
verbose: true
progress: true
`
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := loadConfigFile(configFile)
	if err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}

	if len(cfg.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(cfg.Servers))
	}

	if cfg.Concurrency != 100 {
		t.Errorf("Expected concurrency 100, got %d", cfg.Concurrency)
	}

	if cfg.Iterations != 5 {
		t.Errorf("Expected iterations 5, got %d", cfg.Iterations)
	}

	if !cfg.Verbose {
		t.Error("Expected verbose to be true")
	}
}

func TestLoadConfigFileInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML
	content := "invalid: yaml: content: ["
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	_, err := loadConfigFile(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := loadConfigFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestFindConfigFile(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// Create temp directory with config file
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// No config file should return empty string
	result := findConfigFile()
	if result != "" {
		t.Errorf("Expected empty string when no config exists, got %s", result)
	}

	// Create config file
	configFile := ".dns-bench.yaml"
	if err := os.WriteFile(configFile, []byte("servers: []"), 0600); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	result = findConfigFile()
	if result != configFile {
		t.Errorf("Expected to find %s, got %s", configFile, result)
	}
}

func TestReadDomainsCSV(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "domains.csv")

	content := "domain\nexample.com\ntest.com\n"
	if err := os.WriteFile(csvFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	domains, err := readDomains(csvFile)
	if err != nil {
		t.Fatalf("readDomains failed: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}
}

func TestReadDomainsTXT(t *testing.T) {
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "domains.txt")

	content := "example.com\ntest.com\n"
	if err := os.WriteFile(txtFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create TXT file: %v", err)
	}

	domains, err := readDomains(txtFile)
	if err != nil {
		t.Fatalf("readDomains failed: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(domains))
	}
}

func TestPrintTable(_ *testing.T) {
	// This function writes to stdout, so we just ensure it doesn't panic
	stats := []*ServerStats{
		{
			Server:  "8.8.8.8",
			Total:   10,
			Success: 9,
			Errors:  1,
			Min:     5 * time.Millisecond,
			Max:     50 * time.Millisecond,
			Avg:     15 * time.Millisecond,
			LossPct: 10.0,
		},
	}

	// Should not panic
	printTable(stats, 5*time.Second)
}

func TestReadServersInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "servers.yaml")

	// Invalid YAML structure
	content := "invalid: [yaml"
	if err := os.WriteFile(yamlFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create YAML file: %v", err)
	}

	_, err := readServers(yamlFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}
