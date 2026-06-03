# Kubernetes vs OpenShift

SPCG runs on **any CNCF-compliant Kubernetes cluster** (1.26+) with eBPF-capable nodes (kernel 5.8+). OpenShift is fully supported with dedicated manifests including **Neo4j PVC**, Routes, and SCC.

For full deployment topology see **[DEPLOYMENT.md](./DEPLOYMENT.md)** and **[ARCHITECTURE.md](./ARCHITECTURE.md)**.

## Compatibility matrix

| Feature | Vanilla Kubernetes | OpenShift |
|---------|-------------------|-----------|
| User auth (kubeconfig upload) | Yes | Yes |
| User auth (bearer token) | Yes | Yes (OAuth token) |
| Namespace / workload listing | Yes (RBAC-scoped) | Yes |
| Owner-based capture | Yes | Yes |
| netobserv eBPF sensors | Yes (privileged or CAP_BPF) | Yes (SCC `privileged` binding) |
| UI access | NodePort `:30080` on `spcg-frontend` | Routes `spcg` + `spcg-api` |
| **Neo4j graph store** | In-cluster (`emptyDir` in base) | **10Gi PVC** + peak heap patches |
| Privileged capture SA | `securityContext.privileged` | `manifests/openshift/rbac-capture.yaml` + SCC |

## Deploy on vanilla Kubernetes

```bash
kubectl apply -k manifests/overlays/small
```

Uses **NodePort `30080`** on `spcg-frontend`. Open: **http://\<node-ip\>:30080**

## Deploy on OpenShift

```bash
oc apply -k manifests/overlays/openshift-small    # Small
oc apply -k manifests/overlays/openshift-medium   # Medium (HA + S3)
oc apply -k manifests/overlays/openshift-peak     # Peak (+ Neo4j sizing)
```

Then:

```bash
oc get route -n pcap-frontend
```

Routes terminate TLS at the edge; `spcg-api` has a **5 minute** HAProxy timeout for SSE capture streams.

Ensure the capture DaemonSet can run with **privileged** SCC (binding included in openshift RBAC). Pod Security **privileged** on `pcap-capture` is required for dynamic sensors. **Neo4j** runs in `pcap-frontend` (restricted) with a dedicated PVC.

## Cilium CNI

Clusters using **Cilium** are supported; tolerations include `node.cilium.io/agent-not-ready` on all frontend/capture/Neo4j deployments.

## netobserv CLI binary

The engine deploys **containerized** `netobserv-ebpf-agent` images. You do not need `oc-netobserv` on vanilla K8s unless you use the CLI separately for debugging.

## Local development

- **Kubeconfig mode**: point the UI at any cluster in your uploaded config.
- **Token mode**: the ui-portal process must reach the same API server (set `KUBECONFIG` for the portal when running locally).
