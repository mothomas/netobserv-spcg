#!/usr/bin/env bash
# Argo CD–style OAuth bootstrap: generate client secret, register OAuthClient, create namespace secret.
# Cluster-admin (or oauthclient create + secret write) required once. Admin never copies the secret.
set -euo pipefail

OAUTH_CLIENT_NAME="${OAUTH_CLIENT_NAME:-spcg-ui}"
SECRET_NAME="${SECRET_NAME:-spcg-oauth-client}"
SECRET_KEY="${SECRET_KEY:-client-secret}"
GRANT_METHOD="${GRANT_METHOD:-auto}"

# Layout: secure = spcg-api in spcg-control; small = spcg in pcap-frontend (same-origin callback)
LAYOUT="${SPCG_OAUTH_LAYOUT:-secure}"
LANDING_NS="${LANDING_NS:-spcg-landing}"
CONTROL_NS="${CONTROL_NS:-spcg-control}"
FRONTEND_NS="${FRONTEND_NS:-pcap-frontend}"

case "$LAYOUT" in
  secure)
    ROUTE_NS="$CONTROL_NS"
    ROUTE_NAME="spcg-api"
    CALLBACK_PATH="/api/v1/auth/openshift/callback"
    SECRET_NS="$CONTROL_NS"
    ;;
  small|monolithic)
    ROUTE_NS="$FRONTEND_NS"
    ROUTE_NAME="spcg"
    CALLBACK_PATH="/api/v1/auth/openshift/callback"
    SECRET_NS="$FRONTEND_NS"
    ;;
  *)
    echo "ERROR: SPCG_OAUTH_LAYOUT must be secure or small (got: $LAYOUT)" >&2
    exit 1
    ;;
esac

route_host() {
  oc get route "$ROUTE_NAME" -n "$ROUTE_NS" -o jsonpath='{.spec.host}' 2>/dev/null || true
}

HOST="$(route_host)"
if [ -z "$HOST" ]; then
  echo "ERROR: Route ${ROUTE_NAME} not found in ${ROUTE_NS}. Apply manifests first." >&2
  exit 1
fi
REDIRECT_URI="https://${HOST}${CALLBACK_PATH}"
echo "OAuth redirect URI: ${REDIRECT_URI}"

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

client_secret_from_oauthclient() {
  oc get oauthclient "$OAUTH_CLIENT_NAME" -o jsonpath='{.secret}' 2>/dev/null || true
}

client_secret_from_k8s() {
  oc get secret "$SECRET_NAME" -n "$SECRET_NS" -o jsonpath="{.data.${SECRET_KEY}}" 2>/dev/null \
    | base64 -d 2>/dev/null || true
}

apply_oauthclient() {
  local secret="$1"
  oc apply -f - <<EOF
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: ${OAUTH_CLIENT_NAME}
grantMethod: ${GRANT_METHOD}
redirectURIs:
  - ${REDIRECT_URI}
secret: "${secret}"
EOF
}

apply_k8s_secret() {
  local secret="$1"
  oc create secret generic "$SECRET_NAME" -n "$SECRET_NS" \
    --from-literal="${SECRET_KEY}=${secret}" \
    --dry-run=client -o yaml | oc apply -f -
}

K8S_SECRET="$(client_secret_from_k8s)"
OCP_SECRET="$(client_secret_from_oauthclient)"

if [ -n "$K8S_SECRET" ] && [ -n "$OCP_SECRET" ]; then
  if [ "$K8S_SECRET" = "$OCP_SECRET" ]; then
    echo "OAuthClient and ${SECRET_NS}/${SECRET_NAME} already aligned; updating redirect URI only."
    apply_oauthclient "$OCP_SECRET"
    exit 0
  fi
  echo "WARN: ${SECRET_NAME} and OAuthClient secrets differ; keeping OAuthClient and re-syncing K8s secret." >&2
  apply_k8s_secret "$OCP_SECRET"
  apply_oauthclient "$OCP_SECRET"
  exit 0
fi

if [ -n "$OCP_SECRET" ]; then
  echo "Creating ${SECRET_NS}/${SECRET_NAME} from existing OAuthClient ${OAUTH_CLIENT_NAME}."
  apply_k8s_secret "$OCP_SECRET"
  apply_oauthclient "$OCP_SECRET"
  exit 0
fi

if [ -n "$K8S_SECRET" ]; then
  echo "Registering OAuthClient ${OAUTH_CLIENT_NAME} from existing ${SECRET_NS}/${SECRET_NAME}."
  apply_oauthclient "$K8S_SECRET"
  exit 0
fi

NEW_SECRET="$(random_secret)"
echo "Registering new OAuthClient ${OAUTH_CLIENT_NAME} and ${SECRET_NS}/${SECRET_NAME} (generated secret; not printed)."
apply_oauthclient "$NEW_SECRET"
apply_k8s_secret "$NEW_SECRET"
