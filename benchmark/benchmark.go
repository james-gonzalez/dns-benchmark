package benchmark

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
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
	if strings.HasPrefix(serverAddr, "https://") {
		err = c.measureDoH(serverAddr, m)
	} else if strings.HasPrefix(serverAddr, "tls://") {
		// DoT (DNS over TLS)
		host := strings.TrimPrefix(serverAddr, "tls://")
		// Append default port 853 if not present
		if !strings.Contains(host, ":") {
			host = host + ":853"
		}
		client := new(dns.Client)
		client.Net = "tcp-tls"
		client.Timeout = c.Timeout
		// InsecureSkipVerify is generally bad, but for benchmarking IPs directly it might be needed
		// if the cert expects a hostname. Ideally users provide hostnames for DoT.
		client.TLSConfig = &tls.Config{InsecureSkipVerify: true}

		_, _, err = client.Exchange(m, host)
	} else {
		// Standard UDP
		host := serverAddr
		if !strings.Contains(host, ":") {
			host = host + ":53"
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
		t := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		// Enable HTTP/2 support explicitly
		if err := http2.ConfigureTransport(t); err != nil {
			// Fallback if H2 init fails (unlikely)
		}

		c.httpClient = &http.Client{
			Timeout:   c.Timeout,
			Transport: t,
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("DoH error: %s: %s", resp.Status, string(body))
	}

	// We don't strictly need to unpack the response for benchmarking latency,
	// but it validates the server actually replied with DNS data.
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	respMsg := new(dns.Msg)
	return respMsg.Unpack(respData)
}

// BenchmarkConfig holds the configuration for a benchmark run
type BenchmarkConfig struct {
	Servers     []string
	Domains     []string
	Iterations  int
	Concurrency int
	Timeout     time.Duration
	Duration    time.Duration
	Verbose     bool
}

// Job represents a single benchmark task
type Job struct {
	Server string
	Domain string
}

// Run executes the benchmark with the given configuration
func Run(config BenchmarkConfig) []Result {
	// Use a reasonable buffer size for channels to prevent blocking,
	// but don't try to buffer everything if running for a long duration.
	bufferSize := config.Concurrency * 10
	jobs := make(chan Job, bufferSize)
	results := make(chan Result, bufferSize)

	// Create client
	client := Client{Timeout: config.Timeout}

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
			}
		}()
	}

	// Enqueue jobs
	go func() {
		if config.Duration > 0 {
			// Randomly select jobs to ensure fair coverage across all servers/domains
			// even if the duration is short.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			timer := time.NewTimer(config.Duration)
		loop:
			for {
				select {
				case <-timer.C:
					break loop
				default:
					// Pick random server and domain
					sIdx := rng.Intn(len(config.Servers))
					dIdx := rng.Intn(len(config.Domains))

					job := Job{
						Server: config.Servers[sIdx],
						Domain: config.Domains[dIdx],
					}

					select {
					case <-timer.C:
						break loop
					case jobs <- job:
						// continued
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
		}
		close(jobs)
	}()

	// Wait for workers to finish in a separate goroutine to close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []Result
	for res := range results {
		allResults = append(allResults, res)
	}

	return allResults
}
