# SPCG deployment architecture

This document describes **what gets deployed**, how **Kustomize layers** compose, and how to run on **vanilla Kubernetes** vs **OpenShift** (including SCC).

For the **future secure namespace layout and phased rollout**, see [SECURE-ARCHITECTURE-PLAN.md](./SECURE-ARCHITECTURE-PLAN.md).

---

## 1. Manifest repository layout

```text
manifests/
├── kustomization.yaml          # Default entry → kubernetes/
├── base/                       # Shared workloads (all platforms)
│   ├── namespace-capture.yaml  # pcap-capture, PSS privileged
│   ├── namespace-frontend.yaml # pcap-frontend, PSS restricted
│   ├── network-policies.yaml
│   ├── config-capture-admission.yaml
│   ├── deployment-capture.yaml # spcg-backend-engine
│   ├── deployment-frontend.yaml# spcg-ui-portal + spcg-frontend
│   └── deployment-neo4j.yaml
├── kubernetes/                 # Vanilla K8s: RBAC, NodePort, image tags
│   ├── rbac-capture-k8s.yaml
│   └── patches/
├── openshift/                  # Routes, SCC, Neo4j PVC, ClusterIP UI, image tags
│   ├── rbac-capture.yaml
│   ├── route-openshift.yaml
│   ├── neo4j-pvc.yaml
│   └── patches/
├── overlays/
│   ├── small/                  # Vanilla K8s small tier
│   ├── medium/
│   ├── peak/
│   ├── openshift-small/        # OpenShift small (Routes + Neo4j PVC)
│   ├── openshift-medium/
│   └── openshift-peak/
└── demo-traffic/               # Optional lab ping workload

deploy/
├── Dockerfile.engine
├── Dockerfile.ui
└── Dockerfile.frontend

charts/spcg/                    # Helm alternative (OpenShift-oriented)
```

**Apply commands:**

| Profile | Command |
|---------|---------|
| **Small** (vanilla K8s) | `kubectl apply -k manifests/overlays/small` |
| **Medium** | `kubectl apply -k manifests/overlays/medium` |
| **Peak** | `kubectl apply -k manifests/overlays/peak` |
| **OpenShift Small** | `oc apply -k manifests/overlays/openshift-small` |
| **OpenShift Medium** | `oc apply -k manifests/overlays/openshift-medium` |
| **OpenShift Peak** | `oc apply -k manifests/overlays/openshift-peak` |

---

## 2. Kustomize inheritance chain

```text
                    ┌─────────────┐
                    │    base     │
                    └──────┬──────┘
                           │
              ┌────────────┴────────────┐
              │                         │
     ┌────────▼────────┐      ┌────────▼────────┐
     │   kubernetes    │      │    openshift    │
     │ RBAC, NodePort  │      │ SCC, Routes     │
     └────────┬────────┘      └─────────────────┘
              │
     ┌────────▼────────┐
     │  overlays/small │  ← image tags (ui-portal, frontend)
     └────────┬────────┘
              │
     ┌────────▼────────┐
     │ overlays/medium │  ← admission, replicas=2, ui-portal CPU/RAM
     └────────┬────────┘
              │
     ┌────────▼────────┐
     │  overlays/peak  │  ← admission, replicas=3, engine=2, HPA
     └─────────────────┘
```

**Important:** Tier overlays (`small` → `medium` → `peak`) build on **`kubernetes/`**, not `openshift/`. For OpenShift **with** tier caps, compose overlays manually (see §6).

---

## 3. Runtime topology (all tiers)

### 3.1 Namespaces and workloads

| Namespace | Workload | Type | Notes |
|-----------|----------|------|-------|
| `pcap-capture` | `spcg-backend-engine` | Deployment | `privileged: true`, SA `pcap-executor` |
| `pcap-capture` | `spcg-sensor-{captureId}` | DaemonSet | Created dynamically per capture |
| `pcap-frontend` | `spcg-ui-portal` | Deployment | Admission ConfigMap via `envFrom` |
| `pcap-frontend` | `spcg-frontend` | Deployment | Next.js |
| `pcap-frontend` | `spcg-neo4j` | Deployment | Bolt 7687, HTTP 7474 |

