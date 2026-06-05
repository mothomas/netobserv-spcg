#!/bin/sh
# Minimal debug image (ubi-minimal): no oc/kubectl — in-cluster API via curl only.
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

API_SERVER="${KUBERNETES_SERVICE_HOST:-kubernetes.default.svc}"
API_PORT="${KUBERNETES_SERVICE_PORT:-443}"
API_BASE="https://${API_SERVER}:${API_PORT}"
TOKEN="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
CACERT="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

http_code() {
  curl -sS -o /dev/null -w "%{http_code}" --cacert "$CACERT" \
    -H "Authorization: Bearer ${TOKEN}" "$@"
}

api_get() {
  curl -sfS --cacert "$CACERT" -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/json" "${API_BASE}$1" 2>/dev/null || true
}

api_json() {
  method="$1"
  path="$2"
  body="$3"
  ctype="${4:-application/json}"
  curl -sfS --cacert "$CACERT" -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: ${ctype}" -X "$method" -d "$body" "${API_BASE}${path}"
}

json_field() {
  printf '%s' "$1" | sed -n "s/.*\"$2\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" | head -1
}

b64dec() {
  printf '%s' "$1" | base64 -d 2>/dev/null || true
}

wait_route_host() {
  ns="$1" name="$2" i=0
  while [ "$i" -lt "$ROUTE_WAIT_SECS" ]; do
    host="$(json_field "$(api_get "/apis/route.openshift.io/v1/namespaces/${ns}/routes/${name}")" host)"
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
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
}

k8s_secret_val() {
  body="$(api_get "/api/v1/namespaces/${CONTROL_NS}/secrets/${SECRET_NAME}")"
  b64="$(printf '%s' "$body" | sed -n 's/.*"client-secret":"\([^"]*\)".*/\1/p' | head -1)"
  b64dec "$b64"
}

ocp_secret_val() {
  json_field "$(api_get "/apis/oauth.openshift.io/v1/oauthclients/${OAUTH_CLIENT_NAME}")" secret
}

apply_oauthclient() {
  sec="$1"
  body="$(printf '{"apiVersion":"oauth.openshift.io/v1","kind":"OAuthClient","metadata":{"name":"%s"},"grantMethod":"%s","redirectURIs":["%s"],"secret":"%s"}' \
    "$OAUTH_CLIENT_NAME" "$GRANT_METHOD" "$REDIRECT_URI" "$sec")"
  code="$(http_code -X PUT -H "Content-Type: application/json" -d "$body" \
    "${API_BASE}/apis/oauth.openshift.io/v1/oauthclients/${OAUTH_CLIENT_NAME}")"
  if [ "$code" = "404" ]; then
    api_json POST "/apis/oauth.openshift.io/v1/oauthclients" "$body"
  elif [ "$code" != "200" ] && [ "$code" != "201" ]; then
    echo "ERROR: OAuthClient apply HTTP ${code}" >&2
    exit 1
  fi
}

apply_k8s_secret() {
  sec="$1"
  body="$(printf '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"%s","namespace":"%s"},"type":"Opaque","stringData":{"%s":"%s"}}' \
    "$SECRET_NAME" "$CONTROL_NS" "$SECRET_KEY" "$sec")"
  code="$(http_code -X PUT -H "Content-Type: application/json" -d "$body" \
    "${API_BASE}/api/v1/namespaces/${CONTROL_NS}/secrets/${SECRET_NAME}")"
  if [ "$code" = "404" ]; then
    api_json POST "/api/v1/namespaces/${CONTROL_NS}/secrets" "$body"
  elif [ "$code" != "200" ] && [ "$code" != "201" ]; then
    echo "ERROR: Secret apply HTTP ${code}" >&2
    exit 1
  fi
}

patch_deploy_env() {
  ns="$1" deploy="$2" container="$3" json_env="$4"
  patch="$(printf '{"spec":{"template":{"spec":{"containers":[{"name":"%s","env":%s}]}}}}' "$container" "$json_env")"
  api_json PATCH "/apis/apps/v1/namespaces/${ns}/deployments/${deploy}" "$patch" \
    "application/strategic-merge-patch+json" >/dev/null
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

REDIRECT_URI="https://${API_HOST}${CALLBACK_PATH}"
UI_ORIGIN="https://${UI_HOST}"
API_ORIGIN="https://${API_HOST}"
echo "OAuth redirect URI: ${REDIRECT_URI}"

KS="$(k8s_secret_val)"
OS="$(ocp_secret_val)"

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

ca_json="$(api_get "/api/v1/namespaces/openshift-config-managed/configmaps/oauth-serving-cert")"
ca_b64="$(printf '%s' "$ca_json" | sed -n 's/.*"ca-bundle.crt":"\([^"]*\)".*/\1/p' | head -1)"
if [ -n "$ca_b64" ]; then
  cm_body="$(printf '{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"spcg-oauth-serving-ca","namespace":"%s"},"data":{"ca-bundle.crt":"%s"}}' \
    "$CONTROL_NS" "$ca_b64")"
  code="$(http_code -X PUT -H "Content-Type: application/json" -d "$cm_body" \
    "${API_BASE}/api/v1/namespaces/${CONTROL_NS}/configmaps/spcg-oauth-serving-ca")"
  if [ "$code" = "404" ]; then
    api_json POST "/api/v1/namespaces/${CONTROL_NS}/configmaps" "$cm_body" >/dev/null || true
  fi
fi

OAUTH_HOST="$(json_field "$(api_get "/apis/route.openshift.io/v1/namespaces/openshift-authentication/routes/oauth-openshift")" host)"
if [ -n "$OAUTH_HOST" ]; then
  patch_deploy_env "$CONTROL_NS" "spcg-ui-portal" "ui-portal" \
    "$(printf '[{"name":"OAUTH_TOKEN_URL","value":"https://%s/oauth/token"},{"name":"NO_PROXY","value":".cluster.local,.svc,127.0.0.1,localhost,%s"}]' \
      "$OAUTH_HOST" "$OAUTH_HOST")"
fi

patch_deploy_env "$LANDING_NS" "spcg-frontend" "frontend" \
  "$(printf '[{"name":"SPCG_PUBLIC_API_BASE","value":"%s"},{"name":"SPCG_DISABLE_API_PROXY","value":"true"}]' "$API_ORIGIN")"
patch_deploy_env "$CONTROL_NS" "spcg-ui-portal" "ui-portal" \
  "$(printf '[{"name":"CORS_ORIGIN","value":"%s"}]' "$UI_ORIGIN")"

echo "OAuth bootstrap complete."
