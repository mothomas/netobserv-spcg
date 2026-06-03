# SPCG architecture tiers (Small / Medium / Peak)

Three deployment profiles for **Secure Packet Capture Gateway (SPCG)**.  
**Default install = Small.** Scale to Medium or Peak using **Kubernetes-native controls only** (replicas, resources, HPA, node pools, kustomize/Helm values) — no application code changes required to move between tiers.

**See also:** [DEPLOYMENT.md](./DEPLOYMENT.md) (manifest paths, OpenShift SCC), [ARCHITECTURE.md](./ARCHITECTURE.md) (data flows, decisions).

---

## 1. Design principles

| Principle | Meaning |
|-----------|---------|
| **Small by default** | Minimal replicas, minimal CPU/RAM, single engine shard |
| **Scale via K8s** | Replicas, `resources`, HPA/VPA, PDB, node labels/taints, overlay patches |
| **Sensor dumps up** | eBPF DaemonSet exports raw/filtered flows; enrichment and tenant fanout happen in engine/portal |
| **Thin frontend** | Browser renders stats/graph/PCAP links; no heavy state in Next.js |
| **Tiered retention** | ≤15 min & ≤100 MB → portal RAM download; larger/longer → **S3 streaming** (tenant creds at capture start) |

---

## 2. Logical architecture (all tiers)

```text
                         ┌─────────────────────────────────────┐
                         │  Ingress / NodePort / Route         │
                         └─────────────────┬───────────────────┘
                                           │ HTTPS (SSE + REST)
┌────────────────────────── pcap-frontend ─┴──────────────────────────┐
│  spcg-frontend (N)  ──►  spcg-ui-portal (N, sticky)                 │
│       │                        │                                     │
│       │                        ├── in-memory **flow metadata** (stats/topology) │
│       │                        ├── optional RAM PCAP (Small tier, short captures)│
│       │                        ├── stats / topology / graph API (thin browser)     │
│       │                        └── PCAP export: RAM download **or** S3 stream    │
│       │                        │                                     │
│       │                        ├──► spcg-neo4j (graph, optional)     │
│       │                        └──► S3 endpoint (large PCAP)         │
└───────┼──────────────────────────────────────────────────────────────┘
        │ gRPC :8443
┌───────┴────────────────── pcap-capture ──────────────────────────────┐
│  spcg-backend-engine (1..N shards)                                    │
│    ├── CaptureService gRPC (portal streams)                           │
│    ├── Per-session collectors :19000–19999 (today)                    │
│    └── Future: shared collector + tenant fanout (max 5 sessions/shard)│
│                                                                       │
│  spcg-sensor-* DaemonSet (per session today → shared per node later)  │
│    hostNetwork eBPF agents on all capture nodes                       │
└───────────────────────────────────────────────────────────────────────┘
        │
        ▼
  User Kubernetes API (6443/443) — workloads, sensor deploy RBAC
```

**Per-tenant during capture:** stats, burst plot, Sigma graph, edge drill-down — all from **aggregated APIs** (Neo4j + portal), not raw packets in the browser.

**Isolation model:** one shared capture plane (engine + portal); tenants separated by **auth session id**, **capture session id**, Neo4j subgraph keys, and per-tenant AES-GCM labels (`GRAPH_MASTER_KEY` + auth session).

---

## 2.1 Data plane (as implemented)

| Layer | Responsibility | Storage |
|-------|----------------|---------|
| **Sensor DaemonSet** | eBPF capture on selected pods/nodes | None (streams to engine) |
| **Backend engine** | gRPC ingest, per-session collectors `:19000–19999` | None (forwards to portal stream) |
| **UI portal** | SSE to browser, admission caps, PCAP path choice | Flow **metadata** in RAM; PCAP bytes in RAM **or** S3 multipart |
| **Neo4j** | Per-capture topology subgraph for Sigma.js | In-cluster graph (optional PVC at Peak) |
| **Frontend (Next.js)** | Thin client: SSE metrics, poll graph/context, export links | No PCAP/state |

