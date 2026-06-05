# OpenShift security posture

Current SPCG layouts for production OpenShift. See [ARCHITECTURE.md](./ARCHITECTURE.md) for flows and [DEPLOYMENT.md](./DEPLOYMENT.md) for apply steps.

## Which overlay?

| Overlay | Namespaces | Use when |
|---------|------------|----------|
| **`openshift-small`** (default) | `pcap-frontend` + `pcap-capture` | Lab, migration, same-origin `/api` via Next.js middleware |
| **`openshift-secure`** | `spcg-landing` + `spcg-control` + `pcap-capture` | Production / hardened: landing has **no** API proxy; browser calls Route `spcg-api` |

Future gateway-only K8s API access is described in [SECURE-ARCHITECTURE-PLAN.md](./SECURE-ARCHITECTURE-PLAN.md).

---

## Monolithic layout (`openshift-small`)

### Namespace split (Pod Security)

| Namespace | PSS | Workloads | Why |
|-----------|-----|-----------|-----|
| `pcap-frontend` | **restricted** | `spcg-frontend`, `spcg-ui-portal`, `spcg-neo4j` | UI and API run non-root; no hostNetwork, no caps |
| `pcap-capture` | **privileged** | `spcg-backend-engine`, dynamic `spcg-sensor-*` | eBPF capture requires privileged SCC on `pcap-executor` SA only |

**Principle:** Keep the smallest privileged footprint in `pcap-capture`. Everything user-facing stays in `pcap-frontend` under restricted PSS.

### Authentication (monolithic)

- **OAuthClient** `spcg-ui` with redirect  
  `https://<route-spcg>/api/v1/auth/openshift/callback`
- Browser **always** calls same-origin `/api/*` on Route `spcg` (Next.js middleware → `spcg-ui-portal`)
- Route `spcg-api` is used only for **OAuth authorize redirect**, not XHR (avoids CORS + credentials issues)
- Do not set `__SPCG_API_BASE__` to `spcg-api` in the browser

### RBAC (monolithic)

| Component | SA | Notes |
|-----------|-----|--------|
| `spcg-ui-portal` | **`default`** (`pcap-frontend`) | Route read RBAC for `spcg`, `spcg-api`, `oauth-openshift` |
| `spcg-frontend` | `default` | No cluster API access |
| `spcg-backend-engine` | `pcap-executor` | Privileged SCC + capture RBAC in `pcap-capture` only |

### Network policies (monolithic)

**`pcap-frontend`:** default deny egress; portal → Neo4j, engine, API server :443/:6443; frontend → portal; router → frontend + portal.

**`pcap-capture`:** default deny ingress; portal → engine :8443; sensors → collector 19000–19999.

### Secrets (monolithic)

| Secret | Namespace |
|--------|-----------|
| `spcg-oauth-client` | `pcap-frontend` |
| `spcg-neo4j-auth` | `pcap-frontend` |
| `spcg-graph-master-key` | `pcap-frontend` |

---

## Secure layout (`openshift-secure`)

Greenfield apply: **`oc apply -k manifests/openshift-secure`**, then wait for Job **`spcg-oauth-bootstrap`** (see [manifests/openshift-secure/README.md](../manifests/openshift-secure/README.md)). No `pcap-frontend` namespace is created.

### Namespace split (Pod Security)

| Namespace | PSS | Label | Workloads | K8s API / secrets |
|-----------|-----|-------|-----------|-------------------|
| `spcg-landing` | **restricted** | `spcg.io/role: landing` | `spcg-frontend` | No secrets; no egress |
| `spcg-control` | **restricted** | `spcg.io/role: control` | `spcg-ui-portal`, `spcg-neo4j` | OAuth + graph secrets; portal SA |
| `pcap-capture` | **privileged** | `spcg.io/role: capture` | `spcg-backend-engine`, `spcg-sensor-*` | Executor SA only |

### Authentication (secure)

- **OAuthClient** redirect on **API Route** (not landing):  
  `https://<route-spcg-api>/api/v1/auth/openshift/callback`
- Browser calls **`https://<route-spcg-api>/api/v1/*`** (`SPCG_PUBLIC_API_BASE` on landing frontend; middleware proxy disabled)
- Portal **`CORS_ORIGIN`** = `https://<route-spcg>` (landing URL only)
- Session cookies: cross-origin with `credentials: include`; portal sets CORS to landing origin

### RBAC (secure)

| Component | SA | Namespace | Notes |
|-----------|-----|-----------|--------|
| `spcg-ui-portal` | `default` | `spcg-control` | Reads Route `spcg-api` (control), `spcg` (landing), `oauth-openshift` |
| `spcg-frontend` | `default` | `spcg-landing` | No Route RBAC; no cluster API |
| `spcg-backend-engine` | `pcap-executor` | `pcap-capture` | Unchanged privileged SCC |

### Network policies (secure)

**`spcg-landing`:** default deny ingress + egress; router → frontend :3000 only.

**`spcg-control`:** default deny; router → portal :8080; portal → Neo4j :7687, engine :8443, DNS, API/S3 :443/:6443; Neo4j ingress from portal only.

**`pcap-capture`:** portal from `spcg.io/role: control` → engine :8443; sensor collectors unchanged.

### Secrets (secure)

| Secret | Namespace |
|--------|-----------|
| `spcg-oauth-client` | **`spcg-control`** |
| `spcg-neo4j-auth` | `spcg-control` |
| `spcg-graph-master-key` | `spcg-control` |

---

## Shared hardening

### OpenShift OAuth

- Portal discovers Routes in-cluster; token POST uses `oauth-openshift` Route + `OAUTH_TLS_INSECURE_SKIP_VERIFY` for ingress CA
- User **OAuth access token** stored in portal memory only; never in etcd or browser localStorage
- API calls use the **user bearer token**, not the pod SA
- Login validated with **SelfSubjectReview**

### Kubeconfig (break-glass)

- Enabled via `SPCG_AUTH_METHODS=openshift,kubeconfig` (default)
- Disable for strict prod: `SPCG_AUTH_METHODS=openshift` only

### Supply chain

Images: **`quay.io/moby/spcg-*`** (public). Build **linux/amd64** for cluster nodes.

## Hardening checklist

- [ ] OAuthClient redirect URI matches overlay (monolithic: Route `spcg`; secure: Route `spcg-api`)
- [ ] OAuth secret in correct namespace (`pcap-frontend` vs `spcg-control`)
- [ ] Run `./scripts/openshift-secure-apply.sh` or `./scripts/openshift-force-auth-fix.sh` after upgrade (tier-dependent)
- [ ] Restrict `SPCG_AUTH_METHODS=openshift` in production if kubeconfig upload not required
- [ ] Review RoleBindings for users who can access SPCG

## Residual risk

| Risk | Mitigation |
|------|------------|
| Privileged capture DaemonSets | Scoped namespaces; admission limits; isolated `pcap-capture` |
| Cross-origin API (secure overlay) | Locked `CORS_ORIGIN`; HttpOnly session cookies |
| OAuth TLS skip verify (token POST) | In-cluster only; prefer `spcg-oauth-serving-ca` ConfigMap |
| Kubeconfig upload | Optional; disable in prod |
| Portal memory holds tokens | Session timeout on logout; pod restart clears sessions |
