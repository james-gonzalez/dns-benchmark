#!/bin/sh
# Entrypoint script for DNS Benchmark CronJob
# Generates timestamped results and maintains an index

set -e

RESULTS_DIR="/results"
TIMESTAMP=$(date +%Y-%m-%d-%H-%M-%S)
CSV_FILE="${RESULTS_DIR}/results-${TIMESTAMP}.csv"
HTML_FILE="${RESULTS_DIR}/results-${TIMESTAMP}.html"
LATEST_CSV="${RESULTS_DIR}/results-latest.csv"
LATEST_HTML="${RESULTS_DIR}/results-latest.html"
INDEX_FILE="${RESULTS_DIR}/index.html"

# Ensure results directory exists and is writable
mkdir -p "${RESULTS_DIR}"
chmod 755 "${RESULTS_DIR}"

echo "Starting DNS Benchmark at $(date)"
echo "Results will be saved to:"
echo "  CSV: ${CSV_FILE}"
echo "  HTML: ${HTML_FILE}"

# Run the benchmark
/app/dns-bench \
  -servers /config/servers.yaml \
  -domains /config/domains.txt \
  -o "${CSV_FILE}" \
  -html "${HTML_FILE}"

# Update latest symlinks
ln -sf "results-${TIMESTAMP}.csv" "${LATEST_CSV}"
ln -sf "results-${TIMESTAMP}.html" "${LATEST_HTML}"

echo "Benchmark completed successfully"
echo "Results available at:"
echo "  Latest CSV: ${LATEST_CSV}"
echo "  Latest HTML: ${LATEST_HTML}"

# Generate index.html listing all available reports
generate_index() {
  cat > "${INDEX_FILE}" << 'INDEXEOF'
<!DOCTYPE html>
<html>
<head>
    <title>DNS Benchmark Results</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            margin: 2rem;
            background: #f4f4f9;
            color: #333;
        }
        .container {
            max-width: 1000px;
            margin: 0 auto;
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        h1 {
            margin-top: 0;
            color: #2c3e50;
        }
        .latest {
            background: #e8f5e9;
            padding: 1rem;
            border-radius: 4px;
            margin-bottom: 2rem;
            border-left: 4px solid #4caf50;
        }
        .latest h2 {
            margin-top: 0;
            color: #2e7d32;
        }
        .latest a {
            display: inline-block;
            margin-right: 1rem;
            padding: 0.5rem 1rem;
            background: #4caf50;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            transition: background 0.3s;
        }
        .latest a:hover {
            background: #45a049;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #2c3e50;
            color: white;
        }
        tr:nth-child(even) {
            background-color: #f9f9f9;
        }
        tr:hover {
            background-color: #f1f1f1;
        }
        a {
            color: #1976d2;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .timestamp {
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>DNS Benchmark Results</h1>
        
        <div class="latest">
            <h2>Latest Results</h2>
            <a href="results-latest.html">📊 View Latest Report</a>
            <a href="results-latest.csv">📥 Download Latest CSV</a>
        </div>

        <h2>All Reports</h2>
        <table>
            <thead>
                <tr>
                    <th>Timestamp</th>
                    <th>Report</th>
                    <th>Data</th>
                </tr>
            </thead>
            <tbody>
INDEXEOF

  # List all results in reverse chronological order
  ls -1t "${RESULTS_DIR}"/results-[0-9]*.html 2>/dev/null | while read html_file; do
    basename_html=$(basename "$html_file")
    timestamp=$(echo "$basename_html" | sed 's/results-//;s/.html//')
    csv_file="${RESULTS_DIR}/results-${timestamp}.csv"
    
    cat >> "${INDEX_FILE}" << ROWEOF
                <tr>
                    <td class="timestamp">${timestamp}</td>
                    <td><a href="${basename_html}">📊 View Report</a></td>
                    <td><a href="results-${timestamp}.csv">📥 CSV</a></td>
                </tr>
ROWEOF
  done

  cat >> "${INDEX_FILE}" << 'INDEXEOF'
            </tbody>
        </table>
    </div>
</body>
</html>
INDEXEOF

  chmod 644 "${INDEX_FILE}"
  echo "Index generated: ${INDEX_FILE}"
}

generate_index

echo "All done! Results are ready to serve."
