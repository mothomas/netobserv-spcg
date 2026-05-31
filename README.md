# Secure Packet Capture Gateway (SPCG)

Namespace-scoped, zero-trust packet capture platform wrapping [netobserv-cli](https://github.com/netobserv/netobserv-cli) with a Next.js dashboard and dual-tier Go services.

## Namespace layout

| Namespace | Tier | Workloads | PSS |
|-----------|------|-----------|-----|
| **`pcap-capture`** | Capture | `spcg-backend-engine`, `spcg-sniffer` (netobserv eBPF DaemonSet) | `privileged` |
| **`pcap-frontend`** | Frontend | `spcg-ui-portal` (Go API), `spcg-frontend` (Next.js) | `restricted` |

The UI portal in `pcap-frontend` calls the gRPC engine in `pcap-capture` over mTLS. On capture start, the engine **deploys netobserv eBPF sensors** (DaemonSet manifests derived from [netobserv-cli](https://github.com/netobserv/netobserv-cli)) and receives packets via [flowlogs-pipeline](https://github.com/netobserv/flowlogs-pipeline) gRPC — the same path the CLI uses locally.

```
┌──────────────────────── pcap-frontend ────────────────────────┐
│  Route /  → spcg-frontend (Next.js)                             │
│  Route /api → spcg-ui-portal (OAuth impersonation, SSE)        │
└────────────────────────────┬──────────────────────────────────┘
                             │ gRPC :8443
┌──────────────────────── pcap-capture ─────────────────────────┐
│  spcg-backend-engine  ←  spcg-sensor-{session} (netobserv eBPF DS) │
└────────────────────────────────────────────────────────────────┘
```

## Quick start (local dev)

```bash
make build
NETOBSERV_BIN=oc-netobserv ./bin/backend-engine
ENGINE_GRPC_ADDR=localhost:8443 ./bin/ui-portal
cd frontend && npm install && npm run dev
```

## Deploy (OpenShift)

```bash
kubectl apply -k manifests/
```

Or Helm:

```bash
helm upgrade --install spcg ./charts/spcg
```

## Authentication

| Method | Use case |
|--------|----------|
| **Kubeconfig upload** | Vanilla Kubernetes, local dev, any cluster in your config |
| **Bearer token** | OpenShift OAuth, CI tokens, `kubectl create token` |

Login via `POST /api/v1/auth/login` returns a session id (`X-SPCG-Session`). Kubeconfig bytes are held **in memory only** on the ui-portal and wiped on logout.

See [docs/kubernetes-vs-openshift.md](docs/kubernetes-vs-openshift.md) for platform differences.

## Container images (CI)

GitHub Actions builds and pushes three images on push to `main` and on `v*` tags. Configure [Docker Hub or Quay secrets](docs/ci-cd.md) once, then use:

- `docker.io/mothomas/spcg-backend-engine`
- `docker.io/mothomas/spcg-ui-portal`
- `docker.io/mothomas/spcg-frontend`

## Security

- Kubernetes API: user's kubeconfig identity or bearer token (RBAC-scoped) via `spcg-ui-portal`.
- `pcap-frontend`: `automountServiceAccountToken: false` on UI pods.
- `pcap-capture`: default-deny ingress; engine accepts gRPC only from `spcg-ui-portal` in `pcap-frontend`; sniffer accepts traffic only from `spcg-backend-engine`.
- AI triage credentials: in-memory per session only.
