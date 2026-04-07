#!/bin/sh
# entrypoint.sh — runs dns-bench, saves timestamped results, regenerates index.html
set -e

RESULTS_DIR="/results"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H-%M-%SZ")
HISTORY_CSV="${RESULTS_DIR}/history.csv"

echo "==> Starting DNS benchmark run at ${TIMESTAMP}"

# Run the benchmark, writing timestamped output files
/app/dns-bench \
  -servers=/config/servers.yaml \
  -domains=/config/domains.txt \
  -c=20 \
  -n=3 \
  -t=3s \
  -o="${RESULTS_DIR}/results-${TIMESTAMP}.csv" \
  -html="${RESULTS_DIR}/report-${TIMESTAMP}.html" \
  -progress

echo "==> Benchmark complete. Appending to history CSV..."

# Append this run's results to the master history CSV, tagging each row with the timestamp.
# Skip the header row on all but the very first append.
RUN_CSV="${RESULTS_DIR}/results-${TIMESTAMP}.csv"
if [ ! -f "${HISTORY_CSV}" ]; then
  # First ever run — write header with extra Timestamp column
  echo "Timestamp,Server,Domain,Duration_ms,Error" > "${HISTORY_CSV}"
fi
# Append data rows (skip header line of the per-run CSV)
tail -n +2 "${RUN_CSV}" | sed "s/^/${TIMESTAMP},/" >> "${HISTORY_CSV}"

echo "==> Regenerating index.html..."

