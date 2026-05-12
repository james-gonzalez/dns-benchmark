#!/usr/bin/env bash
# setup-remote-host.sh — install and manage dns-bench on a remote Linux host
#
# Usage:
#   ./scripts/setup-remote-host.sh --host <user@host> [options]
#
# Actions (default: --install):
#   --install     Install binary, config, CA cert, systemd timer, run once immediately
#   --update      Push new binary + updated config, restart timer
#   --run-now     Trigger a benchmark run immediately
#   --status      Show service/timer status and last run logs
#   --uninstall   Stop and remove everything installed by this script
#
# Options:
#   --host        SSH target, e.g. jamesgonzalez@192.168.86.90  (required)
#   --source      Source label shown in dashboard (default: remote hostname)
#   --push-url    URL to submit results to (default: https://dns-benchmark.lan/api/submit-results)
#   --api-key     DNS_BENCH_API_KEY value for authenticated push
#   --arch        Binary arch: amd64 or arm64 (default: auto-detect via uname -m)
#   --servers     Local servers.yaml to upload (default: ./servers.yaml)
#   --domains     Local domains file to upload (default: ./hostnames.txt)
#   --schedule    Systemd OnUnitActiveSec interval (default: 6h)
#
# Examples:
#   ./scripts/setup-remote-host.sh --host user@192.168.1.10 --source raspi --api-key <key>
#   ./scripts/setup-remote-host.sh --host user@192.168.1.20 --source dietpi --api-key <key>
#   ./scripts/setup-remote-host.sh --host user@192.168.1.10 --update
#   ./scripts/setup-remote-host.sh --host user@192.168.1.10 --run-now

set -euo pipefail

# ── defaults ─────────────────────────────────────────────────────────────────

HOST=""
SOURCE=""
PUSH_URL="https://dns-benchmark.lan/api/submit-results"
API_KEY=""
ARCH=""
SERVERS_FILE="./servers.yaml"
DOMAINS_FILE="./hostnames.txt"
SCHEDULE="6h"
ACTION="install"

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/dns-bench"
SERVICE_NAME="dns-bench-run"

# ── helpers ───────────────────────────────────────────────────────────────────

log()  { echo "==> $*"; }
die()  { echo "ERROR: $*" >&2; exit 1; }

ssh_run() { ssh -o StrictHostKeyChecking=no "$HOST" "$@"; }

# Push a local file to a remote path that requires sudo by staging via /tmp
push_file() {
    local src="$1" dest="$2" mode="${3:-644}"
    local tmp="/tmp/dns-bench-upload-$$"
    ssh -o StrictHostKeyChecking=no "$HOST" "cat > ${tmp}" < "$src"
    ssh_run "sudo mv ${tmp} ${dest} && sudo chmod ${mode} ${dest}"
}

require_host() { [ -n "$HOST" ] || die "--host is required"; }

detect_arch() {
    local machine
    machine=$(ssh_run "uname -m")
    case "$machine" in
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        armv7l)  echo "arm"   ;;
        *) die "Unsupported architecture: $machine" ;;
    esac
}

find_binary() {
    local arch="$1"
    local candidate="./dns-bench-linux-${arch}"
    if [ ! -f "$candidate" ]; then
        log "Binary $candidate not found — building..."
        CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -o "$candidate" .
    fi
    echo "$candidate"
}

resolve_source() {
    if [ -n "$SOURCE" ]; then
        echo "$SOURCE"
    else
        ssh_run "hostname -s"
    fi
}

# Verify .lan DNS resolves on the remote host; prepend a working nameserver if not.
ensure_lan_dns() {
    local push_host
    push_host=$(echo "$PUSH_URL" | sed -E 's|https?://([^/:]+).*|\1|')

    log "Checking .lan DNS resolution on remote host..."
    if ssh_run "getent hosts ${push_host}" > /dev/null 2>&1; then
        log "${push_host} resolves OK"
        return
    fi

    log "${push_host} does not resolve — fixing /etc/resolv.conf..."

    # Find a working LAN nameserver from our local perspective
    local lan_ns
    lan_ns=$(dig +short "$push_host" | grep -E '^[0-9]+\.' | head -1)
    [ -n "$lan_ns" ] || die "Cannot resolve ${push_host} locally either — is Technitium running?"

    # Find which nameserver answered
    lan_ns=$(dig +short "$push_host" 2>/dev/null | grep -E '^[0-9]+\.' | head -1)
    # Get the authoritative server from our local config
    local local_ns
    local_ns=$(grep '^nameserver' /etc/resolv.conf | awk '{print $2}' | head -1)

    log "Prepending nameserver ${local_ns} to remote /etc/resolv.conf"
    ssh_run "sudo sed -i '1s/^/nameserver ${local_ns}\n/' /etc/resolv.conf"

    ssh_run "getent hosts ${push_host}" > /dev/null 2>&1 || \
        die "${push_host} still does not resolve after DNS fix — check nameserver config"
    log "${push_host} now resolves OK"
}

