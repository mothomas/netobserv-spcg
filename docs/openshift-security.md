# OpenShift security posture

Current SPCG layout for production OpenShift. See [ARCHITECTURE.md](./ARCHITECTURE.md) for flows and [DEPLOYMENT.md](./DEPLOYMENT.md) for apply steps.

## Namespace split (Pod Security)

| Namespace | PSS | Workloads | Why |
|-----------|-----|-----------|-----|
| `pcap-frontend` | **restricted** | `spcg-frontend`, `spcg-ui-portal`, `spcg-neo4j` | UI and API run non-root; no hostNetwork, no caps |
| `pcap-capture` | **privileged** | `spcg-backend-engine`, dynamic `spcg-sensor-*` | eBPF capture requires privileged SCC on `pcap-executor` SA only |

**Principle:** Keep the smallest privileged footprint in `pcap-capture`. Everything user-facing stays in `pcap-frontend` under restricted PSS.

## Authentication

### OpenShift OAuth (primary)

- **OAuthClient** `spcg-ui` (cluster-admin creates once) with redirect  
  `https://<route-spcg>/api/v1/auth/openshift/callback`
- Portal discovers Routes in-cluster; token POST uses `oauth-openshift` Route + `OAUTH_TLS_INSECURE_SKIP_VERIFY` for ingress CA
- User **OAuth access token** stored in portal memory only (`auth.Store`); never written to etcd or browser localStorage (session id only)
- API calls use the **user bearer token**, not the pod SA (`restConfigForUserBearerToken` clears `BearerTokenFile`)
- Login validated with **SelfSubjectReview** (no cluster-scoped namespace list required)

### Kubeconfig (break-glass)

- Enabled via `SPCG_AUTH_METHODS=openshift,kubeconfig` (default in `manifests/openshift/config-auth-openshift.yaml`)
- Upload in UI; kubeconfig parsed server-side, credentials in RAM until logout
- Disable for strict prod: set `SPCG_AUTH_METHODS=openshift` only (overlay `openshift-kubeconfig` is kubeconfig-only)

### Browser / API path

- Browser **always** calls same-origin `/api/*` on Route `spcg` (Next.js middleware → `spcg-ui-portal`)
- Route `spcg-api` is used only for **OAuth authorize redirect**, not XHR (avoids CORS + credentials issues)
- Do not set `__SPCG_API_BASE__` to `spcg-api` in the browser

## RBAC and service accounts

| Component | SA | Notes |
|-----------|-----|--------|
| `spcg-ui-portal` | **`default`** (namespace) | Gets restricted-v2 automatically; Route read RBAC bound to `default` |
| `spcg-frontend` | `default` | No cluster API access |
| `spcg-backend-engine` | `pcap-executor` | Privileged SCC + capture RBAC in `pcap-capture` only |

Portal SA can **get** Routes: `spcg`, `spcg-api`, `oauth-openshift` (for URL discovery only).

## Network policies (`pcap-frontend`)

- Default deny ingress/egress; explicit allows:
  - Ingress to frontend + portal from OpenShift router
  - Portal → Neo4j, portal → capture engine gRPC, portal → API server :443/:6443, DNS
- Capture namespace policies isolate sensors and engine

## Secrets and supply chain

| Secret | Purpose |
|--------|---------|
| `spcg-oauth-client` | OAuth client secret (matches OAuthClient) |
| `spcg-neo4j-auth` | Neo4j password |
| `spcg-graph-master-key` | Graph label encryption |

Images: **`quay.io/moby/spcg-*`** (public). Build **linux/amd64** for cluster nodes (`docker buildx --platform linux/amd64`).

## Hardening checklist

- [ ] OAuthClient redirect URI matches Route `spcg` exactly
- [ ] Rotate `spcg-oauth-client` if ever exposed
- [ ] Run `./scripts/openshift-force-auth-fix.sh` after upgrade (token URL, NO_PROXY, oauth CA ConfigMap)
- [ ] Restrict `SPCG_AUTH_METHODS=openshift` in production if kubeconfig upload not required
- [ ] Set portal `CORS_ORIGIN` to Route `spcg` URL if exposing `spcg-api` directly
- [ ] Review RoleBindings for users who can access SPCG (same as any cluster API consumer)

## Residual risk

| Risk | Mitigation |
|------|------------|
| Privileged capture DaemonSets | Scoped to user-selected namespaces; admission limits; isolated namespace |
| Neo4j in frontend namespace | emptyDir/small tier; no hostPath; restricted PSS |
| OAuth TLS skip verify (token POST) | In-cluster only to oauth Route; prefer `spcg-oauth-serving-ca` ConfigMap when synced |
| Kubeconfig upload | Optional; disable in prod overlay |
| Portal memory holds tokens | Session timeout on logout; pod restart clears sessions |

Future: three-namespace layout (landing / control / capture) in [SECURE-ARCHITECTURE-PLAN.md](./SECURE-ARCHITECTURE-PLAN.md).
