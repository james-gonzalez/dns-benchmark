package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"dns-bench/benchmark"
	"dns-bench/browser"

	"gopkg.in/yaml.v3"
)

var (
	defaultServers = []string{
		"8.8.8.8",                      // Google (UDP)
		"1.1.1.1",                      // Cloudflare (UDP)
		"tls://1.1.1.1",                // Cloudflare (DoT)
		"https://dns.google/dns-query", // Google (DoH)
		"9.9.9.9",                      // Quad9 (UDP)
	}
	defaultDomains = []string{
		"google.com",
		"facebook.com",
		"amazon.com",
		"apple.com",
		"microsoft.com",
		"netflix.com",
		"twitter.com",
		"instagram.com",
		"linkedin.com",
		"wikipedia.org",
	}
)

func main() {
	var (
		concurrency int
		iterations  int
		timeout     time.Duration
		duration    time.Duration
		domainFile  string
		serverFile  string
		exportFile  string
		htmlFile    string
		browserName string
		verbose     bool
	)

	flag.IntVar(&concurrency, "c", 50, "Number of concurrent queries")
	flag.IntVar(&iterations, "n", 1, "Number of iterations per domain per server")
	flag.DurationVar(&timeout, "t", 1*time.Second, "Timeout for each query")
	flag.DurationVar(&duration, "d", 0, "Duration to run benchmark (e.g. 30s). Overrides -n if set.")
	flag.StringVar(&domainFile, "domains", "", "File containing list of domains (one per line or CSV)")
	flag.StringVar(&serverFile, "servers", "", "File containing list of servers (one per line or YAML)")
	flag.StringVar(&exportFile, "o", "", "Output CSV file for raw results")
	flag.StringVar(&htmlFile, "html", "", "Output HTML report file")
	flag.StringVar(&browserName, "browser", "", "Import domains from browser history (chrome, brave, safari, firefox)")
	flag.BoolVar(&verbose, "v", false, "Verbose logging (show errors and slow queries)")
	flag.Parse()

	servers := defaultServers
	if serverFile != "" {
		var err error
		servers, err = readServers(serverFile)
		if err != nil {
			fmt.Printf("Error reading server file: %v\n", err)
			os.Exit(1)
		}
	}

	domains := defaultDomains
	if domainFile != "" {
		var err error
		domains, err = readDomains(domainFile)
		if err != nil {
			fmt.Printf("Error reading domain file: %v\n", err)
			os.Exit(1)
		}
	} else if browserName != "" {
		fmt.Printf("Extracting domains from %s history...\n", browserName)
		var err error
		domains, err = browser.GetDomains(browserName, 1000) // Limit to 1000 most recent/frequent
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				fmt.Printf("\n⚠️  PERMISSION DENIED: macOS prevented access to %s history.\n", browserName)
				fmt.Printf("To fix this:\n")
				fmt.Printf("1. Open System Settings -> Privacy & Security -> Full Disk Access\n")
				fmt.Printf("2. Grant access to your terminal app (e.g., Terminal, iTerm2, VSCode)\n")
				fmt.Printf("3. Restart the terminal and try again.\n\n")
				os.Exit(1)
			}
			fmt.Printf("Error extracting browser history: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d unique domains from %s\n", len(domains), browserName)
	}

	fmt.Printf("Starting benchmark...\n")
	if duration > 0 {
		fmt.Printf("Servers: %d, Domains: %d, Duration: %v, Concurrency: %d\n", len(servers), len(domains), duration, concurrency)
	} else {
		fmt.Printf("Servers: %d, Domains: %d, Iterations: %d, Concurrency: %d\n", len(servers), len(domains), iterations, concurrency)
	}

	config := benchmark.BenchmarkConfig{
		Servers:     servers,
		Domains:     domains,
		Iterations:  iterations,
		Concurrency: concurrency,
		Timeout:     timeout,
		Duration:    duration,
		Verbose:     verbose,
	}

	start := time.Now()
	results := benchmark.Run(config)
	totalTime := time.Since(start)

	stats := calculateStats(results)
	printTable(stats, totalTime)

	if exportFile != "" {
		if err := exportCSV(results, exportFile); err != nil {
			fmt.Printf("Error exporting results: %v\n", err)
		} else {
			fmt.Printf("Results exported to %s\n", exportFile)
		}
	}

	if htmlFile != "" {
		if err := generateHTML(stats, totalTime, htmlFile); err != nil {
			fmt.Printf("Error generating HTML report: %v\n", err)
		} else {
			fmt.Printf("HTML report generated at %s\n", htmlFile)
		}
	}
}

type ServerStats struct {
	Server    string
	Total     int
	Success   int
	Errors    int
	Min       time.Duration
	Max       time.Duration
	TotalTime time.Duration
	Avg       time.Duration // Pre-calculated for reports
	LossPct   float64       // Pre-calculated for reports
}

