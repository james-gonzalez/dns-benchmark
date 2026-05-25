// Package dashboard generates the index.html dashboard from history.csv.
package dashboard

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed template.html
var dashboardTemplate string

// RunEntry represents a single benchmark run (one report file).
type RunEntry struct {
	Timestamp  string
	ReportFile string
	CSVFile    string
}

// ServerStat holds aggregated latency data for one server across all runs.
type ServerStat struct {
	Server string
	Avg    float64
	P95    float64
	P99    float64
}

// SourceStats holds per-source server stats for the filter tabs.
type SourceStats struct {
	Source       string
	PublicStats  []ServerStat
	PrivateStats []ServerStat
}

// TemplateData is passed to the HTML template.
type TemplateData struct {
	GeneratedAt    string
	PublicStats    []ServerStat
	PrivateStats   []ServerStat
	Sources        []SourceStats
	RecentRuns     []RunEntry
	ArchivedMonths []MonthGroup
}

// MonthGroup groups runs by calendar month for the archive section.
type MonthGroup struct {
	Label string
	Runs  []RunEntry
}

// isPrivate returns true for RFC-1918 / loopback addresses.
func isPrivate(server string) bool {
	plain := strings.TrimPrefix(strings.TrimPrefix(server, "tls://"), "https://")
	plain = strings.SplitN(plain, "/", 2)[0] // strip path
	plain = strings.SplitN(plain, ":", 2)[0] // strip port
	return strings.HasPrefix(plain, "192.168.") ||
		strings.HasPrefix(plain, "10.") ||
		strings.HasPrefix(plain, "127.") ||
		plain == "localhost" ||
		isRFC1918_172(plain)
}

func isRFC1918_172(ip string) bool {
	if !strings.HasPrefix(ip, "172.") {
		return false
	}
	parts := strings.SplitN(ip, ".", 4)
	if len(parts) < 2 {
		return false
	}
	second, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return second >= 16 && second <= 31
}

// Generate reads history.csv from resultsDir and writes index.html to resultsDir.
func Generate(resultsDir string) error {
	historyPath := filepath.Join(resultsDir, "history.csv")

	pubDurs := map[string][]float64{}
	privDurs := map[string][]float64{}
	// per-source: source -> server -> durations
	srcPubDurs := map[string]map[string][]float64{}
	srcPrivDurs := map[string]map[string][]float64{}

	f, err := os.Open(historyPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("opening history.csv: %w", err)
	}
	if err == nil {
		defer func() {
			if cerr := f.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "warning: closing history.csv: %v\n", cerr)
			}
		}()
		if err := parseHistory(f, pubDurs, privDurs, srcPubDurs, srcPrivDurs); err != nil {
			return fmt.Errorf("parsing history.csv: %w", err)
		}
	}

	publicStats := buildStats(pubDurs)
	privateStats := buildStats(privDurs)

	// Build per-source stats, sorted by source name.
	sourceNames := make([]string, 0, len(srcPubDurs))
	seen := map[string]bool{}
	for s := range srcPubDurs {
		if !seen[s] {
			seen[s] = true
			sourceNames = append(sourceNames, s)
		}
	}
	for s := range srcPrivDurs {
		if !seen[s] {
			seen[s] = true
			sourceNames = append(sourceNames, s)
		}
	}
	sort.Strings(sourceNames)

	sources := make([]SourceStats, 0, len(sourceNames))
	for _, src := range sourceNames {
		sources = append(sources, SourceStats{
			Source:       src,
			PublicStats:  buildStats(srcPubDurs[src]),
			PrivateStats: buildStats(srcPrivDurs[src]),
		})
	}

	recent, archived, err := collectRuns(resultsDir)
	if err != nil {
		return fmt.Errorf("collecting run files: %w", err)
	}

	data := TemplateData{
		GeneratedAt:    time.Now().UTC().Format("02 Jan 2006, 15:04 UTC"),
		PublicStats:    publicStats,
		PrivateStats:   privateStats,
		Sources:        sources,
		RecentRuns:     recent,
		ArchivedMonths: archived,
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tmpl, err := template.New("dashboard").Funcs(funcMap).Parse(dashboardTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	out, err := os.Create(filepath.Join(resultsDir, "index.html"))
	if err != nil {
		return fmt.Errorf("creating index.html: %w", err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing index.html: %v\n", cerr)
		}
	}()

	return tmpl.Execute(out, data)
}

func parseHistory(r io.Reader,
	pubDurs map[string][]float64, privDurs map[string][]float64,
	srcPubDurs map[string]map[string][]float64, srcPrivDurs map[string]map[string][]float64) error {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate variable columns

	// skip header
	if _, err := cr.Read(); err != nil {
		return nil // empty file is fine
	}

	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// columns: Timestamp, Server, Domain, Duration_ms, Error[, Source]
		if len(rec) < 4 {
			continue
		}
		if len(rec) >= 5 && strings.TrimSpace(rec[4]) != "" {
			continue
		}
		server := rec[1]
		dur, err := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
		if err != nil || dur <= 0 {
			continue
		}
		source := ""
		if len(rec) >= 6 {
			source = strings.TrimSpace(rec[5])
		}

		if isPrivate(server) {
			privDurs[server] = append(privDurs[server], dur)
			if source != "" {
				if srcPrivDurs[source] == nil {
					srcPrivDurs[source] = map[string][]float64{}
				}
				srcPrivDurs[source][server] = append(srcPrivDurs[source][server], dur)
			}
		} else {
			pubDurs[server] = append(pubDurs[server], dur)
			if source != "" {
				if srcPubDurs[source] == nil {
					srcPubDurs[source] = map[string][]float64{}
				}
				srcPubDurs[source][server] = append(srcPubDurs[source][server], dur)
			}
		}
	}
	return nil
}

