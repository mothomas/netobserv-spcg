# OpenShift login (Argo CD–style)

SPCG mirrors [Argo CD OpenShift SSO](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/user-management/index.md) and the [Dex OpenShift connector](https://github.com/dexidp/dex/blob/master/Documentation/connectors/openshift.md), without running Dex:

| | Argo CD | SPCG |
|---|---------|------|
| External URL | `argocd-cm` → `url` | Route **`spcg`** |
| Callback | `/api/dex/callback` | `/api/v1/auth/openshift/callback` |
| Token URL | Internal OAuth service | `https://oauth.openshift.svc.cluster.local/oauth/token` |

See [ARGO-CD-PARITY.md](./ARGO-CD-PARITY.md) for a full mapping.

## Choose a mode

| Goal | Apply |
|------|--------|
| **OpenShift login** (Argo-style) | `manifests/overlays/openshift-small` + OAuthClient below |
| **Kubeconfig only** (how it worked before OCP auth) | `manifests/overlays/openshift-kubeconfig` |

## OpenShift login setup

### 1. Routes and images

```bash
oc get route spcg -n pcap-frontend
oc apply -k manifests/overlays/openshift-small
```

Requires **`spcg-ui-portal:small-20260624+`** and **`spcg-frontend:small-20260624+`**. If Docker Hub rate-limits pulls, see [openshift-dockerhub-pull-secret.md](../../openshift-dockerhub-pull-secret.md).

### 2. OAuthClient (cluster admin) — same idea as Argo CD

Redirect URI **must match exactly** (cf. [argo-cd#4221](https://github.com/argoproj/argo-cd/issues/4221)):

```bash
UI_HOST=$(oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}')
echo "${UI_HOST}/api/v1/auth/openshift/callback"
```

Apply [oauth-client.yaml](./oauth-client.yaml) after editing host and secret, then:

```bash
oc create secret generic spcg-oauth-client -n pcap-frontend \
  --from-literal=client-secret='<same-as-oauth-client-secret>' \
  --dry-run=client -o yaml | oc apply -f -
```

### 3. Portal RBAC + env (from kustomize)

- `manifests/openshift/rbac-portal-oauth.yaml` — read Routes `oauth-openshift` and `spcg`
- `SPCG_AUTH_METHODS=openshift`, `OAUTH_CLIENT_ID=spcg-ui`, secret `spcg-oauth-client`

```bash
oc apply -k manifests/overlays/openshift-small
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

Optional TLS (Argo `insecureCA: true`):

```bash
oc set env deployment/spcg-ui-portal -n pcap-frontend OAUTH_TLS_INSECURE_SKIP_VERIFY=true
```

### 4. Verify

```bash
./scripts/openshift-verify-auth.sh
```

Or:

```bash
oc exec -n pcap-frontend deployment/spcg-ui-portal -- \
  wget -qO- http://127.0.0.1:8080/api/v1/auth/config | jq .
```

Browser: **Log in via OpenShift** on the Route `spcg` host.

## Kubeconfig-only (revert)

```bash
oc apply -k manifests/overlays/openshift-kubeconfig
oc rollout restart deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

No OAuthClient required; UI shows file upload / paste kubeconfig again.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `404 page not found` on `/api/v1/auth/config` | Portal image too old — use `small-20260624+`, delete stale pods |
| `ImagePullBackOff` / `toomanyrequests` | Cluster still on **docker.io** — apply overlay with **quay.io/moby** images ([openshift-quay-images.md](../../openshift-quay-images.md)) |
| `No sign-in methods` | `SPCG_AUTH_METHODS=openshift` missing on **spcg-frontend** |
| OAuth redirect mismatch | Redirect URI in OAuthClient ≠ discovered callback URL |
| Login timeout to OAuth (Argo [#12599](https://github.com/argoproj/argo-cd/issues/12599)) | Egress/proxy to `oauth-openshift`; token uses in-cluster `oauth.openshift.svc` |

```bash
bash scripts/openshift-force-auth-fix.sh
```
