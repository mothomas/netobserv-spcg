# Packet Trace — graph plotting strategy (Sigma.js)

**Branch:** `tracer`  
**Goal:** Plot every plausible K8s network path in a **readable** way, with **ingress and egress as separate directed flows** (they often share only the workload anchor).

**Decision:** Sigma.js is the **primary** path map renderer. Retire the custom SVG “COP timeline” as the main view; keep it only if needed for print/export.

---

## 1. Problem with today’s model

| Today | Why it breaks |
|-------|----------------|
| One flat `TraceGraph` (nodes + edges) | Ingress hops and egress hops are merged |
| Layout by **rank column** (0→5) | Implies one left-to-right timeline |
| `PathSummary` is a **table row**, not a drawable chain | UI cannot highlight “this ingress option” vs “that egress option” |
| `TraceFlowCanvas` (custom SVG) + `SigmaTopologyGraph` (flow capture UX) | Two renderers, neither models path options |
| Edges lack `path_id` / `direction` membership | Cannot style or filter per path |

**K8s reality:** Traffic **into** a pod and traffic **out of** a pod are different graphs that meet at the **anchor workload** (source pod for egress trace, dest pod for ingress-to-dest trace).

```text
INGRESS (into workload)          ANCHOR           EGRESS (out of workload)
Client → LB → Route → Svc  →  [ Pod ]  →  OVN → Node → EgressIP → Internet
     (path option A)              │              (path option X)
MetalLB VIP → Svc          →    │         →  bond0 → external IP
     (path option B)              │
NodePort → Svc             →    │         →  EgressService → dest
```

---

## 2. Target model: path-first discovery

### 2.1 Core types (extend `internal/trace/types.go`)

```go
// PathDirection classifies flow relative to the anchor workload.
type PathDirection string // ingress | egress | host | context

// PathOption is one discovered route (a chain, not the whole graph).
type PathOption struct {
    ID          string        `json:"id"`
    Direction   PathDirection `json:"direction"`
    Mechanism   string        `json:"mechanism"`   // metallb-bgp, openshift-route, clusterip, egressip, ...
    Label       string        `json:"label"`       // human title for UI
    Status      string        `json:"status"`      // discovered | out_of_scope | theoretical
    Namespace   string        `json:"namespace,omitempty"`
    HopIDs      []string      `json:"hop_ids"`     // ordered node ids
    EdgeIDs     []string      `json:"edge_ids"`    // ordered edge ids
    Confidence  string        `json:"confidence"`  // observed | inferred
}

// TraceNode additions
type TraceNode struct {
    // ... existing fields ...
    Track     string   `json:"track"`      // ingress | egress | anchor | shared | context
    PathRefs  []string `json:"path_refs"`  // path option ids this hop belongs to
}

// TraceEdge additions
type TraceEdge struct {
    // ... existing fields ...
    Direction PathDirection `json:"direction,omitempty"`
    PathRefs  []string      `json:"path_refs,omitempty"`
}
```

### 2.2 Discovery output shape

```json
{
  "graph": {
    "nodes": [ "... shared node pool ..." ],
    "edges": [ "... all edges with path_refs ..." ],
    "path_options": [
      { "id": "ing-metallb-trident", "direction": "ingress", "hop_ids": ["client","vip","svc","pod"] },
      { "id": "egr-egressip-payment", "direction": "egress", "hop_ids": ["pod","ovn","node","egressip","ext"] }
    ],
    "anchor_node_id": "pod/ns/trident-operator-abc"
  }
}
```

**Rule:** `catalog.Resolve` builds **path options first**, then **merges** nodes/edges into the shared graph with `path_refs`. No more “add edge and hope rank sorts it”.

---

## 3. Cluster path inventory (all options to discover)

Each row is a **path option template** the catalog should emit when CRDs/API data match.

### 3.1 Ingress (into source / dest workload)

