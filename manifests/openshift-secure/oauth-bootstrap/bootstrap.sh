#!/bin/sh
set -eu
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

echo "Waiting for Route ${API_ROUTE_NAME} in ${CONTROL_NS}..."
API_HOST="$(wait_route_host "$CONTROL_NS" "$API_ROUTE_NAME")" || {
  echo "ERROR: Route ${API_ROUTE_NAME} not ready in ${CONTROL_NS}" >&2
  exit 1
}
echo "Waiting for Route ${UI_ROUTE_NAME} in ${LANDING_NS}..."
UI_HOST="$(wait_route_host "$LANDING_NS" "$UI_ROUTE_NAME")" || {
  echo "ERROR: Route ${UI_ROUTE_NAME} not ready in ${LANDING_NS}" >&2
  exit 1
}

REDIRECT_URI="https://${API_HOST}${CALLBACK_PATH}"
UI_ORIGIN="https://${UI_HOST}"
API_ORIGIN="https://${API_HOST}"
echo "OAuth redirect URI: ${REDIRECT_URI}"

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

k8s_secret() {
  kubectl_get secret "$SECRET_NAME" -n "$CONTROL_NS" -o "jsonpath={.data.${SECRET_KEY}}" | base64 -d 2>/dev/null || true
}

ocp_secret() {
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
  - ${REDIRECT_URI}
secret: "${sec}"
EOC
}

apply_k8s_secret() {
  sec="$1"
  kubectl create secret generic "$SECRET_NAME" -n "$CONTROL_NS" \
    --from-literal="${SECRET_KEY}=${sec}" \
    --dry-run=client -o yaml | kubectl apply -f -
}

KS="$(k8s_secret)"
OS="$(ocp_secret)"

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
if [ -n "$OAUTH_HOST" ]; then
  OAUTH_TOKEN_URL="https://${OAUTH_HOST}/oauth/token"
  NO_PROXY=".cluster.local,.svc,127.0.0.1,localhost,${OAUTH_HOST}"
  kubectl set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" \
    "OAUTH_TOKEN_URL=${OAUTH_TOKEN_URL}" "NO_PROXY=${NO_PROXY}" || true
fi

kubectl set env "deployment/spcg-frontend" -n "$LANDING_NS" \
  "SPCG_PUBLIC_API_BASE=${API_ORIGIN}" "SPCG_DISABLE_API_PROXY=true"
kubectl set env "deployment/spcg-ui-portal" -n "$CONTROL_NS" \
  "CORS_ORIGIN=${UI_ORIGIN}"

echo "OAuth bootstrap complete."
