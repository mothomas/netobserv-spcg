# SPCG code structure and engineering conventions

This document maps the repository to runtime components, describes package boundaries, and records **security practices** plus **known gaps** (tracked as README TODOs — not removed, because removal would break functionality).

---

## 1. Repository map

```text
netobserv-spcg/
├── cmd/
│   ├── backend-engine/main.go    # gRPC capture plane entrypoint
│   └── ui-portal/main.go         # HTTP API, CORS, engine/Neo4j wiring
├── api/proto/capture/v1/         # CaptureService gRPC contract
├── internal/                     # All production Go (not importable externally)
├── frontend/                     # Next.js 14 dashboard
├── manifests/                    # Kustomize (see DEPLOYMENT.md)
├── deploy/                       # Dockerfiles
├── charts/spcg/                  # Helm chart
├── scripts/                      # Tooling (lab scripts on branch lab/random-scanner only)
├── docs/                         # Architecture & operations docs
└── Makefile                      # proto, build, run targets
```

---

## 2. Go packages (`internal/`)

### 2.1 Layering rules

| Rule | Rationale |
|------|-----------|
| `cmd/*` only wires config and servers | Keeps `main` thin |
| `portal` may import `pcap`, `capture`, `k8s`, `auth`, `ai`, `graph` | API layer orchestrates domain |
| `pcap` must not import `portal` | Domain stays independent |
| `capture` owns engine + sensor; `pcap` owns session analytics | Clear data-plane split |
| `ai` must not import `pcap` directly | Avoid cycles; graph context built in `portal/graphcontext.go` |

### 2.2 Package reference

| Package | Path | Responsibility |
|---------|------|----------------|
| **auth** | `internal/auth/` | Session store, kubeconfig/bearer parsing, wipe on logout |
| **k8s** | `internal/k8s/` | User-scoped clientset, workload list, capture target resolution |
| **capture** | `internal/capture/` | gRPC engine server, admission limits |
| **capture/sensor** | `internal/capture/sensor/` | DaemonSet render/apply, collectors, pod matching |
| **pcap** | `internal/pcap/` | Sessions, topology, PCAP-NG, S3 sink, bounded graph |
| **portal** | `internal/portal/` | HTTP handlers, SSE capture stream, registry, graph/AI routes |
| **graph/neo4j** | `internal/graph/neo4j/` | Neo4j store, Sigma projection, ReplaceTopology |
| **graph/tenantcrypto** | `internal/graph/tenantcrypto/` | AES-GCM label encryption per auth session |
| **ai** | `internal/ai/` | Scrubber, providers, triage/chat message builders |
| **tlsutil** | `internal/tlsutil/` | Optional mTLS for engine gRPC |

### 2.3 Request path (portal)

```text
HTTP Request
  → middleware / CORS (cmd/ui-portal)
  → handlers.go (route table, register*Routes)
  → requireCaptureAccess / authSessionID
  → domain: pcap.Session | capture.Engine | k8s.UserClient | neo4j.Store
```

Key files:

- `internal/portal/handlers.go` — core REST + SSE
- `internal/portal/capture_registry.go` — capture ownership
- `internal/portal/observability_handlers.go` — lightweight topology refresh
- `internal/portal/graph_handlers.go` — Sigma graph (in-memory + async Neo4j)
- `internal/portal/ai_handlers.go` — AI context/chat

### 2.4 Capture path (engine)

```text
gRPC StreamPackets
  → grpc_server.go
  → sensor.Manager.StartSession
  → apply DaemonSet (embedded YAML)
  → collector listens 19000-19999
  → packet.go parses flows → portal stream
```

---

## 3. Frontend structure (`frontend/`)

| Area | Path | Role |
|------|------|------|
| App shell | `app/page.tsx`, `components/layout/` | Navigation, capture lifecycle, polling |
| API proxy | `app/api/v1/[...path]/route.ts` | Server-side forward to `UI_PORTAL_URL` |
| API client | `lib/api.ts`, `lib/ai.ts`, `lib/graph.ts` | Typed fetch helpers |
| Graph UI | `components/SigmaTopologyGraph.tsx`, `lib/sigmaGraphStyle.ts` | Sigma.js + flags |
| Geo (offline) | `lib/ipCountryLookup.ts`, `lib/ipAddress.ts` | DB-IP map, no external geo HTTP |
| Network plot | `lib/networkplot/` | Alternate force layout (legacy path) |
| Styling | `app/globals.css`, `tailwind.config.ts` | Airgap-safe fonts (no Google CDN) |

**Convention:** UI state is **session-scoped in React**; authoritative capture state is always on ui-portal (SSE + REST).