### 3.2 Services and ingress

| Service | Namespace | Access (vanilla) | Access (OpenShift) |
|---------|-----------|------------------|---------------------|
| `spcg-frontend` | `pcap-frontend` | NodePort **30080** → 3000 | Route `spcg` (edge TLS) |
| `spcg-ui-portal` | `pcap-frontend` | ClusterIP :80 → 8080 | Route `spcg-api` path `/api` |
| `spcg-backend-engine` | `pcap-capture` | ClusterIP gRPC 8443 | Same (internal) |
| `spcg-neo4j` | `pcap-frontend` | ClusterIP 7687/7474 | Internal only |

Next.js proxies browser `/api/v1/*` to `spcg-ui-portal` (`frontend/app/api/v1/[...path]/route.ts`). On OpenShift, Route `spcg-api` can expose portal directly for `/api` while UI uses Route `spcg`.

### 3.3 RBAC (capture plane)

**ServiceAccount:** `pcap-executor` in `pcap-capture`

**ClusterRole `spcg-pcap-executor`** (`kubernetes/rbac-capture-k8s.yaml` and `openshift/rbac-capture.yaml`):

| API group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` | namespaces, pods, nodes, services, endpoints | get, list, watch | Sensor targeting |
| `""` | nodes/proxy, pods/exec | create | Diagnostics |
| `apps` | deployments, replicasets, statefulsets | get, list, watch | Owner resolution |
| `apps` | daemonsets | get, list, watch, **create**, **delete** | Per-session sensors |

**Vanilla only:** ClusterRoleBinding `spcg-pcap-executor` → SA `pcap-executor`.

**OpenShift additional binding:** `spcg-pcap-executor-privileged-scc` → ClusterRole `system:openshift:scc:privileged` (see §5).

User-facing Kubernetes operations use **credentials from the UI login**, not `pcap-executor`.

---

## 4. Tier overlays (Small / Medium / Peak)

### 4.1 What each overlay changes

| Artifact | Small | Medium | Peak |
|----------|-------|--------|------|
| **Base chain** | `kubernetes` + small image tags | inherits small + patches | inherits medium + HPA |
| `patch-admission.yaml` | (base ConfigMap only) | `MAX_CONCURRENT_SESSIONS=5`, pods 15, 30m, **S3=true** | sessions 8, pods 20, 60m, S3=true |
| `patch-replicas.yaml` | base replicas (1) | ui-portal **2**, frontend **2** | ui-portal **3**, frontend **3**, engine **2** |
| `patch-ui-portal-resources.yaml` | — | req 500m/512Mi, lim 2CPU/2Gi | — |
| `hpa-frontend.yaml` | — | — | frontend HPA min 2 max 6 @ 70% CPU |

### 4.2 Admission ConfigMap (`spcg-capture-admission`)

| Key | Small (base) | Medium | Peak |
|-----|--------------|--------|------|
| `MAX_CONCURRENT_SESSIONS` | 2 | 5 | 8 |
| `MAX_PODS_PER_SESSION` | 10 | 15 | 20 |
| `MAX_CAPTURE_DURATION` | 15m | 30m | 60m |
| `MAX_CAPTURE_BYTES` | 104857600 (100 MiB) | same | same |
| `S3_OFFLOAD_ENABLED` | **false** | **true** | **true** |

Enforced in Go: `internal/capture/admission/limits.go` at capture stream start.

### 4.3 Image tags (example from repo)

| Component | Small overlay | kubernetes layer (if no overlay) |
|-----------|---------------|----------------------------------|
| `spcg-ui-portal` | `small-20260614` | `latest` (rewrite) |
| `spcg-frontend` | `small-20260614` | `latest` |
| `spcg-backend-engine` | (from kubernetes) `stream-fix-20260601` / `small-20260614` on OpenShift | same |

**Release practice:** bump tags in `manifests/overlays/small/kustomization.yaml` (and rebuild/push images). Verify with:

```bash
kubectl kustomize manifests/overlays/small | grep 'image:'
```

### 4.4 Deployment architecture diagram by tier

**Small** — single control plane replica, RAM PCAP, minimal blast radius:

```text
[Browser]──:30080──►[frontend x1]──►[ui-portal x1]──gRPC──►[engine x1]
                              │                              │
                              └──►[neo4j x1]                 └──►[sensor DS x N]
