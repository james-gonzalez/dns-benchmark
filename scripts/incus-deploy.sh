#!/usr/bin/env bash
# incus-deploy.sh — build and deploy dns-bench as an Incus container
#
# Usage:
#   ./scripts/incus-deploy.sh                          # create + deploy (first time)
#   ./scripts/incus-deploy.sh --update                 # push new binary + restart
#   ./scripts/incus-deploy.sh --destroy                # stop and delete the container
#   ./scripts/incus-deploy.sh --status                 # show service status + IP
#   ./scripts/incus-deploy.sh --run-now                # trigger a benchmark run immediately
#
# Environment variables (optional — enable K8s sync):
#   K8S_NODE   IP/hostname of the K3s node  (e.g. 192.168.86.20)
#   K8S_USER   SSH user on the K3s node     (e.g. jamesgonzalez)
#
# The container serves the dns-bench dashboard at http://<container-ip>:8080
# When K8S_NODE/K8S_USER are set, benchmark results are also synced to
# dns-benchmark.lan via kubectl cp into the nginx pod.

set -euo pipefail

CONTAINER="dns-bench"
IMAGE="images:debian/12"
BINARY_SRC="./dns-bench-linux-amd64"
BINARY_DEST="/usr/local/bin/dns-bench"
RESULTS_DIR="/results"
CONFIG_DIR="/config"

# K8s sync config (override via env or edit here)
K8S_NODE="${K8S_NODE:-192.168.86.20}"
K8S_USER="${K8S_USER:-jamesgonzalez}"

# ── helpers ──────────────────────────────────────────────────────────────────

log()  { echo "==> $*"; }
die()  { echo "ERROR: $*" >&2; exit 1; }

require_incus() {
    command -v incus >/dev/null 2>&1 || die "incus not found in PATH"
}

build_binary() {
    log "Building linux/amd64 binary..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BINARY_SRC" .
    log "Built: $BINARY_SRC"
}

container_exists() {
    incus info "$CONTAINER" >/dev/null 2>&1
}

container_running() {
    incus list "$CONTAINER" --format=csv -c s | grep -q "^RUNNING$"
}

