# Secure Packet Capture Gateway (SPCG)

Namespace-scoped, zero-trust packet capture platform wrapping [netobserv-cli](https://github.com/netobserv/netobserv-cli) with a Next.js dashboard and dual-tier Go services.

## Documentation

| Document | Description |
|----------|-------------|
| [**docs/README.md**](docs/README.md) | Documentation index |
| [**docs/ARCHITECTURE.md**](docs/ARCHITECTURE.md) | Design ideology, concepts, data flows, decision log |
| [**docs/DEPLOYMENT.md**](docs/DEPLOYMENT.md) | Manifests, Small/Medium/Peak, OpenShift Routes & SCC |
| [**docs/CODE-STRUCTURE.md**](docs/CODE-STRUCTURE.md) | Repository layout, packages, security patterns |
| [docs/architecture-tiers.md](docs/architecture-tiers.md) | Tier sizing and scaling roadmap |
| [docs/kubernetes-vs-openshift.md](docs/kubernetes-vs-openshift.md) | Platform quick compare |
| [docs/neo4j-graph.md](docs/neo4j-graph.md) | Graph store and tenant crypto |
| [docs/ci-cd.md](docs/ci-cd.md) | Image build and registry |
| [docs/lab-random-scanner.md](docs/lab-random-scanner.md) | Lab threat-sim (branch `lab/random-scanner` only) |

## Namespace layout

| Namespace | Tier | Workloads | PSS |
|-----------|------|-----------|-----|
| **`pcap-capture`** | Capture | `spcg-backend-engine`, per-session netobserv eBPF DaemonSet | `privileged` |
| **`pcap-frontend`** | Frontend | `spcg-ui-portal` (Go API), `spcg-frontend` (Next.js), `spcg-neo4j` | `restricted` |

The UI portal in `pcap-frontend` calls the gRPC engine in `pcap-capture` over mTLS (optional). On capture start, the engine **deploys netobserv eBPF sensors** (DaemonSet manifests derived from [netobserv-cli](https://github.com/netobserv/netobserv-cli)) and receives packets via [flowlogs-pipeline](https://github.com/netobserv/flowlogs-pipeline) gRPC.

```
┌──────────────────────── pcap-frontend ────────────────────────┐
│  Route / NodePort → spcg-frontend (Next.js)                     │
│  /api → spcg-ui-portal (OAuth/kubeconfig, SSE)                 │
└────────────────────────────┬──────────────────────────────────┘
                             │ gRPC :8443
┌──────────────────────── pcap-capture ─────────────────────────┐
│  spcg-backend-engine  ←  spcg-sensor-{session} (netobserv eBPF) │
└────────────────────────────────────────────────────────────────┘
```

## Quick start (local dev)

```bash
make build
./bin/backend-engine
ENGINE_GRPC_ADDR=localhost:8443 ./bin/ui-portal
cd frontend && npm install && npm run dev
```

## Deploy

| Profile | Command |
|---------|---------|
| **Small** (vanilla K8s) | `kubectl apply -k manifests/overlays/small` |
| **Medium** | `kubectl apply -k manifests/overlays/medium` |
| **Peak** | `kubectl apply -k manifests/overlays/peak` |
| **OpenShift Small** | `oc apply -k manifests/overlays/openshift-small` |
| **OpenShift Medium** | `oc apply -k manifests/overlays/openshift-medium` |
| **OpenShift Peak** | `oc apply -k manifests/overlays/openshift-peak` |

Vanilla UI: **http://\<node-ip\>:30080** (NodePort). OpenShift: `oc get route -n pcap-frontend`.

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for overlay inheritance, network policies, and combining OpenShift with tier admission.

## Authentication

| Method | Use case |
|--------|----------|
| **Kubeconfig upload** | Vanilla Kubernetes, local dev |
| **Bearer token** | OpenShift OAuth, CI tokens |

Login via `POST /api/v1/auth/login` returns `X-SPCG-Session`. Credentials are held **in memory only** on ui-portal and wiped on logout.

## Container images (CI)

See [docs/ci-cd.md](docs/ci-cd.md). Published names:

- `docker.io/mothomas/spcg-backend-engine`
- `docker.io/mothomas/spcg-ui-portal`
- `docker.io/mothomas/spcg-frontend`

## Security (implemented)

- Kubernetes API: user's kubeconfig or bearer (RBAC-scoped) via `spcg-ui-portal`.
- `pcap-frontend`: `automountServiceAccountToken: false`, non-root UI pods, default-deny egress NetworkPolicies.
- `pcap-capture`: default-deny ingress; engine accepts gRPC from ui-portal and collector ports 19000–19999 from sensors.
- Capture session ownership enforced per auth session.
- AI outbound scrubbing (IPs, tokens, MACs) before external LLM calls.
- Offline IP→country map bundled in frontend (no live geo APIs).

Details: [docs/CODE-STRUCTURE.md](docs/CODE-STRUCTURE.md#5-security-practices-in-code-keep-these).

## Security hardening TODO

Items below are **known gaps**. Do **not** remove the related feature to “fix” security—the product depends on it. Harden incrementally and check off here.

- [ ] Replace placeholder secrets (`spcg-neo4j-auth`, `spcg-graph-master-key`) with External Secrets / SealedSecrets / vault injection
- [ ] Require `spcg-engine-mtls` in production overlays (today gRPC falls back to insecure if secret missing)
- [ ] Restrict `CORS_ORIGIN` on ui-portal (default `*` is convenient for dev only)
- [ ] Add shared session store (e.g. Redis) so ui-portal replicas are HA-safe; keep sticky Service until then
- [ ] Narrow `allow-ui-portal-k8s-api-egress` to cluster API CIDR + tenant S3 endpoint CIDRs
- [ ] Add PodDisruptionBudgets for ui-portal and engine at Medium/Peak
- [ ] Neo4j persistent volume + backup for Peak (base uses `emptyDir`)
- [ ] Evaluate netobserv **capabilities-only** SCC/profile vs `privileged` on OpenShift (requires sensor regression tests)
- [ ] TLS for vanilla ingress (Ingress or NodePort terminator)—today HTTP to NodePort
- [ ] OpenShift Route **re-encrypt** or passthrough if in-cluster HTTP is not acceptable
- [ ] Set `imagePullPolicy: IfNotPresent` and document disconnected registry mirroring for airgap
- [ ] Centralize active capture counting for admission (today tied to active SSE streams)
- [ ] Optional slim `ip-country-map.json` build for constrained image sizes
- [ ] GitOps overlay: `openshift` + `medium`/`peak` kustomization root (today combined manually)
- [ ] Engine HPA / shard-by-node-pool for Peak (documented in architecture-tiers, not all in YAML)
- [ ] NetworkPolicy scoped egress for AI provider endpoints when AI analyst is enabled
