package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsPrivate(t *testing.T) {
	tests := []struct {
		server string
		want   bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"127.0.0.1", true},
		{"localhost", true},
		{"172.16.0.1", true},
		{"172.31.255.254", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"tls://192.168.1.1:853", true},
		{"https://10.0.0.1/dns-query", true},
	}

	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			got := isPrivate(tt.server)
			if got != tt.want {
				t.Errorf("isPrivate(%q) = %v, want %v", tt.server, got, tt.want)
			}
		})
	}
}

func TestIsRFC1918_172(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"172.16.0.1", true},
		{"172.20.0.1", true},
		{"172.31.255.254", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"172", false},
		{"172.invalid.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isRFC1918_172(tt.ip)
			if got != tt.want {
				t.Errorf("isRFC1918_172(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestBuildStats(t *testing.T) {
	sums := map[string]float64{
		"8.8.8.8":   100.0,
		"1.1.1.1":   50.0,
		"9.9.9.9":   75.0,
		"zero-runs": 0.0,
	}
	counts := map[string]int{
		"8.8.8.8":   2,
		"1.1.1.1":   2,
		"9.9.9.9":   3,
		"zero-runs": 0,
	}

	stats := buildStats(sums, counts)

	if len(stats) != 3 {
		t.Errorf("expected 3 stats, got %d", len(stats))
	}

	// Should be sorted by average (ascending)
	if stats[0].Server != "9.9.9.9" {
		t.Errorf("expected first server to be 9.9.9.9, got %s", stats[0].Server)
	}
	if stats[0].Avg != 25.0 {
		t.Errorf("expected avg 25.0, got %f", stats[0].Avg)
	}

	if stats[1].Server != "1.1.1.1" {
		t.Errorf("expected second server to be 1.1.1.1, got %s", stats[1].Server)
	}
	if stats[1].Avg != 25.0 {
		t.Errorf("expected avg 25.0, got %f", stats[1].Avg)
	}

	if stats[2].Server != "8.8.8.8" {
		t.Errorf("expected third server to be 8.8.8.8, got %s", stats[2].Server)
	}
	if stats[2].Avg != 50.0 {
		t.Errorf("expected avg 50.0, got %f", stats[2].Avg)
	}
}

func TestParseHistory(t *testing.T) {
	csvData := `Timestamp,Server,Domain,Duration_ms,Error,Source
2026-05-01T10:00:00Z,8.8.8.8,example.com,25.5,,raspi
2026-05-01T10:00:01Z,8.8.8.8,example.org,30.5,,raspi
2026-05-01T10:00:02Z,1.1.1.1,example.com,20.0,,dietpi
2026-05-01T10:00:03Z,192.168.1.1,example.com,15.0,,k3s-pod
2026-05-01T10:00:04Z,8.8.8.8,example.com,0.0,,raspi
2026-05-01T10:00:05Z,1.1.1.1,example.com,-5.0,,dietpi
2026-05-01T10:00:06Z,9.9.9.9,example.com,40.0,timeout,raspi
`

	pubSums := map[string]float64{}
	pubCounts := map[string]int{}
	privSums := map[string]float64{}
	privCounts := map[string]int{}
	srcPubSums := map[string]map[string]float64{}
	srcPubCounts := map[string]map[string]int{}
	srcPrivSums := map[string]map[string]float64{}
	srcPrivCounts := map[string]map[string]int{}

	r := strings.NewReader(csvData)
	err := parseHistory(r, pubSums, pubCounts, privSums, privCounts,
		srcPubSums, srcPubCounts, srcPrivSums, srcPrivCounts)
	if err != nil {
		t.Fatalf("parseHistory failed: %v", err)
	}

	// Check public servers
	if pubSums["8.8.8.8"] != 56.0 {
		t.Errorf("expected 8.8.8.8 sum 56.0, got %f", pubSums["8.8.8.8"])
	}
	if pubCounts["8.8.8.8"] != 2 {
		t.Errorf("expected 8.8.8.8 count 2, got %d", pubCounts["8.8.8.8"])
	}
	if pubSums["1.1.1.1"] != 20.0 {
		t.Errorf("expected 1.1.1.1 sum 20.0, got %f", pubSums["1.1.1.1"])
	}
	if pubCounts["1.1.1.1"] != 1 {
		t.Errorf("expected 1.1.1.1 count 1, got %d", pubCounts["1.1.1.1"])
	}

	// Check private servers
	if privSums["192.168.1.1"] != 15.0 {
		t.Errorf("expected 192.168.1.1 sum 15.0, got %f", privSums["192.168.1.1"])
	}
	if privCounts["192.168.1.1"] != 1 {
		t.Errorf("expected 192.168.1.1 count 1, got %d", privCounts["192.168.1.1"])
	}

	// Check per-source stats
	if srcPubSums["raspi"]["8.8.8.8"] != 56.0 {
		t.Errorf("expected raspi 8.8.8.8 sum 56.0, got %f", srcPubSums["raspi"]["8.8.8.8"])
	}
	if srcPubCounts["raspi"]["8.8.8.8"] != 2 {
		t.Errorf("expected raspi 8.8.8.8 count 2, got %d", srcPubCounts["raspi"]["8.8.8.8"])
	}
	if srcPubSums["dietpi"]["1.1.1.1"] != 20.0 {
		t.Errorf("expected dietpi 1.1.1.1 sum 20.0, got %f", srcPubSums["dietpi"]["1.1.1.1"])
	}
	if srcPrivSums["k3s-pod"]["192.168.1.1"] != 15.0 {
		t.Errorf("expected k3s-pod 192.168.1.1 sum 15.0, got %f", srcPrivSums["k3s-pod"]["192.168.1.1"])
	}
}

func TestParseHistoryEmptyFile(t *testing.T) {
	pubSums := map[string]float64{}
	pubCounts := map[string]int{}
	privSums := map[string]float64{}
	privCounts := map[string]int{}
	srcPubSums := map[string]map[string]float64{}
	srcPubCounts := map[string]map[string]int{}
	srcPrivSums := map[string]map[string]float64{}
	srcPrivCounts := map[string]map[string]int{}

	r := strings.NewReader("")
	err := parseHistory(r, pubSums, pubCounts, privSums, privCounts,
		srcPubSums, srcPubCounts, srcPrivSums, srcPrivCounts)
	if err != nil {
		t.Fatalf("parseHistory on empty file should not error: %v", err)
	}

	if len(pubSums) != 0 {
		t.Errorf("expected empty pubSums, got %d entries", len(pubSums))
	}
}

func TestParseHistoryWithoutSource(t *testing.T) {
	csvData := `Timestamp,Server,Domain,Duration_ms,Error
2026-05-01T10:00:00Z,8.8.8.8,example.com,25.5,
`

	pubSums := map[string]float64{}
	pubCounts := map[string]int{}
	privSums := map[string]float64{}
	privCounts := map[string]int{}
	srcPubSums := map[string]map[string]float64{}
	srcPubCounts := map[string]map[string]int{}
	srcPrivSums := map[string]map[string]float64{}
	srcPrivCounts := map[string]map[string]int{}

	r := strings.NewReader(csvData)
	err := parseHistory(r, pubSums, pubCounts, privSums, privCounts,
		srcPubSums, srcPubCounts, srcPrivSums, srcPrivCounts)
	if err != nil {
		t.Fatalf("parseHistory failed: %v", err)
	}

	if pubSums["8.8.8.8"] != 25.5 {
		t.Errorf("expected 8.8.8.8 sum 25.5, got %f", pubSums["8.8.8.8"])
	}
	if len(srcPubSums) != 0 {
		t.Errorf("expected no per-source stats without source column, got %d", len(srcPubSums))
	}
}

func TestCollectRuns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some report files
	reports := []string{
		"report-2026-05-12T10:00:00Z.html",
		"report-2026-05-11T10:00:00Z.html",
		"report-2026-05-10T10:00:00Z.html",
		"report-2026-04-30T10:00:00Z.html",
		"report-2026-04-29T10:00:00Z.html",
		"report-2026-03-15T10:00:00Z.html",
	}

	for _, name := range reports {
		f, err := os.Create(filepath.Join(tmpDir, name))
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		f.Close()
	}

	recent, archived, err := collectRuns(tmpDir)
	if err != nil {
		t.Fatalf("collectRuns failed: %v", err)
	}

	if len(recent) != 6 {
		t.Errorf("expected 6 recent runs, got %d", len(recent))
	}

	// Check that recent runs are sorted newest first
	if len(recent) > 0 && recent[0].Timestamp != "2026-05-12T10:00:00Z" {
		t.Errorf("expected first recent run to be 2026-05-12T10:00:00Z, got %s", recent[0].Timestamp)
	}

	// With only 6 files, there should be no archived
	if len(archived) != 0 {
		t.Errorf("expected 0 archived months with 6 files, got %d", len(archived))
	}

	// Create more files to trigger archiving
	for i := 1; i <= 20; i++ {
		name := filepath.Join(tmpDir, "report-2026-03-01T10:00:00Z.html")
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		f.Close()
	}

	recent, _, err = collectRuns(tmpDir)
	if err != nil {
		t.Fatalf("collectRuns failed: %v", err)
	}

	if len(recent) > 10 {
		t.Errorf("expected at most 10 recent runs, got %d", len(recent))
	}
}

func TestCollectRunsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	recent, archived, err := collectRuns(tmpDir)
	if err != nil {
		t.Fatalf("collectRuns on empty dir should not error: %v", err)
	}

	if len(recent) != 0 {
		t.Errorf("expected 0 recent runs, got %d", len(recent))
	}
	if len(archived) != 0 {
		t.Errorf("expected 0 archived runs, got %d", len(archived))
	}
}

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a history.csv file
	historyData := `Timestamp,Server,Domain,Duration_ms,Error,Source
2026-05-01T10:00:00Z,8.8.8.8,example.com,25.5,,raspi
2026-05-01T10:00:01Z,1.1.1.1,example.com,20.0,,dietpi
2026-05-01T10:00:02Z,192.168.1.1,example.com,15.0,,k3s-pod
`
	historyPath := filepath.Join(tmpDir, "history.csv")
	if err := os.WriteFile(historyPath, []byte(historyData), 0o644); err != nil {
		t.Fatalf("failed to write history.csv: %v", err)
	}

	// Create a report file
	reportPath := filepath.Join(tmpDir, "report-2026-05-01T10:00:00Z.html")
	if err := os.WriteFile(reportPath, []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("failed to write report: %v", err)
	}

	err := Generate(tmpDir)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that index.html was created
	indexPath := filepath.Join(tmpDir, "index.html")
	_, err = os.Stat(indexPath)
	if err != nil {
		t.Errorf("index.html was not created: %v", err)
	}

	// Check that index.html contains expected content
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "8.8.8.8") {
		t.Error("index.html should contain 8.8.8.8")
	}
	if !strings.Contains(contentStr, "1.1.1.1") {
		t.Error("index.html should contain 1.1.1.1")
	}
	if !strings.Contains(contentStr, "192.168.1.1") {
		t.Error("index.html should contain 192.168.1.1")
	}
}

func TestGenerateNoHistory(t *testing.T) {
	tmpDir := t.TempDir()

	err := Generate(tmpDir)
	if err != nil {
		t.Fatalf("Generate should not fail with no history.csv: %v", err)
	}

	// Check that index.html was created
	indexPath := filepath.Join(tmpDir, "index.html")
	_, err = os.Stat(indexPath)
	if err != nil {
		t.Errorf("index.html was not created: %v", err)
	}
}
