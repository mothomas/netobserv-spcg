#!/usr/bin/env bash
# Remove random-scanner namespace entirely.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-$ROOT/kubeconfig}"
kubectl delete namespace random-scanner --ignore-not-found --wait=true --timeout=120s
echo "random-scanner namespace deleted"