# ── Build the per-server summary table from history.csv ──────────────────────
# Produces lines like: "server_name avg_ms" sorted by avg ascending
SUMMARY=$(awk -F',' '
  NR > 1 && $5 == "" && $4 > 0 {
    sum[$2] += $4
    count[$2]++
  }
  END {
    for (s in sum) {
      printf "%.3f %s\n", sum[s]/count[s], s
    }
  }
' "${HISTORY_CSV}" | sort -n)

# ── Build HTML rows for the summary table ────────────────────────────────────
SUMMARY_ROWS=""
RANK=1
echo "${SUMMARY}" | while IFS=' ' read -r avg server; do
  [ -z "$server" ] && continue
  SUMMARY_ROWS="${SUMMARY_ROWS}<tr><td>${RANK}</td><td><code>${server}</code></td><td>${avg} ms</td></tr>"
  RANK=$((RANK + 1))
done

# ── Build HTML rows for the reports list ─────────────────────────────────────
RECENT_ROWS=""
ARCHIVE_BLOCKS=""
CURRENT_MONTH=""
RUN_INDEX=0

for f in $(ls -t "${RESULTS_DIR}"/report-*.html 2>/dev/null); do
  fname=$(basename "$f")
  ts=$(echo "$fname" | sed 's/report-//;s/\.html//')
  row="<tr><td>${ts}</td><td><a href=\"${fname}\">View Report</a></td><td><a href=\"results-${ts}.csv\">Download CSV</a></td></tr>"

  if [ "${RUN_INDEX}" -lt 10 ]; then
    RECENT_ROWS="${RECENT_ROWS}${row}"
  else
    month=$(printf "%s" "${ts}" | cut -c1-7)
    if [ "${month}" != "${CURRENT_MONTH}" ]; then
      if [ -n "${CURRENT_MONTH}" ]; then
        ARCHIVE_BLOCKS="${ARCHIVE_BLOCKS}</tbody></table></details>"
      fi

      ARCHIVE_BLOCKS="${ARCHIVE_BLOCKS}<details class=\"month-group\"><summary>${month}</summary><table><thead><tr><th>Timestamp (UTC)</th><th>Report</th><th>Raw Data</th></tr></thead><tbody>"
      CURRENT_MONTH="${month}"
    fi

    ARCHIVE_BLOCKS="${ARCHIVE_BLOCKS}${row}"
  fi

  RUN_INDEX=$((RUN_INDEX + 1))
done

if [ -n "${CURRENT_MONTH}" ]; then
  ARCHIVE_BLOCKS="${ARCHIVE_BLOCKS}</tbody></table></details>"
fi

if [ -z "${RECENT_ROWS}" ]; then
  RECENT_ROWS="<tr><td colspan=\"3\">No run reports yet.</td></tr>"
fi

if [ -z "${ARCHIVE_BLOCKS}" ]; then
  ARCHIVE_BLOCKS="<p class=\"updated\">No archived runs yet.</p>"
fi

# ── Write index.html ──────────────────────────────────────────────────────────
cat > "${RESULTS_DIR}/index.html" <<HTML
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>DNS Benchmark Dashboard</title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 0; background: #f0f2f5; color: #1a1a2e; }
    header { background: #16213e; color: #e2e8f0; padding: 1.5rem 2rem; }
    header h1 { margin: 0; font-size: 1.6rem; }
    header p  { margin: 0.25rem 0 0; font-size: 0.9rem; color: #94a3b8; }
    main { max-width: 1100px; margin: 2rem auto; padding: 0 1.5rem; }
    .card { background: white; border-radius: 10px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); padding: 1.5rem; margin-bottom: 2rem; }
    h2 { margin-top: 0; font-size: 1.1rem; color: #16213e; border-bottom: 2px solid #e2e8f0; padding-bottom: 0.5rem; }
    table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
    th { background: #16213e; color: white; padding: 10px 14px; text-align: left; }
    td { padding: 9px 14px; border-bottom: 1px solid #f0f2f5; }
    tr:last-child td { border-bottom: none; }
    tr:hover td { background: #f8fafc; }
    a { color: #3b82f6; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .chart-wrap { position: relative; height: 340px; }
    .updated { font-size: 0.8rem; color: #94a3b8; text-align: right; margin-top: -1rem; margin-bottom: 1rem; }
    .section-subtitle { color: #64748b; font-size: 0.9rem; margin-top: -0.4rem; margin-bottom: 1rem; }
    .month-group { border: 1px solid #e2e8f0; border-radius: 8px; margin-top: 0.75rem; overflow: hidden; }
    .month-group > summary { cursor: pointer; list-style: none; padding: 0.85rem 1rem; font-weight: 600; background: #f8fafc; color: #1e293b; }
    .month-group > summary::-webkit-details-marker { display: none; }
    .month-group > summary::before { content: "▶"; display: inline-block; margin-right: 0.5rem; color: #64748b; transition: transform 0.2s ease; }
    .month-group[open] > summary::before { transform: rotate(90deg); }
  </style>
</head>
<body>
<header>
  <h1>DNS Benchmark Dashboard</h1>
  <p>UK DoH servers &amp; local resolvers &mdash; tested $(date -u +"%d %b %Y, %H:%M UTC")</p>
</header>
<main>

  <div class="card">
    <h2>Public DNS — Average Latency Over Time</h2>
    <div class="chart-wrap"><canvas id="publicChart"></canvas></div>
  </div>

  <div class="card">
    <h2>Private / Local DNS — Average Latency Over Time</h2>
    <div class="chart-wrap"><canvas id="privateChart"></canvas></div>
  </div>

  <div class="card">
    <h2>Public DNS Rankings</h2>
    <p class="updated">Averaged across all runs</p>
    <table>
      <thead><tr><th>#</th><th>Server</th><th>Avg Latency (ms)</th></tr></thead>
      <tbody id="publicSummaryBody"></tbody>
    </table>
  </div>

  <div class="card">
    <h2>Private / Local DNS Rankings</h2>
    <p class="updated">Averaged across all runs</p>
    <table>
      <thead><tr><th>#</th><th>Server</th><th>Avg Latency (ms)</th></tr></thead>
      <tbody id="privateSummaryBody"></tbody>
    </table>
  </div>

  <div class="card">
    <h2>Individual Run Reports</h2>
    <p class="section-subtitle">Latest 10 runs shown first. Older runs are grouped by month.</p>
    <table>
      <thead><tr><th>Timestamp (UTC)</th><th>Report</th><th>Raw Data</th></tr></thead>
      <tbody>
        ${RECENT_ROWS}
      </tbody>
    </table>
    ${ARCHIVE_BLOCKS}
  </div>

</main>

<script>
// Parse history CSV embedded at build time
const csvText = \`$(cat "${HISTORY_CSV}" 2>/dev/null || echo "Timestamp,Server,Domain,Duration_ms,Error")\`;

const rows = csvText.trim().split('\n').slice(1).filter(r => r);
const byServerByTime = {};
const allTimestamps = new Set();

rows.forEach(row => {
  const [ts, server, , durStr, err] = row.split(',');
  if (err && err.trim()) return;
  const dur = parseFloat(durStr);
  if (!dur || dur <= 0) return;
  allTimestamps.add(ts);
  if (!byServerByTime[server]) byServerByTime[server] = {};
  if (!byServerByTime[server][ts]) byServerByTime[server][ts] = { sum: 0, count: 0 };
  byServerByTime[server][ts].sum += dur;
  byServerByTime[server][ts].count++;
});

const timestamps = [...allTimestamps].sort();
const servers = Object.keys(byServerByTime).sort();

// Classify servers as private (local) or public
function isPrivate(server) {
  return /^192\.168\./.test(server) ||
         /^10\./.test(server) ||
         /^172\.(1[6-9]|2\d|3[01])\./.test(server) ||
         /^127\./.test(server) ||
         server === 'localhost';
}

const privateServers = servers.filter(isPrivate);
const publicServers  = servers.filter(s => !isPrivate(s));

// Colour palette
const palette = [
  '#3b82f6','#ef4444','#10b981','#f59e0b','#8b5cf6',
  '#06b6d4','#f97316','#84cc16','#ec4899','#6366f1',
  '#14b8a6','#a855f7','#fb923c','#22c55e','#e11d48'
];

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: { position: 'bottom', labels: { boxWidth: 12, font: { size: 11 } } },
    tooltip: { callbacks: { label: ctx => \` \${ctx.dataset.label}: \${ctx.parsed.y} ms\` } }
  },
  scales: {
    x: { ticks: { maxTicksLimit: 10, maxRotation: 30 } },
    y: { title: { display: true, text: 'Avg Latency (ms)' }, beginAtZero: true }
  }
};

function buildDatasets(serverList) {
  return serverList.map((server, i) => ({
    label: server,
    data: timestamps.map(ts => {
      const d = byServerByTime[server][ts];
      return d ? +(d.sum / d.count).toFixed(3) : null;
    }),
    borderColor: palette[i % palette.length],
    backgroundColor: palette[i % palette.length] + '22',
    tension: 0.3,
    spanGaps: true,
    pointRadius: 3,
  }));
}

new Chart(document.getElementById('publicChart'), {
  type: 'line',
  data: { labels: timestamps, datasets: buildDatasets(publicServers) },
  options: chartOptions,
});

new Chart(document.getElementById('privateChart'), {
  type: 'line',
  data: { labels: timestamps, datasets: buildDatasets(privateServers) },
  options: chartOptions,
});

// Populate summary tables from all-time averages
function populateTable(serverList, tbodyId) {
  const avg = serverList.map(server => {
    let sum = 0, count = 0;
    Object.values(byServerByTime[server]).forEach(d => { sum += d.sum; count += d.count; });
    return { server, avg: count ? +(sum / count).toFixed(3) : Infinity };
  }).sort((a, b) => a.avg - b.avg);

  const tbody = document.getElementById(tbodyId);
  avg.forEach((row, i) => {
    const tr = document.createElement('tr');
    tr.innerHTML = \`<td>\${i+1}</td><td><code>\${row.server}</code></td><td>\${row.avg} ms</td>\`;
    tbody.appendChild(tr);
  });
}

populateTable(publicServers,  'publicSummaryBody');
populateTable(privateServers, 'privateSummaryBody');
</script>
</body>
</html>
HTML

echo "==> index.html updated. Run complete."
