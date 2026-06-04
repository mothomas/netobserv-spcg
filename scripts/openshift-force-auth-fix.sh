#!/usr/bin/env bash
# Apply OpenShift auth ConfigMap, images, and roll out (run from repo root).
set -euo pipefail
NS="${NS:-pcap-frontend}"
PORTAL_IMAGE="${PORTAL_IMAGE:-quay.io/moby/spcg-ui-portal:small-20260624}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-quay.io/moby/spcg-frontend:small-20260624}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if ! oc get secret spcg-quay -n "$NS" &>/dev/null; then
  echo "WARN: secret spcg-quay not found — create before pull (see docs/openshift-quay-images.md)"
fi

echo "Applying openshift-small overlay..."
oc apply -k "${REPO_ROOT}/manifests/overlays/openshift-small"

echo "Ensuring spcg-auth-env ConfigMap..."
oc apply -f "${REPO_ROOT}/manifests/openshift/config-auth-openshift.yaml"

echo "Setting images..."
oc set image "deployment/spcg-ui-portal" -n "$NS" "ui-portal=${PORTAL_IMAGE}"
oc set image "deployment/spcg-frontend" -n "$NS" "frontend=${FRONTEND_IMAGE}"

echo "Setting auth env (in case patches were skipped)..."
oc set env "deployment/spcg-frontend" -n "$NS" "SPCG_AUTH_METHODS=openshift"
oc set env "deployment/spcg-ui-portal" -n "$NS" \
  "SPCG_AUTH_METHODS=openshift" "OAUTH_CLIENT_ID=spcg-ui"

echo "Restarting (delete stuck pods if deployment UP-TO-DATE is 0)..."
oc rollout restart "deployment/spcg-ui-portal" "deployment/spcg-frontend" -n "$NS"
# Old ReplicaSets can leave a running pod on an older tag (e.g. small-20260614) while spec shows 20260622.
sleep 3
STUCK="$(oc get pods -n "$NS" -l app=spcg-ui-portal -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.containers[0].image}{"\n"}{end}' | grep -v "${PORTAL_IMAGE}$" || true)"
if [ -n "$STUCK" ]; then
  echo "Removing portal pod(s) not on ${PORTAL_IMAGE}:"
  echo "$STUCK"
  oc get pods -n "$NS" -l app=spcg-ui-portal -o name | while read -r p; do
    img="$(oc get "$p" -n "$NS" -o jsonpath='{.spec.containers[0].image}')"
    if [ "$img" != "$PORTAL_IMAGE" ]; then
      oc delete "$p" -n "$NS" --wait=true
    fi
  done
fi
oc rollout status "deployment/spcg-ui-portal" -n "$NS" --timeout=5m
oc rollout status "deployment/spcg-frontend" -n "$NS" --timeout=5m

echo ""
echo "=== Portal image on running pod (must match ${PORTAL_IMAGE}) ==="
oc get pods -n "$NS" -l app=spcg-ui-portal \
  -o custom-columns=NAME:.metadata.name,IMAGE:.spec.containers[0].image,READY:.status.containerStatuses[0].ready

echo ""
echo "=== Portal auth/config ==="
oc exec -n "$NS" "deployment/spcg-ui-portal" -- wget -qO- "http://127.0.0.1:8080/api/v1/auth/config" || true
echo ""
echo "=== Deployments ==="
oc get deploy spcg-ui-portal spcg-frontend -n "$NS" \
  -o custom-columns=NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image
