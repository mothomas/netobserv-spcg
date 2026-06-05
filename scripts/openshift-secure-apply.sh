#!/usr/bin/env bash
# Apply openshift-secure overlay and set Route-derived URLs (run from repo root).
set -euo pipefail
LANDING_NS="${LANDING_NS:-spcg-landing}"
CONTROL_NS="${CONTROL_NS:-spcg-control}"
PORTAL_IMAGE="${PORTAL_IMAGE:-quay.io/moby/spcg-ui-portal:small-20260616}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-quay.io/moby/spcg-frontend:small-20260616}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Applying greenfield openshift-secure manifests..."
oc apply -k "${REPO_ROOT}/manifests/openshift-secure"

echo "Bootstrapping OAuth (Argo CD–style: auto OAuthClient + secret)..."
if ! SPCG_OAUTH_LAYOUT=secure "${REPO_ROOT}/scripts/openshift-oauth-bootstrap.sh"; then
  echo "WARN: OAuth bootstrap failed (need cluster-admin for oauthclients?)." >&2
  echo "  Retry: SPCG_OAUTH_LAYOUT=secure ${REPO_ROOT}/scripts/openshift-oauth-bootstrap.sh" >&2
fi

UI_HOST="$(oc get route spcg -n "$LANDING_NS" -o jsonpath='https://{.spec.host}' 2>/dev/null || true)"
API_HOST="$(oc get route spcg-api -n "$CONTROL_NS" -o jsonpath='https://{.spec.host}' 2>/dev/null || true)"
if [ -z "$UI_HOST" ] || [ -z "$API_HOST" ]; then
  echo "WARN: could not read Route hosts; set SPCG_PUBLIC_API_BASE and CORS_ORIGIN manually after Routes exist"
else
  echo "UI route:  ${UI_HOST}"
  echo "API route: ${API_HOST}"
  oc set env "deployment/spcg-frontend" -n "$LANDING_NS" \
    "SPCG_PUBLIC_API_BASE=${API_HOST}" "SPCG_DISABLE_API_PROXY=true"
  oc set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" \
    "CORS_ORIGIN=${UI_HOST}"
fi

echo "Syncing oauth serving CA to ${CONTROL_NS}/spcg-oauth-serving-ca (optional TLS trust)..."
if oc get cm oauth-serving-cert -n openshift-config-managed -o jsonpath='{.data.ca-bundle\.crt}' 2>/dev/null | grep -q BEGIN; then
  oc create configmap spcg-oauth-serving-ca -n "$CONTROL_NS" \
    --from-literal=ca-bundle.crt="$(oc get cm oauth-serving-cert -n openshift-config-managed -o jsonpath='{.data.ca-bundle\.crt}')" \
    --dry-run=client -o yaml | oc apply -f -
fi

OAUTH_TOKEN_URL="$(oc get route oauth-openshift -n openshift-authentication -o jsonpath='https://{.spec.host}/oauth/token' 2>/dev/null || true)"
OAUTH_HOST="$(oc get route oauth-openshift -n openshift-authentication -o jsonpath='{.spec.host}' 2>/dev/null || true)"
CLUSTER_DOMAIN="$(oc get ingresses.config/cluster -o jsonpath='{.spec.domain}' 2>/dev/null || true)"
NO_PROXY=".cluster.local,.svc,127.0.0.1,localhost"
[ -n "$OAUTH_HOST" ] && NO_PROXY="${NO_PROXY},${OAUTH_HOST}"
[ -n "$CLUSTER_DOMAIN" ] && NO_PROXY="${NO_PROXY},.${CLUSTER_DOMAIN}"

oc set image "deployment/spcg-ui-portal" -n "$CONTROL_NS" "ui-portal=${PORTAL_IMAGE}"
oc set image "deployment/spcg-frontend" -n "$LANDING_NS" "frontend=${FRONTEND_IMAGE}"
if [ -n "$OAUTH_TOKEN_URL" ] && [ "$OAUTH_TOKEN_URL" != "https:///oauth/token" ]; then
  oc set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" \
    "OAUTH_TOKEN_URL=${OAUTH_TOKEN_URL}" "NO_PROXY=${NO_PROXY}"
else
  oc set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" "NO_PROXY=${NO_PROXY}"
fi

echo "Restarting workloads..."
oc rollout restart "deployment/spcg-ui-portal" -n "$CONTROL_NS"
oc rollout restart "deployment/spcg-frontend" -n "$LANDING_NS"
oc rollout status "deployment/spcg-ui-portal" -n "$CONTROL_NS" --timeout=5m
oc rollout status "deployment/spcg-frontend" -n "$LANDING_NS" --timeout=5m

echo ""
echo "=== OAuth redirect URI (OAuthClient spcg-ui — created by bootstrap) ==="
if [ -n "$API_HOST" ]; then
  echo "${API_HOST}/api/v1/auth/openshift/callback"
fi
echo ""
echo "=== Portal auth/config ==="
oc exec -n "$CONTROL_NS" "deployment/spcg-ui-portal" -- wget -qO- "http://127.0.0.1:8080/api/v1/auth/config" || true
