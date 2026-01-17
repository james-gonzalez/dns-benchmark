package benchmark

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/http2"
)

// Result holds the outcome of a single DNS query
type Result struct {
	Server   string
	Domain   string
	Duration time.Duration
	Error    error
}

// Client holds configuration for the DNS client
type Client struct {
	Timeout    time.Duration
	httpClient *http.Client
}

// Measure performs a DNS query to a specific server and returns the result
func (c *Client) Measure(serverAddr, domain string) Result {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)

	start := time.Now()
	var err error

	// Detect Protocol
	switch {
	case strings.HasPrefix(serverAddr, "https://"):
		err = c.measureDoH(serverAddr, m)
	case strings.HasPrefix(serverAddr, "tls://"):
		// DoT (DNS over TLS)
		host := strings.TrimPrefix(serverAddr, "tls://")
		// Append default port 853 if not present
		if !strings.Contains(host, ":") {
			host += ":853"
		}
		client := new(dns.Client)
		client.Net = "tcp-tls"
		client.Timeout = c.Timeout
		// InsecureSkipVerify is necessary for benchmarking DNS servers by IP address
		// where the TLS certificate may not match the IP. This is acceptable for
		// performance testing purposes.
		//nolint:gosec // G402: InsecureSkipVerify is intentional for DNS benchmarking
		client.TLSConfig = &tls.Config{InsecureSkipVerify: true}

		_, _, err = client.Exchange(m, host)
	default:
		// Standard UDP
		host := serverAddr
		if !strings.Contains(host, ":") {
			host += ":53"
		}
		client := new(dns.Client)
		client.Timeout = c.Timeout
		_, _, err = client.Exchange(m, host)
	}

	duration := time.Since(start)

	return Result{
		Server:   serverAddr,
		Domain:   domain,
		Duration: duration,
		Error:    err,
	}
}

func (c *Client) measureDoH(url string, m *dns.Msg) error {
	data, err := m.Pack()
	if err != nil {
		return err
	}

	if c.httpClient == nil {
		// Create a transport with TLS config
		// InsecureSkipVerify is necessary for benchmarking DoH servers by IP address
		// where the TLS certificate may not match the IP. This is acceptable for
		// performance testing purposes.
		//nolint:gosec // G402: InsecureSkipVerify is intentional for DNS benchmarking
		t := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		// Enable HTTP/2 support explicitly
		_ = http2.ConfigureTransport(t) // Ignore error - fallback to HTTP/1.1 is acceptable

		c.httpClient = &http.Client{
			Timeout:   c.Timeout,
			Transport: t,
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("DoH error: %s (failed to read body: %w)", resp.Status, err)
		}
		return fmt.Errorf("DoH error: %s: %s", resp.Status, string(body))
	}

	// We don't strictly need to unpack the response for benchmarking latency,
	// but it validates the server actually replied with DNS data.
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	respMsg := new(dns.Msg)
	return respMsg.Unpack(respData)
}

// Config holds the configuration for a benchmark run
type Config struct {
	Servers      []string
	Domains      []string
	Iterations   int
	Concurrency  int
	Timeout      time.Duration
	Duration     time.Duration
	Verbose      bool
	ShowProgress bool // Show progress updates
}

// ProgressUpdate represents benchmark progress
type ProgressUpdate struct {
	Completed int
	Total     int
	Elapsed   time.Duration
}

// Job represents a single benchmark task
type Job struct {
	Server string
	Domain string
}

// Run executes the benchmark with the given configuration
func Run(config Config) []Result {
	// Use a reasonable buffer size for channels to prevent blocking,
	// but don't try to buffer everything if running for a long duration.
	bufferSize := config.Concurrency * 10
	jobs := make(chan Job, bufferSize)
	results := make(chan Result, bufferSize)

	// Create client
	client := Client{Timeout: config.Timeout}

	// Calculate total jobs for progress tracking
	var totalJobs int
	if config.Duration == 0 {
		totalJobs = len(config.Servers) * len(config.Domains) * config.Iterations
	}

	// Progress tracking
	var completed int
	var progressMu sync.Mutex
	startTime := time.Now()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				res := client.Measure(job.Server, job.Domain)
				if config.Verbose {
					if res.Error != nil {
						fmt.Printf("[%s] Error resolving %s: %v\n", job.Server, job.Domain, res.Error)
					} else if res.Duration > 500*time.Millisecond {
						fmt.Printf("[%s] Slow resolve %s: %v\n", job.Server, job.Domain, res.Duration)
					}
				}
				results <- res

				// Update progress
				if config.ShowProgress && totalJobs > 0 {
					progressMu.Lock()
					completed++
					if completed%10 == 0 || completed == totalJobs {
						elapsed := time.Since(startTime)
						pct := float64(completed) / float64(totalJobs) * 100
						fmt.Printf("\rProgress: %d/%d (%.1f%%) - Elapsed: %v", completed, totalJobs, pct, elapsed.Round(time.Second))
					}
					progressMu.Unlock()
				}
			}
		}()
	}

	// Enqueue jobs
	go func() {
		if config.Duration > 0 {
			// Use context for clean cancellation
			ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
			defer cancel()

			// Randomly select jobs to ensure fair coverage across all servers/domains
			//nolint:gosec // G404: math/rand is sufficient for non-cryptographic benchmark randomization
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for {
				select {
				case <-ctx.Done():
					close(jobs)
					return
				default:
					// Pick random server and domain
					sIdx := rng.Intn(len(config.Servers))
					dIdx := rng.Intn(len(config.Domains))

					job := Job{
						Server: config.Servers[sIdx],
						Domain: config.Domains[dIdx],
					}

					select {
					case <-ctx.Done():
						close(jobs)
						return
					case jobs <- job:
						// Job sent successfully
					}
				}
			}
		} else {
			for i := 0; i < config.Iterations; i++ {
				for _, server := range config.Servers {
					for _, domain := range config.Domains {
						jobs <- Job{Server: server, Domain: domain}
					}
				}
			}
			close(jobs)
		}
	}()

	// Wait for workers to finish in a separate goroutine to close results channel
	go func() {
		wg.Wait()
		if config.ShowProgress && totalJobs > 0 {
			fmt.Println() // New line after progress bar
		}
		close(results)
	}()

	// Collect results
	allResults := make([]Result, 0, bufferSize)
	for res := range results {
		allResults = append(allResults, res)
	}

	return allResults
}
