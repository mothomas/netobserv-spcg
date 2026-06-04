# OpenShift login (Argo CD–style)

## One Route is enough

SPCG needs Route **`spcg`** (UI). Route **`spcg-api`** is optional (direct API/SSE). OAuth callback URL uses the **UI** host:

`https://<spcg-route-host>/api/v1/auth/openshift/callback`

(Next.js proxies `/api/*` to `spcg-ui-portal`.)

## 1. Check Routes and pods

```bash
oc get route spcg spcg-api -n pcap-frontend
oc get pods -n pcap-frontend -l 'app in (spcg-frontend,spcg-ui-portal)'
oc get pods -n pcap-capture -l app=spcg-backend-engine
```

If **`spcg` Route is missing**:

```bash
oc apply -k manifests/overlays/openshift-small
```

Open UI: `oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}{"\n"}'`

## 2. OAuthClient (cluster admin)

```bash
UI_HOST=$(oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}')
echo "Register redirect: ${UI_HOST}/api/v1/auth/openshift/callback"
```

```yaml
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: spcg-ui
grantMethod: auto
redirectURIs:
  - https://YOUR-SPCG-ROUTE-HOST/api/v1/auth/openshift/callback
secret: <random-32-chars>
```

```bash
oc create secret generic spcg-oauth-client -n pcap-frontend \
  --from-literal=client-secret='<same-secret>' \
  --dry-run=client -o yaml | oc apply -f -
```

## 3. Portal RBAC + secret

- `manifests/openshift/rbac-portal-oauth.yaml` — SA `spcg-ui-portal` reads Routes
- Deployment env: `SPCG_AUTH_METHODS=openshift`, `OAUTH_CLIENT_ID=spcg-ui`, secret above

```bash
oc apply -k manifests/overlays/openshift-small
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

## 4. Verify

`pcap-frontend` uses **restricted** PodSecurity — plain `oc run curl` is blocked. Use one of:

```bash
# In-cluster (no extra pod)
oc exec -n pcap-frontend deployment/spcg-ui-portal -- \
  wget -qO- http://127.0.0.1:8080/api/v1/auth/config | jq .

# From laptop
oc port-forward -n pcap-frontend deployment/spcg-ui-portal 18080:8080
curl -s http://127.0.0.1:18080/api/v1/auth/config | jq .

# Or run the helper script
./scripts/openshift-verify-auth.sh
```

Browser UI (`oc get route spcg …`) should show **Log in via OpenShift**, not an empty card.

If `/api/v1/auth/config` returns **404 page not found**, the **portal image is too old** — set
`spcg-ui-portal` to **small-20260622+** and restart.

If the UI says **No sign-in methods configured**, `SPCG_AUTH_METHODS=openshift` is missing on
**spcg-frontend** (re-apply overlay or `oc set env` below).

## 5. Quick fix (when UI shows 404 + no methods)

```bash
oc set env deployment/spcg-frontend -n pcap-frontend SPCG_AUTH_METHODS=openshift
oc set env deployment/spcg-ui-portal -n pcap-frontend \
  SPCG_AUTH_METHODS=openshift OAUTH_CLIENT_ID=spcg-ui
oc set image deployment/spcg-ui-portal -n pcap-frontend \
  ui-portal=docker.io/mothomas/spcg-ui-portal:small-20260622
oc set image deployment/spcg-frontend -n pcap-frontend \
  frontend=docker.io/mothomas/spcg-frontend:small-20260623
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
oc rollout status deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

Also ensure `spcg-oauth-client` secret exists (see §2) and run `oc apply -k manifests/overlays/openshift-small`.

## 6. Images

Use tags **small-20260621** or later for `spcg-ui-portal` and **small-20260623** for `spcg-frontend`.
