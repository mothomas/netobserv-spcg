#!/usr/bin/env bash
# Start lab zmap threat sim (local test; manifests/lab/ is gitignored).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export KUBECONFIG="${KUBECONFIG:-$ROOT/kubeconfig}"
if [[ ! -f "$ROOT/manifests/lab/random-scanner/kustomization.yaml" ]]; then
  echo "Run docs/examples/lab-random-scanner/setup-local.sh first" >&2
  exit 1
fi
kubectl apply -k "$ROOT/manifests/lab/random-scanner"
kubectl -n random-scanner scale deployment/continuous-network-scanner --replicas=1
kubectl -n random-scanner rollout status deployment/continuous-network-scanner --timeout=180s
echo "random-scanner running (namespace random-scanner)"
