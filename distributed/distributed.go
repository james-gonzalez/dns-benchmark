package distributed

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"dns-bench/benchmark"
)

// HostConfig represents a remote host to run benchmarks on
type HostConfig struct {
	Name     string // e.g., "rpi-1", "rpi-2"
	Host     string // hostname or IP
	Port     int    // SSH port (default 22)
	User     string // SSH user
	BinaryPath string // path to dns-bench binary on remote
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

// RunDistributed executes benchmarks on multiple hosts and aggregates results
func RunDistributed(hosts []HostConfig, config benchmark.BenchmarkConfig) ([]AggregatedResult, error) {
	// For now, this is a placeholder that shows the structure
	// In a real implementation, this would:
	// 1. SSH into each host
	// 2. Run dns-bench with the same config
	// 3. Collect results
	// 4. Aggregate and average them

	var results []AggregatedResult
	return results, fmt.Errorf("distributed testing not yet implemented")
}

// AggregateResults combines results from multiple hosts
func AggregateResults(hostResults map[string][]benchmark.Result) []AggregatedResult {
	// Group results by server+domain
	grouped := make(map[string][]float64)

	for _, results := range hostResults {
		for _, r := range results {
			if r.Error == "" {
				key := r.Server + "|" + r.Domain
				grouped[key] = append(grouped[key], float64(r.DurationMs))
			}
		}
	}

	// Calculate statistics
	var aggregated []AggregatedResult
	for key, values := range grouped {
		parts := strings.Split(key, "|")
		if len(parts) != 2 {
			continue
		}

		avg := calculateMean(values)
		min := calculateMin(values)
		max := calculateMax(values)
		stdDev := calculateStdDev(values, avg)

		aggregated = append(aggregated, AggregatedResult{
			Server:    parts[0],
			Domain:    parts[1],
			AverageMs: avg,
			MinMs:     min,
			MaxMs:     max,
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
	w.Write([]string{"timestamp", "server", "domain", "avg_ms", "min_ms", "max_ms", "stddev_ms"})

	timestamp := time.Now().Format(time.RFC3339)
	for _, r := range results {
		w.Write([]string{
			timestamp,
			r.Server,
			r.Domain,
			fmt.Sprintf("%.3f", r.AverageMs),
			fmt.Sprintf("%.3f", r.MinMs),
			fmt.Sprintf("%.3f", r.MaxMs),
			fmt.Sprintf("%.3f", r.StdDev),
		})
	}

	return nil
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
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
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
