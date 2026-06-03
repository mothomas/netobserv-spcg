# SPCG deployment architecture

This document describes **what gets deployed**, how **Kustomize layers** compose, and how to run on **vanilla Kubernetes** vs **OpenShift** (including SCC).

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
├── openshift/                  # Routes + privileged SCC binding (no tiers)
│   ├── rbac-capture.yaml
│   └── route-openshift.yaml
├── overlays/
│   ├── small/                  # Default tier image tags
│   ├── medium/                 # HA + S3 required + resources
│   └── peak/                   # More replicas, engine=2, HPA
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
| Vanilla K8s (default kubernetes layer) | `kubectl apply -k manifests/` |
| **Small** tier | `kubectl apply -k manifests/overlays/small` |
| **Medium** tier | `kubectl apply -k manifests/overlays/medium` |
| **Peak** tier | `kubectl apply -k manifests/overlays/peak` |
| **OpenShift** (base + Route + SCC) | `kubectl apply -k manifests/openshift/` |

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
| `spcg-ui-portal` | `small-20260611` | `latest` (rewrite) |
| `spcg-frontend` | `small-20260613` | `latest` |
| `spcg-backend-engine` | (from kubernetes) `stream-fix-20260601` | same |

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

### 5.1 Apply base OpenShift layer

```bash
oc apply -k manifests/openshift/
```

This deploys **base workloads** plus:

1. **Routes** (`manifests/openshift/route-openshift.yaml`)
2. **SCC access** via ClusterRoleBinding (not a custom SCC CR)

### 5.2 Routes

| Route | Service | Path | TLS |
|-------|---------|------|-----|
| `spcg` | `spcg-frontend` | `/` | edge, redirect HTTP |
| `spcg-api` | `spcg-ui-portal` | `/api` | edge |

**Why two routes:** Next.js can serve UI on `/` while API can be pinned to Go portal for large SSE payloads; also supports splitting TLS policies later.

### 5.3 Security Context Constraints (SCC)

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

### 5.4 DNS network policy (OpenShift)

`allow-dns-egress` includes namespace `openshift-dns` (UDP 5353) in addition to `kube-system` (UDP 53). Required for ui-portal to resolve `spcg-backend-engine.pcap-capture.svc` and external S3 endpoints.

### 5.5 Pod Security Admission labels

| Namespace | Label | Level |
|-----------|-------|-------|
| `pcap-capture` | `pod-security.kubernetes.io/enforce` | **privileged** |
| `pcap-frontend` | `pod-security.kubernetes.io/enforce` | **restricted** |

Defined in `namespace-capture.yaml` / `namespace-frontend.yaml`.

---

## 6. Combining OpenShift with tier overlays

Tier overlays today assume the **`kubernetes/`** layer (NodePort patch, vanilla RBAC). OpenShift uses **`base` + openshift RBAC** only.

**Recommended patterns:**

1. **OpenShift + Small admission (manual)**  
   - `oc apply -k manifests/openshift/`  
   - Patch ConfigMap: `oc patch cm spcg-capture-admission -n pcap-frontend --type merge -f manifests/overlays/small/...`  
   - Set images via `oc set image deployment/...`

2. **Custom kustomization (GitOps)** — create e.g. `manifests/openshift-overlays/production/kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../openshift
  - ../../overlays/medium   # only if you add openshift-compatible medium root
patches:
  - path: patch-route-host.yaml
```

3. **Helm** — `charts/spcg/` for teams standardizing on Helm + OpenShift values.

**Do not** blindly `kubectl apply -k overlays/peak` on OpenShift without checking NodePort patches and SCC bindings.

---

## 7. Network policy map

### 7.1 `pcap-capture` (ingress-focused)

| Policy | Effect |
|--------|--------|
| `default-deny-ingress` | Block all ingress by default |
| `allow-frontend-to-backend-engine` | ui-portal → engine **TCP 8443** |
| `allow-sensor-to-backend-collector` | **TCP 19000–19999** (hostNetwork + sensor pods) |

### 7.2 `pcap-frontend` (egress-focused)

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

## 8. Secrets and configuration

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

## 9. Dynamic resources (not in base kustomize)

Created at runtime by `spcg-backend-engine` / sensor manager:

- DaemonSet `spcg-sensor-{captureSessionId}` in `pcap-capture`
- Labels: `app: spcg-sensor`, session metadata

Cleaned up when capture ends (delete DS).

---

## 10. Build and publish images

See [ci-cd.md](./ci-cd.md). Typical local build:

```bash
docker build --platform linux/amd64 -f deploy/Dockerfile.ui -t docker.io/<org>/spcg-ui-portal:<tag> .
docker build --platform linux/amd64 -f deploy/Dockerfile.frontend -t docker.io/<org>/spcg-frontend:<tag> .
docker build --platform linux/amd64 -f deploy/Dockerfile.engine -t docker.io/<org>/spcg-backend-engine:<tag> .
```

Override sensor image at engine runtime: `NETOBSERV_AGENT_IMAGE` env (if wired in deployment).

---

## 11. Post-deploy verification

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

## 12. Related documents

- [ARCHITECTURE.md](./ARCHITECTURE.md) — data flows and design rationale
- [architecture-tiers.md](./architecture-tiers.md) — sizing tables and roadmap
- [kubernetes-vs-openshift.md](./kubernetes-vs-openshift.md) — short platform comparison
