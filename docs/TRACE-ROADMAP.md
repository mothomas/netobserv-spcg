# Packet Trace (tracer) — feature roadmap

**Integration branch:** `tracer`  
**Deploy manifest:** `manifests/openshift-secure`  
**Images:** `quay.io/moby/spcg-ui-portal:tracer-20260607`, `quay.io/moby/spcg-frontend:tracer-20260607`

`feature/trace-probe-paint` is **abandoned** — do not build or deploy from it. All tracer work lands on `tracer` first, then `main` when stable.

---

## Framework (stick to this)

SPCG layers (`frontend/lib/sections.ts`):

| Layer | Boundary | Trace role |
|-------|----------|------------|
| **Workspace** | Namespace / pod selection | Launch trace from scoped workloads |
| **Packet Trace** | Cross-namespace + IP endpoints | Endpoint A ↔ B → infra path map |
| **Capture** | Namespace RBAC | Optional on-demand capture for trace source pods |
| **Flow graph** | Active capture session | Separate from trace discovery |
| **App network** | Capture selection | L7 view inside capture boundary |

**Go packages:**

```text
internal/trace/           catalog, layout, graph_builder, metallb_trace
internal/portal/          trace_handlers, trace_capture_handlers, trace_registry
internal/graph/neo4j/     trace_sigma, ReplaceTraceGraph
frontend/components/trace/  TraceWorkbench, TraceFlowCanvas, TraceEndpointSelector
frontend/lib/trace.ts     API client
```

**Rules:** UI → portal handlers → `internal/trace/*`. No trace logic in Next.js API routes beyond proxy.

---

## Current state on `tracer` (shipped)

| Area | Status |
|------|--------|
| Endpoint A / B (symmetric NS or IP) | Done |
| Source → destination discovery | Done |
| Path map (COP) + Sigma graph toggle | Done |
| Ingress / egress / host path tables | Done |
| Trace-scoped capture start/stop | Done |
| Neo4j Sigma projection (≤80 nodes) | Done |
| Session registry + RBAC owner | Done |
| OpenShift secure images `tracer-20260607` | Pinned in kustomization |

---

## Roadmap

### T1 — Discovery quality (next)

| # | Feature | Package |
|---|---------|---------|
| T1.1 | MetalLB VIP → Service → Route path completeness | `metallb_trace`, `catalog` |
| T1.2 | Multus secondary network in path context | `catalog`, `dynamic` |
| T1.3 | Dual swimlane layout (ingress / egress / anchor) | `layout`, `track` |
| T1.4 | Graph simplify + path focus filters | `graph_simplify`, `path_focus` |
| T1.5 | Trace graph filters in UI | `TraceGraphFilters`, `traceGraphFilter` |

### T2 — Trace + capture UX

| # | Feature | Package |
|---|---------|---------|
| T2.1 | Trace status in workbench header (capture active, event count) | `trace_capture_handlers`, UI |
| T2.2 | Clear “discovery vs capture” copy in workbench | `TraceWorkbench` |
| T2.3 | Link from trace to flow graph when capture running | `page.tsx`, `sections` |

### T3 — Cross-namespace hardening

| # | Feature | Package |
|---|---------|---------|
| T3.1 | Namespace allowlist on trace discover | `catalog`, `k8s/access` |
| T3.2 | Sanitize graph for out-of-scope hops | `rbac` (if added) |
| T3.3 | IDOR tests for trace session / graph / capture | tests |

### T4 — Optional depth (later)

| # | Feature | Notes |
|---|---------|-------|
| T4.1 | OVN SBDB read-only hop enrichment | New discoverer behind flag |
| T4.2 | Persistent trace topology in Neo4j | Beyond session Sigma |
| T4.3 | Live hop verification | Separate effort; not on `tracer` scope now |

---

## Branch & deploy workflow

```bash
git fetch origin
git checkout tracer
git pull origin tracer

# Build portal + frontend from tracer, tag e.g. tracer-YYYYMMDD
# Update manifests/openshift-secure/kustomization.yaml newTag

oc apply -k manifests/openshift-secure
oc rollout restart deployment/spcg-ui-portal -n spcg-control
oc rollout restart deployment/spcg-frontend -n spcg-landing
oc get deploy spcg-ui-portal -n spcg-control \
  -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'
```

**Do not** deploy from `main` for Packet Trace — `main` does not include tracer work until merge.

---

## Merge to `main` (when ready)

1. Soak `tracer` on OpenShift with tagged images  
2. README + docs index updated  
3. PR `tracer` → `main`  
4. Retire `feature/trace-probe-paint` on remote (optional `git push origin --delete feature/trace-probe-paint`)
