package distributed

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dns-bench/benchmark"

	"golang.org/x/crypto/ssh"
)

// HostConfig represents a remote host to run benchmarks on
type HostConfig struct {
	Name       string // e.g., "rpi-1", "rpi-2"
	Host       string // hostname or IP
	Port       int    // SSH port (default 22)
	User       string // SSH user
	BinaryPath string // path to dns-bench binary on remote
	KeyPath    string // path to SSH private key file (default: ~/.ssh/id_rsa)
}

// AggregatedResult represents results from multiple hosts
type AggregatedResult struct {
	Server      string
	Domain      string
	HostResults map[string]benchmark.Result // hostname -> result
	AverageMs   float64
	MinMs       float64
	MaxMs       float64
	StdDev      float64
	Error       string
}

// jsonResult represents the JSON output from remote dns-bench binary
type jsonResult struct {
	Server     string  `json:"server"`
	Domain     string  `json:"domain"`
	DurationMs float64 `json:"duration_ms"`
	Error      string  `json:"error"`
}

// RunDistributed executes benchmarks on multiple hosts and aggregates results
func RunDistributed(hosts []HostConfig, config benchmark.Config) ([]AggregatedResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	hostResults := make(map[string][]benchmark.Result)
	successCount := 0

	for _, host := range hosts {
		wg.Add(1)
		go func(h HostConfig) {
			defer wg.Done()

			results, err := runOnHost(h, config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running on host %s: %v\n", h.Name, err)
				return
			}

			mu.Lock()
			hostResults[h.Name] = results
			successCount++
			mu.Unlock()
		}(host)
	}

	wg.Wait()

	if successCount == 0 {
		return nil, fmt.Errorf("all hosts failed to execute benchmarks")
	}

	return AggregateResults(hostResults), nil
}

// runOnHost executes the benchmark on a single remote host via SSH
func runOnHost(host HostConfig, config benchmark.Config) ([]benchmark.Result, error) {
	// Set default port
	port := host.Port
	if port == 0 {
		port = 22
	}

	// Set default key path
	keyPath := host.KeyPath
	if keyPath == "" {
		usr, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
		keyPath = filepath.Join(usr.HomeDir, ".ssh", "id_rsa")
	}

	// Read private key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key from %s: %w", keyPath, err)
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User: host.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // G106: For benchmarking, host key verification is not critical
		Timeout:         10 * time.Second,
	}

	// Connect to remote host
	addr := fmt.Sprintf("%s:%d", host.Host, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}
	defer client.Close()

	// Build command to run on remote host
	cmd := buildRemoteCommand(host.BinaryPath, config)

	// Create SSH session
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Run command and capture output
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run remote command: %w (output: %s)", err, string(output))
	}

	// Parse JSON output
	var jsonResults []jsonResult
	if err := json.Unmarshal(output, &jsonResults); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w (output: %s)", err, string(output))
	}

	// Convert to benchmark.Result
	results := make([]benchmark.Result, 0, len(jsonResults))
	for _, jr := range jsonResults {
		var resultErr error
		if jr.Error != "" {
			resultErr = fmt.Errorf("%s", jr.Error)
		}

		results = append(results, benchmark.Result{
			Server:   jr.Server,
			Domain:   jr.Domain,
			Duration: time.Duration(jr.DurationMs * float64(time.Millisecond)),
			Error:    resultErr,
		})
	}

	return results, nil
}

// buildRemoteCommand constructs the shell command to run on the remote host
func buildRemoteCommand(binaryPath string, config benchmark.Config) string {
	var cmd strings.Builder

	// Create temp files and populate them
	cmd.WriteString("servers_file=$(mktemp) && domains_file=$(mktemp) && ")

	// Write servers to temp file
	cmd.WriteString("printf '%s\\n' ")
	for i, server := range config.Servers {
		if i > 0 {
			cmd.WriteString(" ")
		}
		// Escape single quotes in server names
		escaped := strings.ReplaceAll(server, "'", "'\\''")
		cmd.WriteString(fmt.Sprintf("'%s'", escaped))
	}
	cmd.WriteString(" > \"$servers_file\" && ")

	// Write domains to temp file
	cmd.WriteString("printf '%s\\n' ")
	for i, domain := range config.Domains {
		if i > 0 {
			cmd.WriteString(" ")
		}
		// Escape single quotes in domain names
		escaped := strings.ReplaceAll(domain, "'", "'\\''")
		cmd.WriteString(fmt.Sprintf("'%s'", escaped))
	}
	cmd.WriteString(" > \"$domains_file\" && ")

	// Run dns-bench with JSON output
	cmd.WriteString(fmt.Sprintf("%s -json -servers \"$servers_file\" -domains \"$domains_file\"", binaryPath))
	cmd.WriteString(fmt.Sprintf(" -n %d", config.Iterations))
	cmd.WriteString(fmt.Sprintf(" -c %d", config.Concurrency))
	cmd.WriteString(fmt.Sprintf(" -t %s", config.Timeout.String()))

	// Clean up temp files
	cmd.WriteString("; rm -f \"$servers_file\" \"$domains_file\"")

	return cmd.String()
}

// AggregateResults combines results from multiple hosts
func AggregateResults(hostResults map[string][]benchmark.Result) []AggregatedResult {
	// Group results by server+domain
	grouped := make(map[string][]float64)

	for _, results := range hostResults {
		for _, r := range results {
			if r.Error == nil {
				key := r.Server + "|" + r.Domain
				grouped[key] = append(grouped[key], float64(r.Duration.Milliseconds()))
			}
		}
	}

	// Calculate statistics
	aggregated := make([]AggregatedResult, 0, len(grouped))
	for key, values := range grouped {
		parts := strings.Split(key, "|")
		if len(parts) != 2 {
			continue
		}

		avg := calculateMean(values)
		minMs := calculateMin(values)
		maxMs := calculateMax(values)
		stdDev := calculateStdDev(values, avg)

		aggregated = append(aggregated, AggregatedResult{
			Server:    parts[0],
			Domain:    parts[1],
			AverageMs: avg,
			MinMs:     minMs,
			MaxMs:     maxMs,
			StdDev:    stdDev,
		})
	}

	// Sort by server, then domain
	sort.Slice(aggregated, func(i, j int) bool {
		if aggregated[i].Server != aggregated[j].Server {
			return aggregated[i].Server < aggregated[j].Server
		}
		return aggregated[i].Domain < aggregated[j].Domain
	})

	return aggregated
}

// ExportAggregatedCSV writes aggregated results to CSV
func ExportAggregatedCSV(results []AggregatedResult, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Write header
	if err := w.Write([]string{"timestamp", "server", "domain", "avg_ms", "min_ms", "max_ms", "stddev_ms"}); err != nil {
		return err
	}

	timestamp := time.Now().Format(time.RFC3339)
	for _, r := range results {
		if err := w.Write([]string{
			timestamp,
			r.Server,
			r.Domain,
			fmt.Sprintf("%.3f", r.AverageMs),
			fmt.Sprintf("%.3f", r.MinMs),
			fmt.Sprintf("%.3f", r.MaxMs),
			fmt.Sprintf("%.3f", r.StdDev),
		}); err != nil {
			return err
		}
	}

	return w.Error()
}

// Helper functions
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance)
}
