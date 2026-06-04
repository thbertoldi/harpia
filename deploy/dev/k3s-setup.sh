#!/usr/bin/env bash
set -euo pipefail

# Harpia Development K3s Setup
# Run once to bootstrap local development cluster

echo "=== Harpia K3s Dev Setup ==="

# Check prerequisites
for cmd in k3s podman kubectl; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: $cmd not found. Please install it first."
        exit 1
    fi
done

echo "[1/3] Starting K3s with podman..."
if ! k3s check-config 2>/dev/null; then
    sudo k3s server --docker --write-kubeconfig-mode 644 &
    sleep 5
fi

echo "[2/3] Setting up kubeconfig..."
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

echo "[3/3] Waiting for node..."
kubectl wait --for=condition=Ready node --all --timeout=60s

echo ""
echo "=== K3s is ready ==="
echo "Run 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' to use kubectl"
echo "Then: mise run dev  (starts Tilt)"