func calculateStats(results []benchmark.Result) []*ServerStats {
	statsMap := make(map[string]*ServerStats)

	for _, res := range results {
		s, ok := statsMap[res.Server]
		if !ok {
			s = &ServerStats{Server: res.Server, Min: time.Hour} // Init min high
			statsMap[res.Server] = s
		}
		s.Total++
		if res.Error != nil {
			s.Errors++
		} else {
			s.Success++
			s.TotalTime += res.Duration
			if res.Duration < s.Min {
				s.Min = res.Duration
			}
			if res.Duration > s.Max {
				s.Max = res.Duration
			}
		}
	}

	var sortedStats []*ServerStats
	for _, s := range statsMap {
		if s.Success > 0 {
			s.Avg = s.TotalTime / time.Duration(s.Success)
		}
		s.LossPct = float64(s.Errors) / float64(s.Total) * 100
		if s.Success == 0 {
			s.Min = 0
		}
		sortedStats = append(sortedStats, s)
	}

	sort.Slice(sortedStats, func(i, j int) bool {
		// Prefer success over failure
		if sortedStats[i].Success > 0 && sortedStats[j].Success == 0 {
			return true
		}
		if sortedStats[i].Success == 0 && sortedStats[j].Success > 0 {
			return false
		}
		// Then sort by Avg latency
		return sortedStats[i].Avg < sortedStats[j].Avg
	})

	return sortedStats
}

func printTable(stats []*ServerStats, totalTime time.Duration) {
	fmt.Printf("\nBenchmark Complete in %v\n\n", totalTime)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "RANK\tSERVER\tAVG LATENCY\tMIN\tMAX\tLOSS %")

	for i, s := range stats {
		fmt.Fprintf(w, "%d\t%s\t%v\t%v\t%v\t%.2f%%\n", i+1, s.Server, s.Avg, s.Min, s.Max, s.LossPct)
	}
	w.Flush()
}

// ServerConfigYAML matches the expected YAML structure
type ServerConfigYAML struct {
	Servers []string `yaml:"servers"`
}

func readServers(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var config ServerConfigYAML
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %v", err)
		}
		return config.Servers, nil
	}

	// Fallback to reading lines (txt)
	return readLines(path)
}

func readDomains(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".csv" {
		return readCSV(path)
	}
	return readLines(path)
}

func readCSV(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var domains []string
	if len(records) == 0 {
		return domains, nil
	}

	colIdx := 0
	// Check for header
	hasHeader := false
	for i, field := range records[0] {
		if strings.ToLower(strings.TrimSpace(field)) == "domain" {
			colIdx = i
			hasHeader = true
			break
		}
	}

	startRow := 0
	if hasHeader {
		startRow = 1
	}

	for i := startRow; i < len(records); i++ {
		record := records[i]
		if len(record) > colIdx {
			domain := strings.TrimSpace(record[colIdx])
			if domain != "" {
				domains = append(domains, domain)
			}
		}
	}
	return domains, nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
		}
	}
	return lines, scanner.Err()
}

func exportCSV(results []benchmark.Result, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	if err := writer.Write([]string{"Server", "Domain", "Duration_ms", "Error"}); err != nil {
		return err
	}

	for _, res := range results {
		errStr := ""
		if res.Error != nil {
			errStr = res.Error.Error()
		}
		record := []string{
			res.Server,
			res.Domain,
			strconv.FormatFloat(float64(res.Duration.Microseconds())/1000.0, 'f', 4, 64),
			errStr,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}

const htmlReportTemplate = `
<!DOCTYPE html>
<html>
<head>
	<title>DNS Benchmark Report</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 2rem; background: #f4f4f9; color: #333; }
		.container { max-width: 1000px; margin: 0 auto; background: white; padding: 2rem; border-radius: 8px; box-shadow: 0 2px 5px rgba(0,0,0,0.1); }
		h1 { margin-top: 0; color: #2c3e50; }
		.summary { margin-bottom: 2rem; padding: 1rem; background: #eef2f7; border-radius: 4px; }
		table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
		th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
		th { background-color: #2c3e50; color: white; }
		tr:nth-child(even) { background-color: #f9f9f9; }
		tr:hover { background-color: #f1f1f1; }
		.good { color: green; font-weight: bold; }
		.bad { color: red; font-weight: bold; }
		.rank { font-weight: bold; color: #555; }
	</style>
</head>
<body>
	<div class="container">
		<h1>DNS Benchmark Results</h1>
		<div class="summary">
			<strong>Total Duration:</strong> {{.TotalTime}}<br>
			<strong>Servers Tested:</strong> {{.ServerCount}}
		</div>

		<table>
			<thead>
				<tr>
					<th>Rank</th>
					<th>Server</th>
					<th>Avg Latency</th>
					<th>Min</th>
					<th>Max</th>
					<th>Loss %</th>
				</tr>
			</thead>
			<tbody>
				{{range $i, $s := .Stats}}
				<tr>
					<td class="rank">{{add $i 1}}</td>
					<td>{{$s.Server}}</td>
					<td>{{$s.Avg}}</td>
					<td>{{$s.Min}}</td>
					<td>{{$s.Max}}</td>
					<td class="{{if gt $s.LossPct 5.0}}bad{{else}}good{{end}}">{{printf "%.2f" $s.LossPct}}%</td>
				</tr>
				{{end}}
			</tbody>
		</table>
	</div>
</body>
</html>
`

func generateHTML(stats []*ServerStats, totalTime time.Duration, path string) error {
	funcMap := template.FuncMap{
		"add": func(i, j int) int { return i + j },
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(htmlReportTemplate)
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data := struct {
		Stats       []*ServerStats
		TotalTime   time.Duration
		ServerCount int
	}{
		Stats:       stats,
		TotalTime:   totalTime,
		ServerCount: len(stats),
	}

	return tmpl.Execute(file, data)
}
