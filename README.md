# DNS Benchmark

A high-performance DNS benchmarking utility written in Go.

## Features

- Measure query latency (Avg, Min, Max)
- Supports **UDP**, **DoT** (DNS over TLS), and **DoH** (DNS over HTTPS)
- Track packet loss/errors
- Concurrent queries
- Customizable server and domain lists
- Export results to CSV

## Usage

### Build

**Standard (macOS/Linux with local build tools):**
Includes Browser History support (requires CGO).
```bash
go build -o dns-bench
```

**Cross-Compile for Linux ARM64 (e.g., Raspberry Pi):**
*Note: Browser history import is disabled in CGO-free builds.*
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dns-bench-linux-arm64
```

**Cross-Compile for Linux AMD64 (Servers):**
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dns-bench-linux-amd64
```

### Run with defaults

```bash
./dns-bench
```

### Options

```
  -c int
        Number of concurrent queries (default 50)
  -n int
        Number of iterations per domain per server (default 1)
  -t duration
        Timeout for each query (default 1s)
  -d duration
        Duration to run benchmark (e.g. 30s). Overrides -n if set.
  -domains string
        File containing list of domains (one per line or CSV)
  -browser string
        Import domains from browser history (chrome, brave, safari, firefox)
  -servers string
        File containing list of servers (one per line or YAML)
  -o string
        Output CSV file for raw results
  -html string
        Output HTML report file
  -v    
        Verbose logging (show errors and slow queries)
```

### Browser History Integration

The tool can extract domains directly from your browser history.

```bash
./dns-bench -browser safari
```

**Note on macOS Permissions:**
If you see a "Permission Denied" error (especially with Safari), you need to grant **Full Disk Access** to your terminal application (e.g., Terminal, iTerm2, VSCode) in System Settings -> Privacy & Security.

**Alternative Workaround:**
If you prefer not to grant full permissions, or if direct integration fails, you can use the included helper script:

```bash
# Export history to a file
./scripts/export_history.sh safari my_domains.csv

# Run benchmark with the exported file
./dns-bench -domains my_domains.csv
```
```bash
./dns-bench -c 50
```

**Run for a specific duration (e.g., 30 seconds):**
```bash
./dns-bench -d 30s
```

**Test with domains from your Chrome history:**
```bash
./dns-bench -browser chrome
```

**Run 10 iterations per domain:**
```bash
./dns-bench -n 10
```

**Use custom lists (TXT, YAML, CSV):**
```bash
./dns-bench -servers servers.yaml -domains top-1000.csv
```

**YAML Server File Format:**
Supports standard UDP, DoT (`tls://`), and DoH (`https://`).

```yaml
servers:
  - 8.8.8.8                        # UDP
  - tls://1.1.1.1                  # DoT
  - https://dns.google/dns-query   # DoH
```

**CSV Domain File Format:**
The tool supports both simple lists and structured CSVs. It will look for a column named "domain" or default to the first column.

*Simple:*
```csv
google.com
netflix.com
```

*Structured:*
```csv
rank,domain,traffic
1,google.com,high
2,facebook.com,high
```

**Export results:**
```bash
./dns-bench -o results.csv
```
