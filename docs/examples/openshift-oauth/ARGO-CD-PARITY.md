# Argo CD ↔ SPCG OpenShift OAuth mapping

Argo CD on OpenShift uses **Dex** with an **openshift** connector ([Dex docs](https://github.com/dexidp/dex/blob/master/Documentation/connectors/openshift.md), [Argo CD operator](https://github.com/argoproj/argo-cd/blob/master/controllers/argocd/sso.go)). SPCG uses the **same OAuth server and redirect rules**, but handles the flow in **spcg-ui-portal** (no Dex).

| Argo CD | SPCG |
|---------|------|
| `argocd-cm` → `url: https://<route-host>` | Route **`spcg`** → `https://<host>` (auto-discovered) |
| Dex callback `/api/dex/callback` | Portal callback `/api/v1/auth/openshift/callback` (proxied by Next.js `/api/*`) |
| `OAuthClient` / SA redirect URI **must match exactly** | Same — register UI host + callback path |
| Authorize: `oauth-openshift` Route `/oauth/authorize` | Same (discovered in-cluster) |
| Token: internal `oauth.openshift.svc` (Dex) | Same: `https://oauth.openshift.svc.cluster.local/oauth/token` |
| `clientID` + `clientSecret` in `argocd-secret` | `OAUTH_CLIENT_ID` + secret **`spcg-oauth-client`** |
| `grantMethod: auto` on OAuthClient | Recommended for UI login |
| Optional `insecureCA: true` (Dex) | `OAUTH_TLS_INSECURE_SKIP_VERIFY=true` on portal if needed |
| Groups claim / RBAC in Argo CD | Kubernetes **RoleBindings** for the user token (unchanged) |

## Redirect URI (most common failure)

Argo CD ([issue #4221](https://github.com/argoproj/argo-cd/issues/4221)):

```text
https://<argocd-route>/api/dex/callback
```

SPCG:

```text
https://<spcg-route>/api/v1/auth/openshift/callback
```

Print yours:

```bash
UI_HOST=$(oc get route spcg -n pcap-frontend -o jsonpath='https://{.spec.host}')
echo "${UI_HOST}/api/v1/auth/openshift/callback"
```

That string must appear **verbatim** in `OAuthClient.spec.redirectURIs` and match what the portal discovers.

## Revert to kubeconfig-only (pre–OpenShift auth)

If cluster auth integration blocks rollout (image pull, old portal tag), use the kubeconfig overlay:

```bash
oc apply -k manifests/overlays/openshift-kubeconfig
```

See [README.md](./README.md).
