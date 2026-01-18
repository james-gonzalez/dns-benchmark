package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"dns-bench/benchmark"
	"dns-bench/browser"
	"dns-bench/validation"

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
		"youtube.com",
		"facebook.com",
		"twitter.com",
		"instagram.com",
		"linkedin.com",
		"reddit.com",
		"wikipedia.org",
		"amazon.com",
		"netflix.com",
		"apple.com",
		"microsoft.com",
		"zoom.us",
		"tiktok.com",
		"whatsapp.com",
		"baidu.com",
		"yahoo.com",
		"yandex.ru",
		"github.com",
		"stackoverflow.com",
		"twitch.tv",
		"discord.com",
		"spotify.com",
		"pinterest.com",
		"ebay.com",
		"tumblr.com",
		"wordpress.com",
		"imdb.com",
		"paypal.com",
		"adobe.com",
		"salesforce.com",
		"dropbox.com",
		"cloudflare.com",
		"aliexpress.com",
		"cnn.com",
		"nytimes.com",
		"bbc.com",
		"theguardian.com",
		"espn.com",
		"washingtonpost.com",
		"forbes.com",
		"bloomberg.com",
		"reuters.com",
		"medium.com",
		"wordpress.org",
		"blogger.com",
		"craigslist.org",
		"etsy.com",
		"shopify.com",
		"wix.com",
		"squarespace.com",
		"godaddy.com",
		"vimeo.com",
		"soundcloud.com",
		"flickr.com",
		"500px.com",
		"deviantart.com",
		"quora.com",
		"yelp.com",
		"tripadvisor.com",
		"airbnb.com",
		"booking.com",
		"expedia.com",
		"hotels.com",
		"kayak.com",
		"zillow.com",
		"indeed.com",
		"glassdoor.com",
		"zendesk.com",
		"slack.com",
		"trello.com",
		"asana.com",
		"notion.so",
		"figma.com",
		"canva.com",
		"bitly.com",
		"mailchimp.com",
		"hubspot.com",
		"atlassian.com",
		"jira.com",
		"confluence.com",
		"atlassian.net",
		"bitbucket.org",
		"gitlab.com",
		"docker.com",
		"kubernetes.io",
		"aws.amazon.com",
		"cloud.google.com",
		"azure.microsoft.com",
		"digitalocean.com",
		"heroku.com",
		"oracle.com",
		"ibm.com",
		"intel.com",
		"amd.com",
		"nvidia.com",
		"hp.com",
		"dell.com",
		"lenovo.com",
		"samsung.com",
	}
)

// Config represents configuration that can be loaded from file or flags
type Config struct {
	Servers     []string      `yaml:"servers"`
	Domains     []string      `yaml:"domains"`
	Concurrency int           `yaml:"concurrency"`
	Iterations  int           `yaml:"iterations"`
	Timeout     time.Duration `yaml:"timeout"`
	Duration    time.Duration `yaml:"duration"`
	Verbose     bool          `yaml:"verbose"`
	Progress    bool          `yaml:"progress"`
	DomainFile  string        `yaml:"domain_file"`
	ServerFile  string        `yaml:"server_file"`
	ExportCSV   string        `yaml:"export_csv"`
	ExportHTML  string        `yaml:"export_html"`
	BrowserName string        `yaml:"browser"`
}

// loadConfigFile loads configuration from a YAML file
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// findConfigFile looks for config file in standard locations
func findConfigFile() string {
	locations := []string{
		".dns-bench.yaml",
		".dns-bench.yml",
	}

	// Also check home directory
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations,
			filepath.Join(home, ".dns-bench.yaml"),
			filepath.Join(home, ".dns-bench.yml"),
		)
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

