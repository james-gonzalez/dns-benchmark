# DNS Benchmark - New Features

## 1. Clear Results Button

A "Clear All Results" button has been added to the dashboard UI that allows you to delete all benchmark results, history, and reports with a single click.

### How to Use

1. Navigate to the DNS Benchmark dashboard
2. Click the **🗑️ Clear All Results** button in the header
3. A confirmation modal will appear
4. Click **Yes, Delete All** to confirm
5. The dashboard will automatically reload with a fresh state

### API Endpoint

You can also clear results programmatically:

```bash
curl -X POST http://localhost:8080/api/clear-results
```

Response:
```json
{
  "deleted": 5
}
```

## 2. Dashboard Server Mode

A new server mode allows you to serve the dashboard with API endpoints:

```bash
./dns-bench -serve 8080
```

This will:
- Start an HTTP server on port 8080
- Serve the dashboard at `http://localhost:8080`
- Provide API endpoints for clearing results
- Automatically regenerate the dashboard

## 3. Multi-Host Testing (Experimental)

The foundation for distributed testing across multiple hosts has been added. This allows you to:

- Run benchmarks on multiple remote machines (e.g., Raspberry Pis)
- Aggregate results from all hosts
- Calculate average, min, max, and standard deviation across hosts
- Export aggregated results to CSV

### Configuration

Create a `hosts.yaml` file:

```yaml
hosts:
  - name: rpi-1
    host: 192.168.1.100
    port: 22
    user: pi
    binary_path: /usr/local/bin/dns-bench
  - name: rpi-2
    host: 192.168.1.101
    port: 22
    user: pi
    binary_path: /usr/local/bin/dns-bench
```

### Usage (Future)

```bash
./dns-bench -distributed hosts.yaml -servers servers.yaml -domains domains.txt -o aggregated-results.csv
```

This will:
1. SSH into each host
2. Run the benchmark with the same configuration
3. Collect results from all hosts
4. Aggregate and average the results
5. Export to CSV with min/max/stddev columns

### Result Format

Aggregated results include:
- `avg_ms`: Average latency across all hosts
- `min_ms`: Minimum latency observed
- `max_ms`: Maximum latency observed
- `stddev_ms`: Standard deviation (shows consistency)

## Implementation Details

### API Package (`api/api.go`)

- `ClearResultsHandler`: Deletes all result files and regenerates the dashboard
- `HealthHandler`: Returns API health status

### Distributed Package (`distributed/distributed.go`)

- `HostConfig`: Configuration for remote hosts
- `AggregatedResult`: Result structure with statistics
- `AggregateResults`: Combines results from multiple hosts
- `ExportAggregatedCSV`: Exports aggregated results to CSV

### Server Package (`server.go`)

- `serveDashboard`: HTTP server for dashboard and API
- Serves static files from results directory
- Handles API requests

## Future Enhancements

1. **SSH Integration**: Automatically run benchmarks on remote hosts via SSH
2. **Real-time Aggregation**: Stream results as they complete from each host
3. **Web UI for Multi-Host**: Configure hosts and run distributed tests from the dashboard
4. **Result Comparison**: Compare results across different hosts and time periods
5. **Alerting**: Notify when results deviate from baseline