---

## 4. Architectural cleanliness checklist

| Area | Status | Notes |
|------|--------|-------|
| Separation capture vs UI namespaces | Good | Mirrored in manifests |
| Domain package boundaries | Good | Cycle fixes documented (graphcontext in portal) |
| Tier logic in ConfigMap not `if tier` in code | Good | `admission/limits.go` reads env |
| Stress-path observability | Good | `topology_limits.go`, dedicated observability route |
| Embedded sensor manifests | Acceptable | `sensor/embed.go` — version with engine image |
| In-memory auth/sessions | Debt | Documented TODO for Redis |
| Optional insecure gRPC | Debt | TLS secret optional by design for lab |
| Image tag drift in kustomize | Careful | `images.name` must match deployment image names |

---

## 5. Security practices in code (keep these)

| Practice | Implementation | If removed |
|----------|----------------|------------|
| User-scoped K8s API | `k8s.NewUserClient`, kubeconfig in `auth.Store` | **Breaks** namespace/workload pickers |
| Credential wipe on logout | `auth.Wipe`, session `wipeLocked` | **Breaks** compliance story; tokens linger |
| Capture ownership checks | `capture_registry.go` | **Breaks** multi-user isolation on shared portal |
| Admission limits | `capture/admission/limits.go` | **Breaks** tier caps; OOM risk |
| AI scrubbing before LLM | `ai/scrubber.go`, `sanitize.go` | **Breaks** safe use of external AI |
| Bounded topology | `topology_limits.go` | **Breaks** UI under high packet rates |
| Frontend non-root + no SA token | `deployment-frontend.yaml` | **Breaks** restricted PSS compliance |
| NetworkPolicy default deny | `network-policies.yaml` | **Breaks** zero-trust network model |
| Tenant graph crypto | `tenantcrypto` + `GRAPH_MASTER_KEY` | **Breaks** multi-tenant graph isolation |
| S3 direct upload | `s3sink.go` | **Breaks** Medium/Peak large capture path |
| `automountServiceAccountToken: false` on UI | deployment spec | **Breaks** restricted pod hardening |
| In-flight capture stream guard (frontend) | `flowsInFlightRef` | **Breaks** stress stability (request pile-up) |

---

## 6. Security gaps → README TODO (harden, do not delete feature)

These are **intentional or transitional** weaknesses. Fixing them requires additive hardening, not feature removal.

| Gap | Risk | Hardening direction |
|-----|------|-------------------|
| Placeholder Neo4j/graph secrets in YAML | Credential leak | External Secrets / SealedSecrets |
| gRPC insecure without `spcg-engine-mtls` | MITM inside cluster | Require mTLS in prod overlay |
| CORS `*` default on ui-portal | CSRF-like abuse from browsers | Restrict `CORS_ORIGIN` per environment |
| In-memory sessions | No true HA for portal replicas | Redis + sticky ingress |
| Broad egress 443/6443 on ui-portal | Data exfil path | NetworkPolicy with S3/API CIDR allowlist |
| hostNetwork sensor ingress rule | Less precise NP identity | Document + optional node selector pools |
| Neo4j `emptyDir` only in base | Data loss on restart | PVC patch for Peak |
| Engine `privileged: true` | Large attack surface | Capability-based SCC (OpenShift) + testing |
| NodePort HTTP (vanilla) | No TLS to UI | Ingress TLS termination |
| OpenShift Route edge TLS only | HTTP inside cluster | Re-encrypt or passthrough mTLS |
| `imagePullPolicy: Always` | Airgap pull failures | `IfNotPresent` + mirrored registry |
| Active capture count = SSE streams only | Admission bypass edge case | Centralized session counter |
| 39MB geo JSON in frontend image | Image size | Optional slim map build tag |

Full checklist lives in the root [README.md](../README.md#security-hardening-todo).

---

## 7. Testing layout

| Package | Test files |
|---------|------------|
| `pcap` | `topology_test.go`, `topology_limits_test.go`, `analytics_test.go`, … |
| `capture/admission` | `limits_test.go` |
| `capture/sensor` | `packet_test.go`, `podmatch_test.go` |

Run:

```bash
go test ./internal/...
```

Frontend: `npm run build` (typecheck + lint).

---

## 8. Protocol and codegen

```bash
make proto   # regenerates from api/proto/capture/v1/capture.proto
```

Engine and portal share generated gRPC stubs under `api/` or internal generated paths (see `Makefile`).

---

## 9. Related documents

- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [DEPLOYMENT.md](./DEPLOYMENT.md)
- [neo4j-graph.md](./neo4j-graph.md)