//nolint:gocyclo // main() handles CLI flag parsing and orchestration; complexity is acceptable
func main() {
	var (
		configFile   string
		concurrency  int
		iterations   int
		timeout      time.Duration
		duration     time.Duration
		domainFile   string
		serverFile   string
		exportFile   string
		htmlFile     string
		browserName  string
		verbose      bool
		showProgress bool
	)

	flag.StringVar(&configFile, "config", "", "Path to config file (YAML)")
	flag.IntVar(&concurrency, "c", 0, "Number of concurrent queries")
	flag.IntVar(&iterations, "n", 0, "Number of iterations per domain per server")
	flag.DurationVar(&timeout, "t", 0, "Timeout for each query")
	flag.DurationVar(&duration, "d", 0, "Duration to run benchmark (e.g. 30s). Overrides -n if set.")
	flag.StringVar(&domainFile, "domains", "", "File containing list of domains (one per line or CSV)")
	flag.StringVar(&serverFile, "servers", "", "File containing list of servers (one per line or YAML)")
	flag.StringVar(&exportFile, "o", "", "Output CSV file for raw results")
	flag.StringVar(&htmlFile, "html", "", "Output HTML report file")
	flag.StringVar(&browserName, "browser", "", "Import domains from browser history (chrome, brave, safari, firefox)")
	flag.BoolVar(&verbose, "v", false, "Verbose logging (show errors and slow queries)")
	flag.BoolVar(&showProgress, "progress", false, "Show progress bar during benchmark")
	flag.Parse()

	// Load config file if specified or found
	var cfg *Config
	if configFile != "" {
		var err error
		cfg, err = loadConfigFile(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded config from %s\n", configFile)
	} else if found := findConfigFile(); found != "" {
		var err error
		cfg, err = loadConfigFile(found)
		if err == nil {
			fmt.Printf("Loaded config from %s\n", found)
		}
	}

	// Set defaults
	if cfg == nil {
		cfg = &Config{
			Concurrency: 50,
			Iterations:  1,
			Timeout:     1 * time.Second,
		}
	}

	// CLI flags override config file
	if concurrency > 0 {
		cfg.Concurrency = concurrency
	}
	if iterations > 0 {
		cfg.Iterations = iterations
	}
	if timeout > 0 {
		cfg.Timeout = timeout
	}
	if duration > 0 {
		cfg.Duration = duration
	}
	if domainFile != "" {
		cfg.DomainFile = domainFile
	}
	if serverFile != "" {
		cfg.ServerFile = serverFile
	}
	if exportFile != "" {
		cfg.ExportCSV = exportFile
	}
	if htmlFile != "" {
		cfg.ExportHTML = htmlFile
	}
	if browserName != "" {
		cfg.BrowserName = browserName
	}
	if verbose {
		cfg.Verbose = verbose
	}
	if showProgress {
		cfg.Progress = showProgress
	}

	// Apply final defaults
	if cfg.Concurrency == 0 {
		cfg.Concurrency = 50
	}
	if cfg.Iterations == 0 {
		cfg.Iterations = 1
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 1 * time.Second
	}

	servers := cfg.Servers
	if len(servers) == 0 {
		servers = defaultServers
	}
	if cfg.ServerFile != "" {
		var err error
		servers, err = readServers(cfg.ServerFile)
		if err != nil {
			fmt.Printf("Error reading server file: %v\n", err)
			os.Exit(1)
		}
	}

	// Validate servers
	validServers, serverWarnings := validation.ValidateServers(servers)
	if len(serverWarnings) > 0 && cfg.Verbose {
		fmt.Println("Server validation warnings:")
		for _, warning := range serverWarnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	if len(validServers) == 0 {
		fmt.Println("Error: no valid servers to test")
		os.Exit(1)
	}
	servers = validServers

	domains := cfg.Domains
	if len(domains) == 0 {
		domains = defaultDomains
	}
	if cfg.DomainFile != "" {
		var err error
		domains, err = readDomains(cfg.DomainFile)
		if err != nil {
			fmt.Printf("Error reading domain file: %v\n", err)
			os.Exit(1)
		}
	} else if cfg.BrowserName != "" {
		fmt.Printf("Extracting domains from %s history...\n", cfg.BrowserName)
		var err error
		domains, err = browser.GetDomains(cfg.BrowserName, 1000) // Limit to 1000 most recent/frequent
		if err != nil {
			if strings.Contains(err.Error(), "operation not permitted") {
				fmt.Printf("\n⚠️  PERMISSION DENIED: macOS prevented access to %s history.\n", cfg.BrowserName)
				fmt.Printf("To fix this:\n")
				fmt.Printf("1. Open System Settings -> Privacy & Security -> Full Disk Access\n")
				fmt.Printf("2. Grant access to your terminal app (e.g., Terminal, iTerm2, VSCode)\n")
				fmt.Printf("3. Restart the terminal and try again.\n\n")
				os.Exit(1)
			}
			fmt.Printf("Error extracting browser history: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d unique domains from %s\n", len(domains), cfg.BrowserName)
	}

	// Validate domains
	validDomains, domainWarnings := validation.ValidateDomains(domains)
	if len(domainWarnings) > 0 && cfg.Verbose {
		fmt.Println("Domain validation warnings:")
		for _, warning := range domainWarnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	if len(validDomains) == 0 {
		fmt.Println("Error: no valid domains to test")
		os.Exit(1)
	}
	domains = validDomains

	fmt.Printf("Starting benchmark...\n")
	if cfg.Duration > 0 {
		fmt.Printf("Servers: %d, Domains: %d, Duration: %v, Concurrency: %d\n", len(servers), len(domains), cfg.Duration, cfg.Concurrency)
	} else {
		fmt.Printf("Servers: %d, Domains: %d, Iterations: %d, Concurrency: %d\n", len(servers), len(domains), cfg.Iterations, cfg.Concurrency)
	}

	config := benchmark.Config{
		Servers:      servers,
		Domains:      domains,
		Iterations:   cfg.Iterations,
		Concurrency:  cfg.Concurrency,
		Timeout:      cfg.Timeout,
		Duration:     cfg.Duration,
		Verbose:      cfg.Verbose,
		ShowProgress: cfg.Progress,
	}

	start := time.Now()
	results := benchmark.Run(config)
	totalTime := time.Since(start)

	stats := calculateStats(results)
	printTable(stats, totalTime)

	if cfg.ExportCSV != "" {
		if err := exportCSV(results, cfg.ExportCSV); err != nil {
			fmt.Printf("Error exporting results: %v\n", err)
		} else {
			fmt.Printf("Results exported to %s\n", cfg.ExportCSV)
		}
	}

	if cfg.ExportHTML != "" {
		if err := generateHTML(stats, totalTime, cfg.ExportHTML); err != nil {
			fmt.Printf("Error generating HTML report: %v\n", err)
		} else {
			fmt.Printf("HTML report generated at %s\n", cfg.ExportHTML)
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

	sortedStats := make([]*ServerStats, 0, len(statsMap))
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
	if _, err := fmt.Fprintln(w, "RANK\tSERVER\tAVG LATENCY\tMIN\tMAX\tLOSS %"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write header: %v\n", err)
	}

	for i, s := range stats {
		if _, err := fmt.Fprintf(w, "%d\t%s\t%v\t%v\t%v\t%.2f%%\n", i+1, s.Server, s.Avg, s.Min, s.Max, s.LossPct); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write row: %v\n", err)
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to flush output: %v\n", err)
	}
}

// ServerConfigYAML matches the expected YAML structure
type ServerConfigYAML struct {
	Servers []string `yaml:"servers"`
}

func readServers(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err := os.ReadFile(path)
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
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", err)
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", err)
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", err)
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", err)
		}
	}()

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
