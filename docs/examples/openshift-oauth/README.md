# OpenShift OAuth for SPCG UI

## 1. Discover your Route hosts

```bash
UI_HOST=$(oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}')
API_HOST=$(oc get route spcg-api -n pcap-frontend -o jsonpath='https://{.spec.host}')
OAUTH_HOST=$(oc get route oauth-openshift -n openshift-authentication -o jsonpath='https://{.spec.host}' 2>/dev/null || echo "https://oauth-openshift.apps.$(oc cluster-info | sed -n 's/.*apps\.\([^ ]*\).*/\1/p')")

echo "UI_HOST=$UI_HOST"
echo "API_HOST=$API_HOST"
echo "OAUTH_HOST=$OAUTH_HOST"
```

Edit these files and replace `apps.example.com` placeholders:

- `manifests/openshift/patches/ui-portal-auth-openshift.yaml`
- `manifests/openshift/patches/frontend-public-api.yaml`

| Env | Value |
|-----|--------|
| `SPCG_FRONTEND_URL` | `$UI_HOST` |
| `SPCG_PUBLIC_API_BASE` | `$API_HOST` |
| `OAUTH_AUTHORIZE_URL` | `$OAUTH_HOST/oauth/authorize` |
| `OAUTH_TOKEN_URL` | `$OAUTH_HOST/oauth/token` |
| `OAUTH_REDIRECT_URL` | `$API_HOST/api/v1/auth/openshift/callback` |

## 2. OAuthClient (cluster admin)

```yaml
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: spcg-ui
grantMethod: auto
redirectURIs:
  - https://YOUR-SPCG-API-HOST/api/v1/auth/openshift/callback
secret: <random-32-chars>
```

```bash
oc apply -f oauth-client.yaml
oc create secret generic spcg-oauth-client -n pcap-frontend \
  --from-literal=client-secret='<same-secret>' \
  --dry-run=client -o yaml | oc apply -f -
```

## 3. Apply and restart

```bash
oc apply -k manifests/overlays/openshift-small
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

## 4. Verify

```bash
# Portal env
oc exec -n pcap-frontend deploy/spcg-ui-portal -- wget -qO- http://127.0.0.1:8080/api/v1/auth/config

# From laptop (API route)
curl -s "https://$(oc get route spcg-api -n pcap-frontend -o jsonpath='{.spec.host}')/api/v1/auth/config"
```

Expected:

```json
{"methods":["openshift"],"openshift":{"authorize_path":"...","authorize_url":"https://spcg-api.../api/v1/auth/openshift/authorize"}}
```

UI should show **Sign in with OpenShift** only (no bearer token, no kubeconfig when `SPCG_AUTH_METHODS=openshift`).
