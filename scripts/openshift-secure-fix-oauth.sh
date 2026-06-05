#!/usr/bin/env bash
# Fix OpenShift OAuth invalid_request for manifests/openshift-secure (3-namespace layout).
# Registers OAuthClient spcg-ui redirect URIs from live Routes and syncs portal env.
set -euo pipefail

LANDING_NS="${LANDING_NS:-spcg-landing}"
CONTROL_NS="${CONTROL_NS:-spcg-control}"
CLIENT_ID="${CLIENT_ID:-spcg-ui}"
SECRET_NAME="${SECRET_NAME:-spcg-oauth-client}"
CALLBACK_PATH="${CALLBACK_PATH:-/api/v1/auth/openshift/callback}"

UI_HOST="$(oc get route spcg -n "$LANDING_NS" -o jsonpath='{.spec.host}')"
API_HOST="$(oc get route spcg-api -n "$CONTROL_NS" -o jsonpath='{.spec.host}' 2>/dev/null || true)"
[ -n "$UI_HOST" ] || { echo "ERROR: Route spcg not found in ${LANDING_NS}" >&2; exit 1; }
[ -n "$API_HOST" ] || API_HOST="$UI_HOST"

UI_ORIGIN="https://${UI_HOST}"
API_ORIGIN="https://${API_HOST}"
UI_REDIRECT="${UI_ORIGIN}${CALLBACK_PATH}"
API_REDIRECT="${API_ORIGIN}${CALLBACK_PATH}"

echo "UI Route:  ${UI_ORIGIN}"
echo "API Route: ${API_ORIGIN}"
echo "OAuth redirect (primary): ${UI_REDIRECT}"

if oc get oauthclient "$CLIENT_ID" -o name >/dev/null 2>&1; then
  echo ""
  echo "Current OAuthClient redirectURIs:"
  oc get oauthclient "$CLIENT_ID" -o jsonpath='{range .redirectURIs[*]}{.}{"\n"}{end}' || true
  SECRET="$(oc get oauthclient "$CLIENT_ID" -o jsonpath='{.secret}')"
else
  echo "OAuthClient ${CLIENT_ID} not found — creating new secret"
  SECRET="$(openssl rand -hex 16)"
fi

echo ""
echo "Applying OAuthClient ${CLIENT_ID}..."
oc apply -f - <<EOF
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: ${CLIENT_ID}
grantMethod: auto
redirectURIs:
  - ${UI_REDIRECT}
  - ${API_REDIRECT}
  - ${UI_ORIGIN}/
  - ${API_ORIGIN}/
secret: "${SECRET}"
EOF

oc create secret generic "$SECRET_NAME" -n "$CONTROL_NS" \
  --from-literal=client-secret="${SECRET}" \
  --dry-run=client -o yaml | oc apply -f -

K8S_SECRET="$(oc get secret "$SECRET_NAME" -n "$CONTROL_NS" -o jsonpath='{.data.client-secret}' 2>/dev/null | base64 -d 2>/dev/null || true)"
OC_SECRET="$(oc get oauthclient "$CLIENT_ID" -o jsonpath='{.secret}' 2>/dev/null || true)"
if [ -n "$K8S_SECRET" ] && [ -n "$OC_SECRET" ] && [ "$K8S_SECRET" != "$OC_SECRET" ]; then
  echo "WARN: syncing ${SECRET_NAME} from OAuthClient (was out of sync)" >&2
  oc create secret generic "$SECRET_NAME" -n "$CONTROL_NS" \
    --from-literal=client-secret="${OC_SECRET}" \
    --dry-run=client -o yaml | oc apply -f -
fi

OAUTH_HOST="$(oc get route oauth-openshift -n openshift-authentication -o jsonpath='{.spec.host}' 2>/dev/null || true)"
PORTAL_ENV=(
  "SPCG_AUTH_METHODS=openshift,kubeconfig"
  "OAUTH_CLIENT_ID=${CLIENT_ID}"
  "SPCG_FRONTEND_URL=${UI_ORIGIN}"
  "SPCG_PUBLIC_API_BASE=${API_ORIGIN}"
  "OAUTH_REDIRECT_URL=${UI_REDIRECT}"
  "CORS_ORIGIN=${UI_ORIGIN}"
  "OAUTH_TLS_INSECURE_SKIP_VERIFY=true"
)
if [ -n "$OAUTH_HOST" ]; then
  PORTAL_ENV+=(
    "OAUTH_AUTHORIZE_URL=https://${OAUTH_HOST}/oauth/authorize"
    "OAUTH_TOKEN_URL=https://${OAUTH_HOST}/oauth/token"
    "NO_PROXY=.cluster.local,.svc,127.0.0.1,localhost,${OAUTH_HOST}"
  )
fi

oc set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" "${PORTAL_ENV[@]}"
oc set env "deployment/spcg-frontend" -n "$LANDING_NS" \
  "SPCG_AUTH_METHODS=openshift,kubeconfig" \
  "SPCG_PUBLIC_API_BASE-" \
  "SPCG_DISABLE_API_PROXY=true"

oc rollout restart "deployment/spcg-ui-portal" -n "$CONTROL_NS"
oc rollout restart "deployment/spcg-frontend" -n "$LANDING_NS"
oc rollout status "deployment/spcg-ui-portal" -n "$CONTROL_NS" --timeout=5m
oc rollout status "deployment/spcg-frontend" -n "$LANDING_NS" --timeout=5m

echo ""
echo "=== auth/config (with public host) ==="
oc exec -n "$CONTROL_NS" "deployment/spcg-ui-portal" -- wget -qO- \
  --header="X-Forwarded-Host: ${UI_HOST}" \
  --header="X-Forwarded-Proto: https" \
  "http://127.0.0.1:8080/api/v1/auth/config" || true
echo ""
echo "Done. Hard-refresh ${UI_ORIGIN} and retry OpenShift login."