| Mechanism | K8s objects | Hop chain (typical) | Already in catalog |
|-----------|-------------|---------------------|-------------------|
| **MetalLB L2** | Service LB, IPPool, L2Advertisement | Client → VIP → Service → Pod | Partial |
| **MetalLB BGP** | Service LB, IPPool, BGPAdvertisement, BGPPeer | Client → VIP → Service → Pod (+ BGP peer context) | Partial |
| **OpenShift Route** | Route → Service | Client → Route → Service → Pod | Yes |
| **Ingress (nginx/haproxy)** | Ingress → Service | Client → Ingress → Service → Pod | No |
| **NodePort** | Service NodePort | Client → Node → Service → Pod | Partial |
| **HostNetwork pod** | Pod hostNetwork | Client → Node (host port) → Pod | No |
| **ClusterIP (in-cluster)** | Service ClusterIP | Source Pod → Service → Dest Pod | Partial |
| **Multus secondary** | NAD + attachment | Client/LB → secondary net → Pod (net1) | Partial (NAD node only) |
| **External IP on Service** | Service externalIPs | Client → external IP → Service → Pod | No |

### 3.2 Egress (out of source workload toward destination)

| Mechanism | K8s objects | Hop chain (typical) | Already in catalog |
|-----------|-------------|---------------------|-------------------|
| **OVN EgressIP** | EgressIP CR | Pod → OVN → Node → EgressIP → Dest | Yes |
| **EgressService** | EgressService CR | Pod → EgressService → Dest | Yes |
| **EgressNetwork (OS)** | EgressNetwork | Pod → egress router → Dest | No |
| **Default SNAT (node)** | Node routing | Pod → OVN → Node → bond/uplink → Dest | Partial |
| **Multus egress** | NAD default route on net1 | Pod → secondary iface → Dest | No |
| **Service mesh egress** | Gateway / Egress Gateway | Pod → sidecar → gateway → Dest | No (future) |

### 3.3 Host / CNI context (dimmed, not primary path)

| Mechanism | Purpose |
|-----------|---------|
| OVN logical port | Infra hop on node |
| veth / host-veth | Pod ↔ host |
| bond / NIC | Physical uplink |
| NetworkPolicy | Policy attachment (dashed context edge) |
| NAD | Secondary network definition |

### 3.4 Scope states

| Status | Meaning | UI |
|--------|---------|-----|
| `discovered` | In RBAC namespace scope | Solid path, selectable |
| `out_of_scope` | CR exists, namespace not in scope | Dashed, grey |
| `theoretical` | Inferred (no CR proof) | Dotted |

---

## 4. Layout strategy (readable Sigma plot)

### 4.1 Canonical “hub” layout

```text
┌─────────────────────────────────────────────────────────────────┐
│  INGRESS SWIMLANE (y = 0..1 band)                                │
│  [Client] → [VIP] → [Route] → [Service] ──┐                      │
│  [Client] → [NodePort Svc] ───────────────┼──→ [ANCHOR POD]      │
├───────────────────────────────────────────┼──────────────────────┤
│  ANCHOR (y = mid)                         │                      │
│                              [ trident-operator / 10.0.0.149 ]  │
├───────────────────────────────────────────┼──────────────────────┤
│  EGRESS SWIMLANE (y = 2..3 band)          │                      │
│                    ┌── [OVN] → [Node] → [EgressIP] → [8.8.8.8]  │
│                    └── [bond0] ───────────────────→ [dest pod]   │
└─────────────────────────────────────────────────────────────────┘
│  CONTEXT (bottom band, low opacity): NP, BGP peer, NAD pills     │
└─────────────────────────────────────────────────────────────────┘
```

**Layout rules:**

1. **X-axis = hop depth** within a path (not global rank 0–5).
2. **Y-axis = track band:** ingress above anchor, egress below, context at bottom.
3. **Anchor pod** is the only node allowed in both ingress terminus and egress origin.
4. **Shared infra** (node, OVN) may appear once with `track: shared` if same id on both sides.
5. **Multiple path options** = parallel chains in the same band (offset Y slightly per option).

### 4.2 Coordinates for Sigma

- Backend computes `(x, y)` per node in `internal/trace/layout_paths.go` (new).
- `trace_sigma.go` maps `x,y` directly to Sigma (already does `X: n.X + w/2`).
- Edge `path_refs` drive highlight when user selects a path option in the sidebar.

### 4.3 Sigma.js rendering (`frontend/components/trace/TraceSigmaPathMap.tsx` — new)

Reuse patterns from `SigmaTopologyGraph.tsx`:

| Element | Sigma approach |
|---------|----------------|
| Nodes | `graphology` + `NodeCircleProgram` / icons by `kind` |
| Edges | Curved edges (`type: 'curvedArrow'`), color by `direction` |
| Labels | `renderLabels: true`, grid `labelGridCellSize` |
| Selection | Click path in list → `edgeReducer` / `nodeReducer` highlight `path_refs` |
| Zoom | `ZoomControl`, `fitViewport` on load |
| Context nodes | `hidden: true` until “Show context” toggle |

