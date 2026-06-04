# OpenShift OAuth for SPCG UI

Create an OAuth client and secret before enabling `SPCG_AUTH_METHODS=openshift` on `spcg-ui-portal`.

## 1. OAuthClient (cluster admin)

Adjust `redirectURIs` to match your API Route host (`OAUTH_REDIRECT_URL` in `manifests/openshift/patches/ui-portal-auth-openshift.yaml`).

```yaml
apiVersion: oauth.openshift.io/v1
kind: OAuthClient
metadata:
  name: spcg-ui
grantMethod: auto
redirectURIs:
  - https://spcg-api.apps.example.com/api/v1/auth/openshift/callback
secret: <generate-random-string>
```

```bash
oc apply -f oauth-client.yaml
```

## 2. Portal secret

```bash
oc create secret generic spcg-oauth-client -n pcap-frontend \
  --from-literal=client-secret='<same-as-oauth-client-secret>'
```

## 3. Patch env URLs

Edit `manifests/openshift/patches/ui-portal-auth-openshift.yaml`:

- `SPCG_FRONTEND_URL` → UI Route URL (`spcg` Route)
- `OAUTH_*` → your cluster OAuth host
- `OAUTH_REDIRECT_URL` → API Route + `/api/v1/auth/openshift/callback`

## 4. UI flow

User clicks **Sign in with OpenShift** → cluster OAuth → callback creates session → browser lands on `/auth/callback` → app continues with `X-SPCG-Session`.

Bearer token paste is **not** shown when `SPCG_AUTH_METHODS=openshift`. Vanilla K8s overlays use `kubeconfig` only.
