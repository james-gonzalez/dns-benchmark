#!/usr/bin/env bash
# sync-to-k8s.sh — copy dns-bench results into the Longhorn PVC backing dns-benchmark.lan
# Deploy this script to the K3s node: ~/sync-to-k8s.sh
# Called remotely by the Incus container after each benchmark run (tar piped via SSH stdin).
set -euo pipefail

# Stable globalmount path for the dns-bench-results PVC (Longhorn RWX)
PVC_MOUNT="/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/0e6bff408bb2d7b383e52d100074e685ff2237a65f21faea7777659e23fcfe71/globalmount"

STAGING_DIR=$(mktemp -d)
cleanup() { rm -rf "$STAGING_DIR"; }
trap cleanup EXIT

# Unpack the tar from stdin into staging dir
tar -xf - -C "$STAGING_DIR"

echo "==> Syncing results to PVC mount..."
sudo rsync -a --exclude='lost+found' "$STAGING_DIR/" "$PVC_MOUNT/"

COUNT=$(find "$STAGING_DIR" -maxdepth 1 -type f | wc -l | tr -d ' ')
echo "==> Sync complete ($COUNT files). dns-benchmark.lan updated."