```

**Medium** — HA UI, S3 required, no engine scale-out:

```text
[Browser]──►[frontend x2]──►[ui-portal x2]──►[engine x1]──►[sensor DS]
                    │              │
                    └── sessionAffinity (Service)
```

**Peak** — UI scale + engine shard + HPA:

```text
[Browser]──►[frontend 2-6 HPA]──►[ui-portal x3]──►[engine x2]──►[sensor DS]
```

---

## 5. OpenShift deployment

OpenShift uses the same **base workloads** as vanilla Kubernetes (including **Neo4j**, admission ConfigMap, and graph secrets) plus Routes, privileged SCC binding, ClusterIP frontend Service, and a **Neo4j PVC**.

### 5.1 Apply commands

| Tier | Command |
|------|---------|
| **Small** (default) | `oc apply -k manifests/overlays/openshift-small` |
| **Secure** (3-namespace split) | `oc apply -k manifests/overlays/openshift-secure` or `./scripts/openshift-secure-apply.sh` |
| **Medium** | `oc apply -k manifests/overlays/openshift-medium` |
| **Peak** | `oc apply -k manifests/overlays/openshift-peak` |
| Base only (same as small) | `oc apply -k manifests/openshift/` |

### 5.2 What the OpenShift layer adds

| File / patch | Purpose |
|--------------|---------|
| `openshift/config-auth-openshift.yaml` | `SPCG_AUTH_METHODS=openshift,kubeconfig` |
| `openshift/rbac-capture.yaml` | `pcap-executor` RBAC + **SCC `privileged`** |
| `openshift/rbac-portal-oauth.yaml` | Route reader RBAC for **`default` SA** |
| `openshift/route-openshift.yaml` | Routes `spcg` (UI) and `spcg-api` (OAuth authorize only) |
| `openshift/patches/ui-portal-auth-openshift.yaml` | OAuth env, `default` SA, optional oauth CA mount |
| `openshift/patches/frontend-auth-openshift.yaml` | Auth ConfigMap on frontend |
| `openshift/patches/neo4j-pod-security.yaml` | OpenShift restricted UID assignment |
| `openshift/patches/quay-explicit-images.yaml` | Pin **quay.io/moby** image tags |
| `openshift/kustomization.yaml` | Image rewrites (portal/frontend `small-20260616`, built from commit `70288bf`) |

**Security:** [openshift-security.md](./openshift-security.md)

### 5.3 Routes

| Route | Service | Use |
|-------|---------|-----|
| `spcg` | `spcg-frontend` | **All browser traffic** — UI + `/api/*` proxy to portal |
| `spcg-api` | `spcg-ui-portal` | **OAuth authorize redirect only** (not browser XHR) |

Browser JSON APIs must stay on Route `spcg` (same origin). Route `spcg-api` avoids CORS issues for OAuth start URL only.

### 5.4 Security Context Constraints (SCC)

SPCG does **not** ship a custom `SecurityContextConstraints` object. It uses the platform **`privileged`** SCC through RBAC:

```yaml
# manifests/openshift/rbac-capture.yaml (excerpt)
kind: ClusterRoleBinding
metadata:
  name: spcg-pcap-executor-privileged-scc
roleRef:
  name: system:openshift:scc:privileged
subjects:
  - kind: ServiceAccount
    name: pcap-executor
    namespace: pcap-capture
```

**Who needs privileged:**

| Workload | Reason |
|----------|--------|
| `spcg-backend-engine` | `securityContext.privileged: true` in `deployment-capture.yaml` (eBPF orchestration path) |
| `spcg-sensor-*` DaemonSet | `hostNetwork`, `NET_RAW`, often `privileged` in embedded sensor manifest |

**Frontend namespace (`pcap-frontend`):** PSS **restricted** — ui-portal and Next.js run non-root without caps.

**Verify SCC on OpenShift:**

```bash
oc describe sa pcap-executor -n pcap-capture
oc get clusterrolebinding spcg-pcap-executor-privileged-scc
oc adm policy who-can use scc privileged -n pcap-capture
```

**Hardening note (TODO):** netobserv supports **capabilities-only** profiles on some platforms (`CAP_BPF`, `NET_ADMIN`, `PERFMON`). Moving off `privileged` requires manifest + SCC changes and regression testing — do not drop privileged without validating sensor start.

### 5.5 DNS network policy (OpenShift)

`allow-dns-egress` includes namespace `openshift-dns` (UDP 5353) in addition to `kube-system` (UDP 53). Required for ui-portal to resolve `spcg-backend-engine.pcap-capture.svc` and external S3 endpoints.

### 5.6 Pod Security Admission labels

| Namespace | Label | Level |
|-----------|-------|-------|
| `pcap-capture` | `pod-security.kubernetes.io/enforce` | **privileged** |
| `pcap-frontend` | `pod-security.kubernetes.io/enforce` | **restricted** |

Defined in `namespace-capture.yaml` / `namespace-frontend.yaml`.

---

## 6. OpenShift tier overlays

Same admission/replica semantics as vanilla tiers; inherit from `manifests/openshift/` instead of `manifests/kubernetes/`.

| Overlay path | Equivalent to |
|--------------|---------------|
| `overlays/openshift-small` | OpenShift base + small images |
| `overlays/openshift-medium` | + HA UI, S3 required, ui-portal resources |
| `overlays/openshift-peak` | + engine×2, frontend HPA, **Neo4j heap 2G / pagecache 1G** |

Peak Neo4j patch: `overlays/openshift-peak/patch-neo4j-resources.yaml`.

### 6.1 OpenShift secure overlay (`openshift-secure`)

Production-hardened **three-namespace** layout from [SECURE-ARCHITECTURE-PLAN.md](./SECURE-ARCHITECTURE-PLAN.md). Use when landing pods must have **no API proxy egress** and the browser calls the control-plane Route directly.

| Namespace | PSS | Workloads |
|-----------|-----|-----------|
| `spcg-landing` | restricted | `spcg-frontend` only |
| `spcg-control` | restricted | `spcg-ui-portal`, `spcg-neo4j`, OAuth/graph secrets |
| `pcap-capture` | privileged | `spcg-backend-engine`, ephemeral `spcg-sensor-*` |

**Apply:**

```bash
./scripts/openshift-secure-apply.sh
# or: oc apply -k manifests/overlays/openshift-secure
```

**After apply (cluster admin):**

1. Move or create OAuth secret in **`spcg-control`**:  
   `oc create secret generic spcg-oauth-client -n spcg-control --from-literal=client-secret=<same-as-OAuthClient>`
2. Register OAuthClient redirect URI:  
   `https://$(oc get route spcg-api -n spcg-control -o jsonpath='{.spec.host}')/api/v1/auth/openshift/callback`
3. Run the apply script (or manually set Route-derived env):  
   `SPCG_PUBLIC_API_BASE` on `spcg-frontend` (landing), `CORS_ORIGIN` on `spcg-ui-portal` (control, landing Route URL).

**Routes:**

| Route | Namespace | Service | Browser use |
|-------|-----------|---------|-------------|
| `spcg` | `spcg-landing` | `spcg-frontend` | UI `/` only |
| `spcg-api` | `spcg-control` | `spcg-ui-portal` | All `/api/v1/*`, OAuth callback |

**When to use:** `openshift-secure` for production / pen-test profile. Keep `openshift-small` for lab and migration soak (monolithic `pcap-frontend`).

---

## 7. Combining OpenShift with tier overlays (legacy note)

Tier overlays under `manifests/overlays/openshift-*` **replace** the manual merge workflow documented earlier. Use those paths directly.

---

## 8. Network policy map

### 8.1 `pcap-capture` (ingress-focused)

| Policy | Effect |
|--------|--------|
| `default-deny-ingress` | Block all ingress by default |
| `allow-frontend-to-backend-engine` | ui-portal → engine **TCP 8443** |
| `allow-sensor-to-backend-collector` | **TCP 19000–19999** (hostNetwork + sensor pods) |

### 8.2 `pcap-frontend` (egress-focused)

| Policy | Effect |
|--------|--------|
| `default-deny-egress` | Block all egress by default |
| `allow-dns-egress` | DNS to kube-system / openshift-dns |
| `allow-ui-portal-to-capture-engine` | portal → engine gRPC |
| `allow-frontend-to-ui-portal` | Next.js → portal HTTP |
| `allow-ui-portal-to-neo4j` | portal → Neo4j **7687** |
| `allow-ui-portal-k8s-api-egress` | **TCP 443, 6443** (user API + S3 HTTPS) |
| `allow-ingress-to-frontend-services` | Ingress to UI pods **3000, 8080** |

**Operational implication:** S3 endpoints must be reachable on 443 from ui-portal pods (world-open today — see README TODO for CIDR-scoped policy).

---

## 9. Secrets and configuration

| Secret | Namespace | Keys | Required |
|--------|-----------|------|----------|
| `spcg-neo4j-auth` | `pcap-frontend` | `password` | Yes (Neo4j boot) |
| `spcg-graph-master-key` | `pcap-frontend` | `GRAPH_MASTER_KEY` | Yes for encrypted graph labels |
| `spcg-engine-mtls` | `pcap-capture` | TLS cert/key | **Optional** — without it, gRPC uses insecure credentials |

**Replace placeholders before production** (see README Security TODO).

| ConfigMap | Purpose |
|-----------|---------|
| `spcg-capture-admission` | Tier limits + S3 policy |

---

## 10. Dynamic resources (not in base kustomize)

Created at runtime by `spcg-backend-engine` / sensor manager:

- DaemonSet `spcg-sensor-{captureSessionId}` in `pcap-capture`
- Labels: `app: spcg-sensor`, session metadata

Cleaned up when capture ends (delete DS).

---

## 11. Build and publish images

See [ci-cd.md](./ci-cd.md). Typical local build:

```bash
docker build --platform linux/amd64 -f deploy/Dockerfile.ui -t docker.io/<org>/spcg-ui-portal:<tag> .
docker build --platform linux/amd64 -f deploy/Dockerfile.frontend -t docker.io/<org>/spcg-frontend:<tag> .
docker build --platform linux/amd64 -f deploy/Dockerfile.engine -t docker.io/<org>/spcg-backend-engine:<tag> .
```

Override sensor image at engine runtime: `NETOBSERV_AGENT_IMAGE` env (if wired in deployment).

---

## 12. Post-deploy verification

```bash
kubectl get pods -n pcap-capture
kubectl get pods -n pcap-frontend
kubectl get netpol -n pcap-capture
kubectl get netpol -n pcap-frontend
curl -s http://<node-ip>:30080/   # vanilla
oc get route -n pcap-frontend      # OpenShift
```

Login → start short capture → confirm sensor DS → observability API returns topology.

---

## 13. Related documents

- [ARCHITECTURE.md](./ARCHITECTURE.md) — data flows and design rationale
- [architecture-tiers.md](./architecture-tiers.md) — sizing tables and roadmap
- [kubernetes-vs-openshift.md](./kubernetes-vs-openshift.md) — short platform comparison
