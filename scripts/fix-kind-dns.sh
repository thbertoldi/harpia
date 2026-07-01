#!/usr/bin/env bash
# Idempotently point kind's CoreDNS at public resolvers so in-cluster external
# calls (e.g. api.deepseek.com used by the plan classifier, RSS feeds) work
# despite the kind node's flaky embedded resolver ("server misbehaving").
# Best-effort also adds a public nameserver to the kind node to help image pulls.
#
# Safe to run on every `tilt up`: it no-ops when already patched.
set -euo pipefail

if ! command -v kubectl >/dev/null 2>&1; then
  echo "fix-kind-dns: kubectl not found; skipping." >&2
  exit 0
fi

if ! kubectl -n kube-system get configmap coredns >/dev/null 2>&1; then
  echo "fix-kind-dns: coredns configmap not found (is the kind cluster up?); skipping." >&2
  exit 0
fi

CURRENT="$(kubectl -n kube-system get configmap coredns -o jsonpath='{.data.Corefile}')"
if printf '%s' "$CURRENT" | grep -q "forward . 1.1.1.1 8.8.8.8"; then
  echo "fix-kind-dns: CoreDNS already forwarding to public resolvers."
else
  echo "fix-kind-dns: patching CoreDNS to forward to 1.1.1.1 / 8.8.8.8..."
  printf '%s' "$CURRENT" \
    | sed 's#forward \. /etc/resolv.conf#forward . 1.1.1.1 8.8.8.8#' >/tmp/harpia-Corefile
  kubectl -n kube-system create configmap coredns \
    --from-file=Corefile=/tmp/harpia-Corefile \
    --dry-run=client -o yaml | kubectl -n kube-system apply -f - >/dev/null
  kubectl -n kube-system rollout restart deploy/coredns >/dev/null
  echo "fix-kind-dns: CoreDNS patched."
fi

# Best-effort: help node-level image pulls (kubelet uses the node's resolver,
# not CoreDNS). Works with either docker- or podman-backed kind.
NODE="${HARPIA_KIND_NODE:-kind-control-plane}"
for RUNTIME in docker podman; do
  if command -v "$RUNTIME" >/dev/null 2>&1 && "$RUNTIME" inspect "$NODE" >/dev/null 2>&1; then
    if ! "$RUNTIME" exec "$NODE" grep -q "1.1.1.1" /etc/resolv.conf 2>/dev/null; then
      if "$RUNTIME" exec "$NODE" sh -c 'echo "nameserver 1.1.1.1" >> /etc/resolv.conf' 2>/dev/null; then
        echo "fix-kind-dns: added public nameserver to node $NODE via $RUNTIME."
      fi
    fi
    break
  fi
done
