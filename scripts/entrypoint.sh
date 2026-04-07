#!/bin/sh
# entrypoint.sh — runs dns-bench, saves timestamped results, regenerates index.html
set -e

RESULTS_DIR="/results"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H-%M-%SZ")
HISTORY_CSV="${RESULTS_DIR}/history.csv"
RUN_CSV="${RESULTS_DIR}/results-${TIMESTAMP}.csv"

echo "==> Starting DNS benchmark run at ${TIMESTAMP}"

# Run the benchmark, writing timestamped output files
/app/dns-bench \
  -servers=/config/servers.yaml \
  -domains=/config/domains.txt \
  -c=20 \
  -n=3 \
  -t=3s \
  -o="${RUN_CSV}" \
  -html="${RESULTS_DIR}/report-${TIMESTAMP}.html" \
  -progress

echo "==> Benchmark complete. Appending to history CSV..."

# Append this run's results to the master history CSV, tagging each row with the timestamp.
if [ ! -f "${HISTORY_CSV}" ]; then
  echo "Timestamp,Server,Domain,Duration_ms,Error" > "${HISTORY_CSV}"
fi
# Append data rows (skip header line of the per-run CSV)
tail -n +2 "${RUN_CSV}" | sed "s/^/${TIMESTAMP},/" >> "${HISTORY_CSV}"

echo "==> Regenerating dashboard..."

/app/dns-bench -dashboard "${RESULTS_DIR}"

echo "==> Done."
