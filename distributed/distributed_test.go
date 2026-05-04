package distributed

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dns-bench/benchmark"
)

func TestAggregateResults(t *testing.T) {
	input := map[string][]benchmark.Result{
		"host-a": {
			{Server: "1.1.1.1", Domain: "example.com", Duration: 10 * time.Millisecond},
			{Server: "1.1.1.1", Domain: "example.com", Duration: 20 * time.Millisecond},
			{Server: "9.9.9.9", Domain: "example.org", Duration: 15 * time.Millisecond, Error: os.ErrDeadlineExceeded},
		},
		"host-b": {
			{Server: "1.1.1.1", Domain: "example.com", Duration: 30 * time.Millisecond},
			{Server: "8.8.8.8", Domain: "example.net", Duration: 5 * time.Millisecond},
		},
	}

	got := AggregateResults(input)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	if got[0].Server != "1.1.1.1" || got[0].Domain != "example.com" {
		t.Fatalf("first result = %+v", got[0])
	}
	if got[0].AverageMs != 20 || got[0].MinMs != 10 || got[0].MaxMs != 30 {
		t.Fatalf("stats = %+v, want avg/min/max 20/10/30", got[0])
	}
	if got[1].Server != "8.8.8.8" || got[1].Domain != "example.net" {
		t.Fatalf("second result = %+v", got[1])
	}
	if got[1].StdDev != 0 {
		t.Fatalf("stddev = %v, want 0", got[1].StdDev)
	}
}

func TestExportAggregatedCSV(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "aggregated.csv")
	results := []AggregatedResult{{
		Server:    "1.1.1.1",
		Domain:    "example.com",
		AverageMs: 12.3456,
		MinMs:     10,
		MaxMs:     15,
		StdDev:    2.5,
	}}

	if err := ExportAggregatedCSV(results, filename); err != nil {
		t.Fatalf("ExportAggregatedCSV: %v", err)
	}

	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	wantHeader := []string{"timestamp", "server", "domain", "avg_ms", "min_ms", "max_ms", "stddev_ms"}
	for i, want := range wantHeader {
		if recs[0][i] != want {
			t.Fatalf("header[%d] = %q, want %q", i, recs[0][i], want)
		}
	}
	if recs[1][1] != "1.1.1.1" || recs[1][2] != "example.com" || recs[1][3] != "12.346" {
		t.Fatalf("row = %v", recs[1])
	}
}

func TestBuildRemoteCommand(t *testing.T) {
	config := benchmark.Config{
		Servers:     []string{"8.8.8.8", "1.1.1.1"},
		Domains:     []string{"google.com", "example.com"},
		Iterations:  3,
		Concurrency: 10,
		Timeout:     500 * time.Millisecond,
	}

	cmd := buildRemoteCommand("/usr/local/bin/dns-bench", config)

	// Must create temp files
	if !strings.Contains(cmd, "mktemp") {
		t.Error("command should create temp files via mktemp")
	}
	// Must include -json flag
	if !strings.Contains(cmd, "-json") {
		t.Error("command should include -json flag")
	}
	// Must include binary path
	if !strings.Contains(cmd, "/usr/local/bin/dns-bench") {
		t.Error("command should include binary path")
	}
	// Must include iteration count
	if !strings.Contains(cmd, "-n 3") {
		t.Errorf("command should include -n 3, got: %s", cmd)
	}
	// Must include concurrency
	if !strings.Contains(cmd, "-c 10") {
		t.Errorf("command should include -c 10, got: %s", cmd)
	}
	// Must include timeout
	if !strings.Contains(cmd, "-t 500ms") {
		t.Errorf("command should include -t 500ms, got: %s", cmd)
	}
	// Must clean up temp files
	if !strings.Contains(cmd, "rm -f") {
		t.Error("command should clean up temp files")
	}
	// Must include server values
	for _, s := range config.Servers {
		if !strings.Contains(cmd, s) {
			t.Errorf("command should include server %q", s)
		}
	}
	// Must include domain values
	for _, d := range config.Domains {
		if !strings.Contains(cmd, d) {
			t.Errorf("command should include domain %q", d)
		}
	}
}

func TestBuildRemoteCommandEscaping(t *testing.T) {
	// Ensure single quotes in server/domain names are escaped
	config := benchmark.Config{
		Servers:     []string{"it's.a.server"},
		Domains:     []string{"it's.a.domain"},
		Iterations:  1,
		Concurrency: 1,
		Timeout:     time.Second,
	}
	cmd := buildRemoteCommand("/bin/dns-bench", config)
	// Should not contain unescaped single quotes that would break the shell command
	// The escaped form is: 'it'\''s.a.server'
	if strings.Contains(cmd, "'it's") {
		t.Error("single quotes in server names should be escaped")
	}
}

func TestRunDistributedAllHostsFail(t *testing.T) {
	hosts := []HostConfig{
		{Name: "bad-host-1", Host: "192.0.2.1", Port: 22, User: "test", BinaryPath: "/bin/dns-bench"},
		{Name: "bad-host-2", Host: "192.0.2.2", Port: 22, User: "test", BinaryPath: "/bin/dns-bench"},
	}
	config := benchmark.Config{
		Servers:     []string{"8.8.8.8"},
		Domains:     []string{"example.com"},
		Iterations:  1,
		Concurrency: 1,
		Timeout:     100 * time.Millisecond,
	}

	_, err := RunDistributed(hosts, config)
	if err == nil {
		t.Fatal("expected error when all hosts fail, got nil")
	}
	if !strings.Contains(err.Error(), "all hosts failed") {
		t.Errorf("error message = %q, want to contain 'all hosts failed'", err.Error())
	}
}

func TestHostConfigDefaults(t *testing.T) {
	h := HostConfig{
		Name:       "test",
		Host:       "192.168.1.1",
		User:       "pi",
		BinaryPath: "/usr/local/bin/dns-bench",
		// Port and KeyPath intentionally omitted to test defaults
	}
	if h.Port != 0 {
		t.Errorf("default Port = %d, want 0 (handled in runOnHost)", h.Port)
	}
	if h.KeyPath != "" {
		t.Errorf("default KeyPath = %q, want empty (handled in runOnHost)", h.KeyPath)
	}
}