# Install the Home LAN CA root cert so the remote host trusts https://*.lan
install_ca_cert() {
    log "Installing Home LAN CA root certificate..."

    # Fetch the root CA from the cluster
    local ca_pem
    ca_pem=$(kubectl get configmap -n step-ca step-ca-step-certificates-certs \
        -o jsonpath='{.data.root_ca\.crt}')

    [ -n "$ca_pem" ] || die "Could not fetch root CA from cluster"

    # Write to /tmp locally, push to remote, install into system trust store
    local tmp_ca
    tmp_ca=$(mktemp /tmp/home-lan-root-ca-XXXX.crt)
    echo "$ca_pem" > "$tmp_ca"

    push_file "$tmp_ca" "/usr/local/share/ca-certificates/home-lan-root-ca.crt" "644"
    rm -f "$tmp_ca"

    ssh_run "sudo update-ca-certificates"
    log "CA cert installed — ${PUSH_URL%%/api*} is now trusted"
}

write_service_unit() {
    local source="$1"
    local env_line=""
    [ -n "$API_KEY" ] && env_line="Environment=DNS_BENCH_API_KEY=${API_KEY}"

    ssh_run "sudo tee /etc/systemd/system/${SERVICE_NAME}.service > /dev/null" <<EOF
[Unit]
Description=DNS Benchmark Run
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
${env_line}
ExecStart=${INSTALL_DIR}/dns-bench-run.sh
StandardOutput=journal
StandardError=journal
EOF
}

write_run_script() {
    local source="$1"
    ssh_run "sudo tee ${INSTALL_DIR}/dns-bench-run.sh > /dev/null" <<EOF
#!/bin/sh
set -e
${INSTALL_DIR}/dns-bench \\
  -servers=${CONFIG_DIR}/servers.yaml \\
  -domains=${CONFIG_DIR}/domains.txt \\
  -c=20 \\
  -n=3 \\
  -t=3s \\
  -source=${source} \\
  -push=${PUSH_URL}
EOF
    ssh_run "sudo chmod 755 ${INSTALL_DIR}/dns-bench-run.sh"
}

# ── actions ───────────────────────────────────────────────────────────────────

do_install() {
    [ -f "$SERVERS_FILE" ] || die "Servers file not found: $SERVERS_FILE"
    [ -f "$DOMAINS_FILE" ] || die "Domains file not found: $DOMAINS_FILE"

    [ -z "$ARCH" ] && ARCH=$(detect_arch)
    local binary
    binary=$(find_binary "$ARCH")
    local source
    source=$(resolve_source)

    log "Installing dns-bench on $HOST (arch=$ARCH, source=$source)"

    log "Creating config directory..."
    ssh_run "sudo mkdir -p ${CONFIG_DIR}"

    log "Pushing binary..."
    push_file "$binary" "${INSTALL_DIR}/dns-bench" "755"

    log "Pushing config files..."
    push_file "$SERVERS_FILE" "${CONFIG_DIR}/servers.yaml"
    push_file "$DOMAINS_FILE"  "${CONFIG_DIR}/domains.txt"

    ensure_lan_dns
    install_ca_cert

    log "Writing run script..."
    write_run_script "$source"

    log "Installing systemd units..."
    write_service_unit "$source"

    ssh_run "sudo tee /etc/systemd/system/${SERVICE_NAME}.timer > /dev/null" <<EOF
[Unit]
Description=DNS Benchmark Timer
Requires=${SERVICE_NAME}.service

[Timer]
OnBootSec=2min
OnUnitActiveSec=${SCHEDULE}
Persistent=true

[Install]
WantedBy=timers.target
EOF

    log "Enabling and starting timer..."
    ssh_run "sudo systemctl daemon-reload && sudo systemctl enable --now ${SERVICE_NAME}.timer"

    log "Triggering initial run..."
    ssh_run "sudo systemctl start ${SERVICE_NAME}.service"

    log ""
    log "Done! dns-bench is installed on $HOST"
    log "  Source label:  $source"
    log "  Push URL:      $PUSH_URL"
    log "  Schedule:      every $SCHEDULE"
    log "  Logs:          ssh $HOST sudo journalctl -u ${SERVICE_NAME}.service -f"
}

