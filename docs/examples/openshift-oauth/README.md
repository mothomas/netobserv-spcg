# OpenShift login (Argo CD–style)

SPCG discovers OAuth and Route hosts from the cluster. **No domain URLs in env** — only an OAuth client + secret (one-time cluster admin setup).

## 1. OAuthClient

Redirect URI is discovered from the `spcg-api` Route:

`https://<spcg-api-route-host>/api/v1/auth/openshift/callback`

```bash
API_HOST=$(oc get route spcg-api -n pcap-frontend -o jsonpath='https://{.spec.host}')
```

```yaml
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: spcg-ui
grantMethod: auto
redirectURIs:
  - REPLACE_WITH_API_HOST/api/v1/auth/openshift/callback
secret: <random-32-chars>
```

```bash
oc apply -f oauth-client.yaml
oc create secret generic spcg-oauth-client -n pcap-frontend \
  --from-literal=client-secret='<same-secret>' \
  --dry-run=client -o yaml | oc apply -f -
```

## 2. Deploy

```bash
oc apply -k manifests/overlays/openshift-small
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

Portal ServiceAccount `spcg-ui-portal` must read Routes (`manifests/openshift/rbac-portal-oauth.yaml`).

## 3. UI

Single **Log in via OpenShift** button → cluster login page (username/password) → return to SPCG.

Vanilla Kubernetes overlays use **kubeconfig file** only (`SPCG_AUTH_METHODS=kubeconfig`).

## 4. Verify

```bash
curl -s "https://$(oc get route spcg-api -n pcap-frontend -o jsonpath='{.spec.host}')/api/v1/auth/config" | jq .
```

Expect `public_api_base`, `openshift.authorize_url`, and `methods: ["openshift"]`.