**Color tokens** (match existing `sigmaGraphStyle.ts`):

| Track | Edge color | Node border |
|-------|------------|-------------|
| ingress | `#60cdff` | accent blue |
| egress | `#34d399` | green |
| anchor | `#94b4ff` | strong accent |
| context | `#64748b` @ 35% opacity | grey |
| policy drop | `#f87171` dashed | red |

---

## 5. UI strategy

### 5.1 Single primary view: Sigma path map

Replace COP/Sigma toggle with:

```text
[ Path map (Sigma) ]     ← default
[ Path list ]            ← sidebar or bottom drawer
[ Table: ingress/egress/host ]  ← keep PathSummary tables
```

### 5.2 Path list panel

- **Ingress (N options)** — radio or multi-select highlight
- **Egress (M options)** — independent selection
- Selecting ingress option A does not hide egress options; both can be highlighted simultaneously (different colors).

### 5.3 Filters (reuse `TraceGraphFilters` concept)

- Mechanism: MetalLB / Route / EgressIP / …
- Scope: discovered only / include out-of-scope
- Context: show NetworkPolicy, NAD, BGP peer

---

## 6. Implementation phases (`tracer` branch)

### Phase G0 — Data model (1 sprint)

| Task | Files |
|------|-------|
| Add `PathOption`, `path_options` on `TraceGraph` | `types.go` |
| Add `track`, `path_refs` on nodes/edges | `types.go` |
| `path_builder.go` — build chains from catalog steps | new |
| Tests: one ingress + one egress chain | `path_builder_test.go` |

### Phase G1 — Discovery refactor (1–2 sprints)

| Task | Files |
|------|-------|
| Refactor `discoverMetalLB` → emit `PathOption` | `catalog.go`, `metallb_trace.go` |
| Refactor `discoverRoutes` → ingress chains | `catalog.go` |
| Refactor `discoverEgress` → egress chains | `catalog.go` |
| Ingress controller (Ingress resource) | new discoverer |
| NodePort / ExternalIP path templates | `catalog.go` |

### Phase G2 — Layout (1 sprint)

| Task | Files |
|------|-------|
| `layout_paths.go` — hub + swimlane coordinates | new |
| Remove rank-column-only layout for trace | `layout.go` deprecate or wrap |
| Sigma projection uses track colors | `trace_sigma.go` |

### Phase G3 — Sigma UI (1 sprint)

| Task | Files |
|------|-------|
| `TraceSigmaPathMap.tsx` — dedicated trace Sigma | new |
| `TracePathOptionList.tsx` — ingress/egress picker | new |
| Wire `TraceWorkbench` default view to Sigma | `TraceWorkbench.tsx` |
| Retire or hide `TraceFlowCanvas` | optional |

### Phase G4 — Polish

| Task | Notes |
|------|-------|
| Multus path options (primary vs net1) | Per-interface chains |
| Dest workload ingress (reverse direction) | When dest is pod not IP |
| Neo4j store path options | Optional persistence |
| Export path diagram PNG | Sigma canvas |

---

## 7. What not to do

- **Do not** force one DAG from client to internet through a single ordered rank.
- **Do not** reuse capture flow graph layout (sequence / pod-to-pod) for infra trace.
- **Do not** add probe-paint until path options plot correctly (abandoned branch).
- **Do not** put layout logic only in frontend — backend owns coordinates for API + Neo4j consistency.

---

## 8. Success criteria

1. User sees **at least one ingress chain** and **at least one egress chain** for a typical OpenShift + MetalLB + EgressIP lab.
2. Selecting an ingress option highlights only that chain; egress chains stay visible but dimmed.
3. All discovered CR types in scope appear in the **path list**, not only in a table.
4. Sigma pan/zoom performs smoothly at 40 nodes / 60 edges.
5. `kubectl kustomize` + `tracer-YYYYMMDD` image deploy shows the new UI on `tracer`.

---

## 9. Related docs

- [TRACE-ROADMAP.md](./TRACE-ROADMAP.md) — branch strategy and phases
- [CODE-STRUCTURE.md](./CODE-STRUCTURE.md) — package boundaries
- `frontend/lib/sections.ts` — Packet Trace layer scope
