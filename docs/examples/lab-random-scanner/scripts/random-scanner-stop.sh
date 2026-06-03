#!/usr/bin/env bash
# Stop lab zmap threat sim without removing manifests from the cluster.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-$ROOT/kubeconfig}"
kubectl -n random-scanner scale deployment/continuous-network-scanner --replicas=0 --ignore-not-found
echo "random-scanner scaled to 0"