func percentileFloat(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func buildStats(durs map[string][]float64) []ServerStat {
	stats := make([]ServerStat, 0, len(durs))
	for server, d := range durs {
		if len(d) == 0 {
			continue
		}
		sorted := make([]float64, len(d))
		copy(sorted, d)
		sort.Float64s(sorted)
		var sum float64
		for _, v := range sorted {
			sum += v
		}
		stats = append(stats, ServerStat{
			Server: server,
			Avg:    sum / float64(len(sorted)),
			P95:    percentileFloat(sorted, 95),
			P99:    percentileFloat(sorted, 99),
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Avg != stats[j].Avg {
			return stats[i].Avg < stats[j].Avg
		}
		return stats[i].Server < stats[j].Server
	})
	return stats
}

// collectRuns scans resultsDir for report-*.html files and returns recent + archived groups.
func collectRuns(resultsDir string) (recent []RunEntry, archived []MonthGroup, err error) {
	pattern := filepath.Join(resultsDir, "report-*.html")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, err
	}

	// Sort descending (newest first) — filenames are ISO timestamps so lexicographic works.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))

	const recentCount = 10
	monthMap := map[string]*MonthGroup{}
	var monthOrder []string

	for i, path := range matches {
		fname := filepath.Base(path)
		ts := strings.TrimSuffix(strings.TrimPrefix(fname, "report-"), ".html")
		entry := RunEntry{
			Timestamp:  ts,
			ReportFile: fname,
			CSVFile:    "results-" + ts + ".csv",
		}
		if i < recentCount {
			recent = append(recent, entry)
		} else {
			month := ""
			if len(ts) >= 7 {
				month = ts[:7] // "2026-04"
			}
			if _, ok := monthMap[month]; !ok {
				monthMap[month] = &MonthGroup{Label: month}
				monthOrder = append(monthOrder, month)
			}
			monthMap[month].Runs = append(monthMap[month].Runs, entry)
		}
	}

	for _, m := range monthOrder {
		archived = append(archived, *monthMap[m])
	}
	return recent, archived, nil
}