do_update() {
    [ -z "$ARCH" ] && ARCH=$(detect_arch)
    local binary
    binary=$(find_binary "$ARCH")
    local source
    source=$(resolve_source)

    log "Updating dns-bench on $HOST..."

    log "Pushing binary..."
    push_file "$binary" "${INSTALL_DIR}/dns-bench" "755"

    if [ -f "$SERVERS_FILE" ]; then
        log "Pushing updated servers.yaml..."
        push_file "$SERVERS_FILE" "${CONFIG_DIR}/servers.yaml"
    fi

    if [ -f "$DOMAINS_FILE" ]; then
        log "Pushing updated domains.txt..."
        push_file "$DOMAINS_FILE" "${CONFIG_DIR}/domains.txt"
    fi

    log "Updating run script and service unit..."
    write_run_script "$source"
    write_service_unit "$source"

    log "Reloading systemd..."
    ssh_run "sudo systemctl daemon-reload && sudo systemctl restart ${SERVICE_NAME}.timer"

    log "Updated! Next scheduled run in $SCHEDULE"
}

do_run_now() {
    log "Triggering benchmark run on $HOST..."
    ssh_run "sudo systemctl start ${SERVICE_NAME}.service"
    log "Run started. Follow logs with:"
    log "  ssh $HOST sudo journalctl -u ${SERVICE_NAME}.service -f"
}

do_status() {
    log "── Timer ──────────────────────────────────────────────────────"
    ssh_run "sudo systemctl status ${SERVICE_NAME}.timer --no-pager" || true
    log ""
    log "── Last run ───────────────────────────────────────────────────"
    ssh_run "sudo journalctl -u ${SERVICE_NAME}.service -n 30 --no-pager" || true
}

do_uninstall() {
    log "Uninstalling dns-bench from $HOST..."
    ssh_run "sudo systemctl disable --now ${SERVICE_NAME}.timer 2>/dev/null || true"
    ssh_run "sudo systemctl disable --now ${SERVICE_NAME}.service 2>/dev/null || true"
    ssh_run "sudo rm -f \
        /etc/systemd/system/${SERVICE_NAME}.service \
        /etc/systemd/system/${SERVICE_NAME}.timer \
        ${INSTALL_DIR}/dns-bench \
        ${INSTALL_DIR}/dns-bench-run.sh \
        /usr/local/share/ca-certificates/home-lan-root-ca.crt"
    ssh_run "sudo rm -rf ${CONFIG_DIR}"
    ssh_run "sudo systemctl daemon-reload && sudo update-ca-certificates"
    log "Uninstalled."
}

# ── argument parsing ──────────────────────────────────────────────────────────

while [ $# -gt 0 ]; do
    case "$1" in
        --host)      HOST="$2";         shift 2 ;;
        --source)    SOURCE="$2";       shift 2 ;;
        --push-url)  PUSH_URL="$2";     shift 2 ;;
        --api-key)   API_KEY="$2";      shift 2 ;;
        --arch)      ARCH="$2";         shift 2 ;;
        --servers)   SERVERS_FILE="$2"; shift 2 ;;
        --domains)   DOMAINS_FILE="$2"; shift 2 ;;
        --schedule)  SCHEDULE="$2";     shift 2 ;;
        --install)   ACTION="install";  shift ;;
        --update)    ACTION="update";   shift ;;
        --run-now)   ACTION="run-now";  shift ;;
        --status)    ACTION="status";   shift ;;
        --uninstall) ACTION="uninstall"; shift ;;
        *) die "Unknown option: $1" ;;
    esac
done

require_host

case "$ACTION" in
    install)   do_install   ;;
    update)    do_update    ;;
    run-now)   do_run_now   ;;
    status)    do_status    ;;
    uninstall) do_uninstall ;;
esac