get_ip() {
    incus list "$CONTAINER" --format=csv -c 4 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

push_systemd_unit() {
    local src="$1" dest="$2"
    incus file push "$src" "${CONTAINER}${dest}"
}

# ── SSH key setup ─────────────────────────────────────────────────────────────

setup_ssh_key() {
    log "Setting up SSH key for K8s sync..."

    # Generate key inside the container if it doesn't exist
    incus exec "$CONTAINER" -- bash -c '
        mkdir -p /root/.ssh && chmod 700 /root/.ssh
        if [ ! -f /root/.ssh/dns-bench-sync ]; then
            ssh-keygen -t ed25519 -N "" -f /root/.ssh/dns-bench-sync -C "dns-bench-incus-sync"
            echo "Generated new SSH key."
        else
            echo "SSH key already exists."
        fi
    '

    # Pull the public key out of the container
    PUBKEY=$(incus exec "$CONTAINER" -- cat /root/.ssh/dns-bench-sync.pub)

    log "Authorizing key on ${K8S_USER}@${K8S_NODE}..."
    ssh "${K8S_USER}@${K8S_NODE}" "
        mkdir -p ~/.ssh && chmod 700 ~/.ssh
        touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys
        if ! grep -qF '${PUBKEY}' ~/.ssh/authorized_keys 2>/dev/null; then
            echo '${PUBKEY}' >> ~/.ssh/authorized_keys
            echo 'Key added.'
        else
            echo 'Key already authorized.'
        fi
    "

    log "Deploying sync script to ${K8S_USER}@${K8S_NODE}..."
    scp ./scripts/sync-to-k8s.sh "${K8S_USER}@${K8S_NODE}:~/sync-to-k8s.sh"
    ssh "${K8S_USER}@${K8S_NODE}" "chmod +x ~/sync-to-k8s.sh"

    log "Writing sync env config into container..."
    incus exec "$CONTAINER" -- bash -c "cat > /etc/dns-bench-sync.env <<EOF
K8S_NODE=${K8S_NODE}
K8S_USER=${K8S_USER}
K8S_SYNC_SCRIPT=~/sync-to-k8s.sh
EOF"

    log "SSH key setup complete."
}

# ── actions ───────────────────────────────────────────────────────────────────

do_create() {
    build_binary

    if container_exists; then
        die "Container '$CONTAINER' already exists. Use --update to redeploy, or --destroy first."
    fi

    log "Launching container '$CONTAINER' from $IMAGE..."
    incus launch "$IMAGE" "$CONTAINER" \
        -c limits.cpu=2 \
        -c limits.memory=512MiB \
        -c boot.autostart=true \
        -c boot.autostart.priority=50

    log "Waiting for container to be ready..."
    sleep 4

    log "Installing openssh-client..."
    incus exec "$CONTAINER" -- apt-get update -qq
    incus exec "$CONTAINER" -- apt-get install -y -qq openssh-client

    log "Creating directories..."
    incus exec "$CONTAINER" -- mkdir -p "$RESULTS_DIR" "$CONFIG_DIR"

    log "Pushing binary..."
    incus file push "$BINARY_SRC" "${CONTAINER}${BINARY_DEST}" --mode=755

    log "Pushing runner script..."
    incus file push ./scripts/dns-bench-run.sh "${CONTAINER}/usr/local/bin/dns-bench-run.sh" --mode=755

    log "Pushing config files..."
    incus file push ./servers.yaml "${CONTAINER}${CONFIG_DIR}/servers.yaml"
    incus file push ./hostnames.txt "${CONTAINER}${CONFIG_DIR}/domains.txt"

    log "Pushing systemd units..."
    push_systemd_unit ./scripts/dns-bench.service     /etc/systemd/system/dns-bench.service
    push_systemd_unit ./scripts/dns-bench-run.service /etc/systemd/system/dns-bench-run.service
    push_systemd_unit ./scripts/dns-bench-run.timer   /etc/systemd/system/dns-bench-run.timer

    log "Enabling services and timer..."
    incus exec "$CONTAINER" -- systemctl daemon-reload
    incus exec "$CONTAINER" -- systemctl enable dns-bench.service
    incus exec "$CONTAINER" -- systemctl start dns-bench.service
    incus exec "$CONTAINER" -- systemctl enable dns-bench-run.timer
    incus exec "$CONTAINER" -- systemctl start dns-bench-run.timer

    log "Verifying dashboard service..."
    sleep 1
    incus exec "$CONTAINER" -- systemctl is-active dns-bench.service

    # Set up K8s sync if node is reachable
    if ssh -o ConnectTimeout=3 -o BatchMode=yes "${K8S_USER}@${K8S_NODE}" true 2>/dev/null; then
        setup_ssh_key
    else
        log "K8s node ${K8S_NODE} not reachable — skipping sync setup."
        log "  Run './scripts/incus-deploy.sh --setup-sync' later to enable it."
    fi

    IP=$(get_ip)
    log "Done!"
    log "  Dashboard:      http://${IP}:8080"
    log "  dns-benchmark.lan updated after each scheduled run (every 6h)"
    log "  Logs:           incus exec $CONTAINER -- journalctl -u dns-bench-run.service -f"
    log "  Shell:          incus exec $CONTAINER -- bash"
    log "  Run now:        ./scripts/incus-deploy.sh --run-now"
}

do_update() {
    build_binary

    container_exists || die "Container '$CONTAINER' does not exist. Run without flags to create it."
    container_running || { log "Starting container..."; incus start "$CONTAINER"; sleep 2; }

    log "Stopping dashboard service (to release binary lock)..."
    incus exec "$CONTAINER" -- systemctl stop dns-bench.service

    log "Pushing updated binary..."
    incus file push "$BINARY_SRC" "${CONTAINER}${BINARY_DEST}" --mode=755

    log "Pushing updated runner script..."
    incus file push ./scripts/dns-bench-run.sh "${CONTAINER}/usr/local/bin/dns-bench-run.sh" --mode=755

    log "Restarting dashboard service..."
    incus exec "$CONTAINER" -- systemctl start dns-bench.service

    sleep 1
    incus exec "$CONTAINER" -- systemctl is-active dns-bench.service

    IP=$(get_ip)
    log "Updated! Dashboard at: http://${IP}:8080"
}

do_destroy() {
    container_exists || die "Container '$CONTAINER' does not exist."
    log "Stopping and deleting container '$CONTAINER'..."
    incus delete "$CONTAINER" --force
    log "Container deleted."
}

do_status() {
    container_exists || die "Container '$CONTAINER' does not exist."
    echo "── Dashboard service ──────────────────────────────────────────"
    incus exec "$CONTAINER" -- systemctl status dns-bench.service --no-pager || true
    echo ""
    echo "── Benchmark timer ────────────────────────────────────────────"
    incus exec "$CONTAINER" -- systemctl status dns-bench-run.timer --no-pager || true
    echo ""
    IP=$(get_ip)
    log "Dashboard: http://${IP}:8080"
}

do_run_now() {
    container_exists || die "Container '$CONTAINER' does not exist."
    container_running || { log "Starting container..."; incus start "$CONTAINER"; sleep 2; }
    log "Triggering benchmark run..."
    incus exec "$CONTAINER" -- systemctl start dns-bench-run.service
    log "Run started. Follow logs with:"
    log "  incus exec $CONTAINER -- journalctl -u dns-bench-run.service -f"
}

do_setup_sync() {
    container_exists || die "Container '$CONTAINER' does not exist."
    setup_ssh_key
}

# ── main ──────────────────────────────────────────────────────────────────────

require_incus

case "${1:-}" in
    --update)     do_update     ;;
    --destroy)    do_destroy    ;;
    --status)     do_status     ;;
    --run-now)    do_run_now    ;;
    --setup-sync) do_setup_sync ;;
    "")           do_create     ;;
    *) echo "Usage: $0 [--update | --destroy | --status | --run-now | --setup-sync]" >&2; exit 1 ;;
esac
