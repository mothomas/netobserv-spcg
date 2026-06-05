#!/usr/bin/env bash
# Deprecated: use manifests/openshift-secure (Job-based bootstrap) directly.
#   oc apply -k manifests/openshift-secure
#   oc wait --for=condition=complete job/spcg-oauth-bootstrap -n spcg-control --timeout=15m
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
echo "Applying manifests/openshift-secure (OAuth bootstrap Job included)..." >&2
oc apply -k "${REPO_ROOT}/manifests/openshift-secure"
echo "Waiting for Job spcg-oauth-bootstrap..." >&2
oc wait --for=condition=complete "job/spcg-oauth-bootstrap" -n spcg-control --timeout=15m
oc logs "job/spcg-oauth-bootstrap" -n spcg-control --tail=50