**S3 streaming mode (GUI):** user supplies endpoint, bucket, prefix, and credentials; **Test S3** before capture; frames never land in pod buffers — only aggregated events for stats/graph. Object key: `{prefix}/{capture-id}/merged.pcapng`.

**RAM mode (Small default):** PCAP-NG buffered in portal memory (trimmed per pod) until download or teardown; capped by `MAX_CAPTURE_BYTES` and `MAX_CAPTURE_DURATION`.

**Medium/Peak:** `S3_OFFLOAD_ENABLED=true` requires S3 streaming at capture start (tenant-owned bucket); portal still holds metadata only.

## 3. Three tiers at a glance

| Dimension | **Small** | **Medium** | **Peak** |
|-----------|-----------|------------|----------|
| **Target** | Dev, PoC, 1–2 concurrent captures | Prod team, 3–5 captures | Incident / multi-tenant burst |
| **Concurrent captures** | 1–2 | 3–5 | 5–8 (with caps) |
| **Pods per session (cap)** | 10 | 10–20 | 20 |
| **Nodes in scope** | ≤5 | ≤20 | ≤20+ |
| **PCAP hot path** | Portal RAM (≤ caps) | S3 streaming required | S3 streaming required |
| **Neo4j** | Optional / off | 1 replica | 1–2 replicas or external |
| **Engine shards** | 1 | 1 | 2+ (HPA or fixed replicas) |

---

## 4. Component sizing matrix

### 4.1 Sensor DaemonSet (per node, `pcap-capture`)

| | Small | Medium | Peak |
|---|------|--------|------|
| **CPU request / limit** | 250m / 1 | 500m / 2 | 1 / 4 |
| **Memory request / limit** | 256Mi / 1Gi | 512Mi / 2Gi | 1Gi / 4Gi |
| **DS model** | Per-session DS (current) | Per-session DS | Shared DS per node (future) |
| **K8s scale lever** | Fewer concurrent sessions | Node pool size | Dedicated capture node pool + taints |

### 4.2 Backend engine + collector (`spcg-backend-engine`)

Collector runs **in-process** on the engine pod today (ports 19000–19999).

| | Small | Medium | Peak |
|---|------|--------|------|
| **Replicas** | 1 | 1 | 2–4 (HPA on CPU + custom metric later) |
| **CPU request / limit** | 500m / 2 | 1 / 4 | 2 / 8 |
| **Memory request / limit** | 1Gi / 4Gi | 2Gi / 8Gi | 4Gi / 16Gi |
| **K8s scale lever** | `replicas: 1` | Patch resources | HPA + second Deployment shard by node pool |

### 4.3 UI portal (`spcg-ui-portal`)

| | Small | Medium | Peak |
|---|------|--------|------|
| **Replicas** | 1 | 2 | 3–5 |
| **CPU request / limit** | 200m / 1 | 500m / 2 | 1 / 4 |
| **Memory request / limit** | 256Mi / 1Gi | 512Mi / 2Gi | 1Gi / 4Gi |
| **Session affinity** | ClientIP (optional) | **Required** | Required + external session store (future) |
| **K8s scale lever** | `replicas: 1` | `replicas: 2` + affinity | HPA on CPU/memory |

### 4.4 Frontend (`spcg-frontend`)

| | Small | Medium | Peak |
|---|------|--------|------|
| **Replicas** | 1 | 2 | 3+ |
| **CPU request / limit** | 100m / 500m | 200m / 1 | 500m / 2 |
| **Memory request / limit** | 128Mi / 512Mi | 256Mi / 1Gi | 512Mi / 2Gi |
| **K8s scale lever** | `replicas: 1` | HPA | HPA + Ingress |

### 4.5 Neo4j graph store (`spcg-neo4j`, optional)

