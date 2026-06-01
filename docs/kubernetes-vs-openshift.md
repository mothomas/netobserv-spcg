# Kubernetes vs OpenShift

SPCG runs on **any CNCF-compliant Kubernetes cluster** (1.26+) with eBPF-capable nodes (kernel 5.8+). OpenShift is fully supported but not required.

## Compatibility matrix

| Feature | Vanilla Kubernetes | OpenShift |
|---------|-------------------|-----------|
| User auth (kubeconfig upload) | Yes | Yes |
| User auth (bearer token) | Yes | Yes (OAuth token) |
| Namespace / workload listing | Yes (RBAC-scoped) | Yes |
| Owner-based capture | Yes | Yes |
| netobserv eBPF sensors | Yes (privileged or CAP_BPF) | Yes (SCC or capabilities) |
| UI access | NodePort `:30080` on `spcg-frontend` | `manifests/openshift/route-openshift.yaml` |
| Privileged capture SA | `securityContext.privileged` | `manifests/openshift/rbac-capture.yaml` + SCC |

## Deploy on vanilla Kubernetes

```bash
kubectl apply -k manifests/
```

Uses **NodePort `30080`** on `spcg-frontend` (UI + `/api` proxied to ui-portal in-cluster). No Ingress or Cilium LB required.

Open: **http://\<node-ip\>:30080**

**OpenShift:**

```bash
kubectl apply -k manifests/openshift/
```

Ensure the capture DaemonSet can run with **privileged** or **BPF + NET_ADMIN + PERFMON** (see netobserv-ebpf-agent docs). Pod Security **privileged** on `pcap-capture` is required for the dynamic sensors.

## Cilium CNI

Clusters using **Cilium** are supported; control-plane tolerations include `node.cilium.io/agent-not-ready` where needed.

## netobserv CLI binary

The engine deploys **containerized** `netobserv-ebpf-agent` images. You do not need `oc-netobserv` on vanilla K8s unless you use the CLI separately for debugging.

## Local development

- **Kubeconfig mode**: point the UI at any cluster in your uploaded config; no portal-side cluster URL needed.
- **Token mode**: the ui-portal process must reach the same API server (set `KUBECONFIG` for the portal when running locally).
