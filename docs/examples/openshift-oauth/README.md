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

```bash
UI=$(oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}')
curl -s "${UI}/api/v1/auth/config" | jq .
```

From laptop browser: open **`$UI`** — you should see **Log in via OpenShift** (not kubeconfig).

If `/api/v1/auth/config` fails, check frontend → portal NetworkPolicy and `SPCG_API_URL` on frontend deployment (in-cluster `http://spcg-ui-portal.pcap-frontend.svc.cluster.local:80`).

## 5. Images

Use tags **small-20260618** or later for `spcg-ui-portal` and `spcg-frontend`.