| | Small | Medium | Peak |
|---|------|--------|------|
| **Deploy?** | Off or 1 | 1 | 1 (+ PVC) |
| **CPU request / limit** | — / 1 | 500m / 2 | 1 / 4 |
| **Memory request / limit** | — / 2Gi | 1Gi / 4Gi | 2Gi / 8Gi |
| **Storage** | emptyDir | emptyDir or 10Gi PVC | 50Gi+ PVC |
| **K8s scale lever** | Omit from kustomization | Resource patch | PVC + node affinity |

---

## 5. Where data lives during a session

| Data | RAM capture (Small) | S3 streaming capture (any tier) |
|------|---------------------|----------------------------------|
| **Packet frames / PCAP** | Portal pod **RAM** (`pcap.Session`, trimmed) | **Tenant S3** multipart upload; **zero** frame bytes in portal RAM |
| **Flow metadata** | Portal RAM events (for stats, JSONL, topology build) | Same — metadata only, no `Frame` payload |
| **Stats / burst plot** | Computed from events; API poll | Same |
| **Graph (Sigma)** | Neo4j subgraph via `POST /api/v1/graph/topology` | Same |
| **Browser** | SSE chunk metrics + RAM PCAP download | SSE metrics + presigned S3 URL (`Open in S3`) |
| **Teardown** | Purge RAM + Neo4j subgraph | Finalize S3 multipart + purge metadata + Neo4j |

**Admission policy** (ConfigMap `spcg-capture-admission` → ui-portal env; enforced in portal):

```yaml
MAX_CAPTURE_DURATION: "15m"        # hard stop (all modes)
MAX_CAPTURE_BYTES: "104857600"     # RAM mode only; S3 mode exempt
MAX_PODS_PER_SESSION: "10"
MAX_CONCURRENT_SESSIONS: "2"       # Small; Medium=5; Peak=8
S3_OFFLOAD_ENABLED: "false"        # true → capture start requires S3 GUI + test
```

API: `GET /api/v1/capture/limits` exposes active policy to the UI.

---

## 6. Kubernetes-native scaling path

Scale **Small → Medium → Peak** by overlay or Helm values only.

### 6.1 Recommended layout

```text
manifests/
  base/                    # Small defaults baked in
  overlays/
    small/                 # explicit Small (default)
    medium/                # resource + replica patches
    peak/                  # HPA, PDB, neo4j PVC, engine replicas
```

### 6.2 Small → Medium (no code change)

| Action | K8s mechanism |
|--------|----------------|
| More UI capacity | `kubectl scale deploy/spcg-frontend --replicas=2` |
| Portal HA | `spcg-ui-portal` replicas=2, keep `sessionAffinity: ClientIP` |
| Engine headroom | Strategic merge patch: raise `resources.limits` |
| Neo4j on | Add `deployment-neo4j.yaml` to overlay resources |
| S3 offload | ConfigMap env on ui-portal: `S3_OFFLOAD_ENABLED=true` (requires S3 in capture GUI) |

### 6.3 Medium → Peak

| Action | K8s mechanism |
|--------|----------------|
| Engine horizontal | `replicas: 2` on engine **or** HPA `minReplicas: 2 maxReplicas: 4` |
| Frontend HPA | CPU 70%, min 2 max 6 |
| Capture node pool | `nodeSelector: spcg.io/capture=true` on engine + sensors |
| Neo4j persistence | PVC `ReadWriteOnce`, increase heap via env |
| PodDisruptionBudget | `minAvailable: 1` on portal + frontend |

### 6.4 What stays outside K8s scaling

These require **future code** (not tier toggles):

- Shared sensor DaemonSet (5 sessions per node)
- Engine tenant fanout matcher
- Redis/external session store for stateless portal
- Custom HPA metric (ingest lag / events/sec)

Until then, **limit concurrent captures** via ConfigMap admission caps.

---

## 7. Network & firewall by tier

Same paths all tiers; Peak adds egress to S3.

| Flow | Port | Namespace |
|------|------|-------------|
| User → UI | 30080 or 443 | pcap-frontend |
| frontend → ui-portal | 8080 | pcap-frontend |
| ui-portal → engine | 8443 gRPC | frontend → capture |
| sensors → engine | 19000–19999 TCP | capture (hostNetwork) |
| ui-portal → K8s API | 6443/443 | frontend → user cluster |
| ui-portal → neo4j | 7687 | frontend |
| ui-portal → S3 | 443 | frontend (Medium/Peak) |

