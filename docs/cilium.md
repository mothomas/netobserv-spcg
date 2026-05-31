# SPCG on clusters with Cilium CNI

**Short answer:** SPCG and netobserv work on vanilla Kubernetes with **Cilium** networking. You should not hit a hard blocker, but you may need light tuning when Cilium and netobserv both load eBPF programs on the same nodes.

## Why it usually works

- NetObserv is **CNI-agnostic** — it attaches to host/veth interfaces and enriches flows via the Kubernetes API, not via Cilium APIs.
- SPCG deploys the same **netobserv-ebpf-agent** image the [netobserv-cli](https://github.com/netobserv/netobserv-cli) uses for packet capture.
- Our sensor DaemonSet already sets `TC_ATTACH_MODE=any`, which lets the agent pick **TCX** when the kernel supports it (Linux 6.6+). TCX chains multiple eBPF TC programs so **Cilium and netobserv can coexist** without one replacing the other.

## What can go wrong

| Issue | Cause | Mitigation |
|-------|--------|------------|
| Agent fails to start / TC attach error | Cilium + netobserv both use TC; old kernels use netlink attach | Kernel **5.8+** (required anyway); prefer **6.6+** for TCX; on conflict set Cilium `bpf.tc.priority` > 1 (see [OBI/Cilium notes](https://opentelemetry.io/docs/zero-code/obi/cilium-compatibility/)) |
| Duplicate packets / inflated byte counts | Same flow seen on `eth0`, `cilium_*`, and `lxc*` veth | Enable dedup: `DEDUPER=firstCome` on the sensor |
| Missing or odd pod metadata on ingress | Encrypted overlay / tunnel paths (common with Cilium) | Expected in some paths; filter by `SrcK8S_*` / `DstK8S_*` still works for workload-scoped capture |
| High CPU on busy nodes | Two eBPF observability stacks (Cilium Hubble + netobserv) | Scope captures (owner filters), avoid cluster-wide idle sensors, limit concurrent sessions |
| Hubble + SPCG together | Both observe traffic | Supported; watch node CPU and eBPF map limits |

## Recommended sensor env (Cilium)

Set on `spcg-backend-engine` or per-session DaemonSet (via future chart values):

```yaml
env:
  - name: TC_ATTACH_MODE
    value: "tcx"          # use when kernel >= 6.6; else keep "any"
  - name: DEDUPER
    value: "firstCome"  # reduces duplicate flows across cilium + veth
  - name: EXCLUDE_INTERFACES
    value: "lo,cilium_net,cilium_host"  # adjust after: ip link on a node
  # Optional: only pod-facing paths
  # - name: INTERFACES
  #   value: "/lxc.+/,/eth0/"
```

Discover interface names on a node:

```bash
kubectl debug node/<node> -it --image=nicolaka/netshoot -- ip -br link
```

## What SPCG does *not* depend on

- CiliumNetworkPolicy CRDs (capture is eBPF + K8s metadata, not policy API).
- OpenShift OVN / `br-ex` interfaces (OpenShift-specific; Cilium uses different naming).
- `oc-netobserv` CLI on the node (agents run in containers).

## Verification checklist

1. Kernel: `uname -r` → 5.8+ on capture nodes.
2. Deploy `pcap-capture` with privileged or BPF/NET_ADMIN/PERFMON.
3. Start a **single-pod** capture; confirm SSE chunks in the UI.
4. If the sensor pod crashes, check logs for `TC` / `attach` errors → apply table above.
5. If counts look doubled, enable `DEDUPER=firstCome` and tighten `EXCLUDE_INTERFACES`.

## Running alongside Cilium Hubble

Hubble and SPCG serve different goals (service map / DNS vs filtered PCAP for triage). They can run on the same cluster; isolate capture to namespaces/workloads you are debugging to limit overhead.
