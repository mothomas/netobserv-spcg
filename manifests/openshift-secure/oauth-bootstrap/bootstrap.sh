#!/bin/sh
# Uses oc/kubectl from OCP debug tools image (openshift/tools or rhel9/support-tools).
set -eu
if command -v oc >/dev/null 2>&1; then
  kubectl() { oc "$@"; }
elif ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: oc/kubectl not found — use openshift/tools or rhel9/support-tools image" >&2
  exit 1
fi

OAUTH_CLIENT_NAME="${OAUTH_CLIENT_NAME:-spcg-ui}"
SECRET_NAME="${SECRET_NAME:-spcg-oauth-client}"
SECRET_KEY="${SECRET_KEY:-client-secret}"
GRANT_METHOD="${GRANT_METHOD:-auto}"
LANDING_NS="${LANDING_NS:-spcg-landing}"
CONTROL_NS="${CONTROL_NS:-spcg-control}"
API_ROUTE_NAME="${API_ROUTE_NAME:-spcg-api}"
UI_ROUTE_NAME="${UI_ROUTE_NAME:-spcg}"
CALLBACK_PATH="${CALLBACK_PATH:-/api/v1/auth/openshift/callback}"
ROUTE_WAIT_SECS="${ROUTE_WAIT_SECS:-300}"

kubectl_get() { kubectl get "$@" 2>/dev/null || true; }

wait_route_host() {
  ns="$1" name="$2" i=0
  while [ "$i" -lt "$ROUTE_WAIT_SECS" ]; do
    host="$(kubectl_get route "$name" -n "$ns" -o jsonpath='{.spec.host}')"
    if [ -n "$host" ]; then
      printf '%s' "$host"
      return 0
    fi
    i=$((i + 5))
    sleep 5
  done
  return 1
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

k8s_secret_val() {
  kubectl_get secret "$SECRET_NAME" -n "$CONTROL_NS" -o "jsonpath={.data.${SECRET_KEY}}" | base64 -d 2>/dev/null || true
}

ocp_secret_val() {
  kubectl_get oauthclient "$OAUTH_CLIENT_NAME" -o jsonpath='{.secret}' || true
}

apply_oauthclient() {
  sec="$1"
  cat <<EOC | kubectl apply -f -
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: ${OAUTH_CLIENT_NAME}
grantMethod: ${GRANT_METHOD}
redirectURIs:
  - ${API_REDIRECT_URI}
  - ${UI_REDIRECT_URI}
  - https://${API_HOST}/
  - https://${UI_HOST}/
secret: "${sec}"
EOC
}

apply_k8s_secret() {
  sec="$1"
  kubectl create secret generic "$SECRET_NAME" -n "$CONTROL_NS" \
    --from-literal="${SECRET_KEY}=${sec}" \
    --dry-run=client -o yaml | kubectl apply -f -
}

rollout_restart() {
  kubectl rollout restart "deployment/$1" -n "$2" >/dev/null 2>&1 || true
}

echo "Waiting for Route ${API_ROUTE_NAME} in ${CONTROL_NS}..."
API_HOST="$(wait_route_host "$CONTROL_NS" "$API_ROUTE_NAME")" || {
  echo "ERROR: Route ${API_ROUTE_NAME} not ready" >&2
  exit 1
}
echo "Waiting for Route ${UI_ROUTE_NAME} in ${LANDING_NS}..."
UI_HOST="$(wait_route_host "$LANDING_NS" "$UI_ROUTE_NAME")" || {
  echo "ERROR: Route ${UI_ROUTE_NAME} not ready" >&2
  exit 1
}

API_REDIRECT_URI="https://${API_HOST}${CALLBACK_PATH}"
UI_REDIRECT_URI="https://${UI_HOST}${CALLBACK_PATH}"
UI_ORIGIN="https://${UI_HOST}"
API_ORIGIN="https://${API_HOST}"
echo "OAuth redirect URIs:"
echo "  API (primary): ${API_REDIRECT_URI}"
echo "  UI (fallback):  ${UI_REDIRECT_URI}"

KS="$(k8s_secret_val)"
OS="$(ocp_secret_val)"
case "$KS" in
  ""|pending-bootstrap-replace-me) KS="" ;;
esac

if [ -n "$KS" ] && [ -n "$OS" ]; then
  if [ "$KS" != "$OS" ]; then
    echo "WARN: syncing ${SECRET_NAME} from OAuthClient" >&2
    apply_k8s_secret "$OS"
  fi
  apply_oauthclient "$OS"
elif [ -n "$OS" ]; then
  apply_k8s_secret "$OS"
  apply_oauthclient "$OS"
elif [ -n "$KS" ]; then
  apply_oauthclient "$KS"
else
  NEW="$(random_secret)"
  echo "Creating OAuthClient ${OAUTH_CLIENT_NAME} and secret ${SECRET_NAME} (value not logged)."
  apply_oauthclient "$NEW"
  apply_k8s_secret "$NEW"
fi

if kubectl_get cm oauth-serving-cert -n openshift-config-managed -o jsonpath='{.data.ca-bundle\.crt}' | grep -q BEGIN; then
  kubectl create configmap spcg-oauth-serving-ca -n "$CONTROL_NS" \
    --from-literal=ca-bundle.crt="$(kubectl_get cm oauth-serving-cert -n openshift-config-managed -o jsonpath='{.data.ca-bundle\.crt}')" \
    --dry-run=client -o yaml | kubectl apply -f -
fi

OAUTH_HOST="$(kubectl_get route oauth-openshift -n openshift-authentication -o jsonpath='{.spec.host}' || true)"
PORTAL_ENV="CORS_ORIGIN=${UI_ORIGIN}"
PORTAL_ENV="${PORTAL_ENV} SPCG_FRONTEND_URL=${UI_ORIGIN}"
PORTAL_ENV="${PORTAL_ENV} SPCG_PUBLIC_API_BASE=${API_ORIGIN}"
PORTAL_ENV="${PORTAL_ENV} OAUTH_REDIRECT_URL=${API_REDIRECT_URI}"
if [ -n "$OAUTH_HOST" ]; then
  OAUTH_TOKEN_URL="https://${OAUTH_HOST}/oauth/token"
  OAUTH_AUTHORIZE_URL="https://${OAUTH_HOST}/oauth/authorize"
  NO_PROXY=".cluster.local,.svc,127.0.0.1,localhost,${OAUTH_HOST}"
  PORTAL_ENV="${PORTAL_ENV} OAUTH_TOKEN_URL=${OAUTH_TOKEN_URL}"
  PORTAL_ENV="${PORTAL_ENV} OAUTH_AUTHORIZE_URL=${OAUTH_AUTHORIZE_URL}"
  PORTAL_ENV="${PORTAL_ENV} NO_PROXY=${NO_PROXY}"
fi
# shellcheck disable=SC2086
kubectl set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" ${PORTAL_ENV} || true

kubectl set env "deployment/spcg-frontend" -n "$LANDING_NS" \
  "SPCG_PUBLIC_API_BASE=${API_ORIGIN}" "SPCG_DISABLE_API_PROXY=true"

rollout_restart "spcg-ui-portal" "$CONTROL_NS"
rollout_restart "spcg-frontend" "$LANDING_NS"

echo "OAuth bootstrap complete."