---

## 8. Small tier — default deployment spec

This is the **baseline** to ship in `manifests/base` and `overlays/small`.

```yaml
# Replicas (Small)
spcg-frontend:      1
spcg-ui-portal:     1
spcg-backend-engine: 1
spcg-neo4j:         0   # omit or 1 with minimal resources

# Admission (ConfigMap)
MAX_CONCURRENT_SESSIONS: 2
MAX_PODS_PER_SESSION: 10
MAX_CAPTURE_DURATION: 15m
MAX_CAPTURE_BYTES: 100Mi
S3_OFFLOAD_ENABLED: false
```

**Expected capacity:** 1–2 light captures, ≤10 pods each, dev/single-team use.

---

## 9. Medium tier — production team

```yaml
spcg-frontend:      2
spcg-ui-portal:     2   # sessionAffinity required
spcg-backend-engine: 1   # larger resources
spcg-neo4j:         1

MAX_CONCURRENT_SESSIONS: 5
MAX_PODS_PER_SESSION: 15
S3_OFFLOAD_ENABLED: true   # spill when over limits
```

**Expected capacity:** 3–5 concurrent captures, namespaces up to ~20 nodes, PCAP to S3 when large.

---

## 10. Peak tier — incident / burst

```yaml
spcg-frontend:      3+  (HPA 2–6)
spcg-ui-portal:     3   (HPA 2–5, sticky sessions)
spcg-backend-engine: 2+  (HPA or static 2–4)
spcg-neo4j:         1   (PVC, 4–8Gi heap)

MAX_CONCURRENT_SESSIONS: 8
MAX_PODS_PER_SESSION: 20
S3_OFFLOAD_ENABLED: true
```

**Expected capacity:** 5–8 heavy sessions with strict rate caps; dedicated capture node pool recommended.

---

## 11. Operational checklist

### Install Small (default)

```bash
kubectl apply -k manifests/overlays/small
# or: kubectl apply -k manifests/kubernetes  # once base = Small
```

### Promote to Medium

```bash
kubectl apply -k manifests/overlays/medium
kubectl rollout status -n pcap-frontend deploy/spcg-ui-portal
kubectl rollout status -n pcap-capture deploy/spcg-backend-engine
```

### Promote to Peak

```bash
kubectl apply -k manifests/overlays/peak
kubectl get hpa -n pcap-frontend
```

### Roll back

```bash
kubectl apply -k manifests/overlays/small
```

---

## 12. Roadmap vs tier (code, not K8s)

| Capability | Small (now) | Medium (config) | Peak (code + K8s) |
|------------|-------------|-----------------|-------------------|
| Per-session DaemonSet | ✓ | ✓ | → shared sensor |
| Portal RAM PCAP | ✓ (≤ caps) | optional | S3 required |
| S3 streaming (GUI) | ✓ optional | ✓ required | ✓ required |
| Neo4j + Sigma graph | ✓ | ✓ | ✓ + PVC |
| 5 sessions/sensor | — | partial | shared sensor + fanout |
| Stateless portal | — | sticky replicas | Redis session store |

---

## 13. Decision summary

| Question | Answer |
|----------|--------|
| Backend per tenant? | **No** — one capture plane; isolate by session id + auth |
| Scale frontend? | **Yes** — first horizontal scale target |
| Scale engine? | **When ingest CPU/lag rises** — second scale target |
| Default size? | **Small** — scale with overlays/HPA, not code forks |
| PCAP during capture? | **RAM** if under caps (Small); **S3 stream** when user selects S3 or tier requires it |
| Scale tiers? | **`kubectl apply -k manifests/overlays/{small,medium,peak}`** — no code fork |

---

*Overlays: `manifests/overlays/small` (default), `medium`, `peak` — encode Sections 8–10 via ConfigMap + replica patches.*
