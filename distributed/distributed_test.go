package distributed

import (
	"encoding/csv"
	"os"
	"path/filepath"
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
