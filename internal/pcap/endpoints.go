package pcap

import "strings"

type podRef struct {
	Namespace string
	Name      string
	OwnerKind string
}

// resolveFlowEndpoints maps a capture event to directed workload vertices.
// Uses K8s enrich fields when present; otherwise SrcAddr/DstAddr vs tracked pod IPs.
func resolveFlowEndpoints(ev FlowEvent, tracked []TrackedPod) (from, to TopologyNode) {
	from = nodeFromMeta(ev.FlowMeta, true, "")
	to = nodeFromMeta(ev.FlowMeta, false, "")
	if from.ID != "" && to.ID != "" {
		return from, to
	}

	ipMap := trackedPodIPIndex(tracked)
	srcIP := flowString(ev.FlowMeta, "SrcAddr")
	dstIP := flowString(ev.FlowMeta, "DstAddr")
	if srcIP == "" || dstIP == "" {
		frame := ethernetPayload(ev.Frame)
		fSrc, fDst := ipsFromEthernet(frame)
		if srcIP == "" {
			srcIP = fSrc
		}
		if dstIP == "" {
			dstIP = fDst
		}
	}

	if srcIP != "" && dstIP != "" {
		if from.ID == "" {
			from = nodeForIP(srcIP, ipMap)
		}
		if to.ID == "" {
			to = nodeForIP(dstIP, ipMap)
		}
	}

	cap := nodeFromCapturePod(ev.CapturePod, tracked)
	from, to = orientToCapturePod(from, to, cap, srcIP, dstIP, ipMap)

	if from.ID == "" && cap.ID != "" {
		from = cap
	}
	if from.ID == "" || to.ID == "" {
		return TopologyNode{}, TopologyNode{}
	}
	return from, to
}

// orientToCapturePod ties sensor-scoped packets to the selected pod when IP→pod
// mapping is missing (common without K8s enrich on the packet gRPC path).
func orientToCapturePod(from, to, cap TopologyNode, srcIP, dstIP string, ipMap map[string]podRef) (TopologyNode, TopologyNode) {
	if cap.ID == "" || srcIP == "" || dstIP == "" {
		return from, to
	}
	capRef, hasCap := ipMapRef(cap, ipMap)
	_ = capRef

	srcPod := nodeForIP(srcIP, ipMap)
	dstPod := nodeForIP(dstIP, ipMap)
	srcIsCap := srcPod.ID == cap.ID || (hasCap && srcIP != "" && ipMap[srcIP] == capRef)
	dstIsCap := dstPod.ID == cap.ID || (hasCap && dstIP != "" && ipMap[dstIP] == capRef)

	switch {
	case srcIsCap && !dstIsCap:
		from, to = srcPod, dstPod
		if from.ID == "" {
			from = cap
		}
		if to.ID == "" {
			to = externalNode(dstIP)
		}
	case dstIsCap && !srcIsCap:
		from, to = srcPod, dstPod
		if from.ID == "" {
			from = externalNode(srcIP)
		}
		if to.ID == "" {
			to = cap
		}
	case strings.HasPrefix(from.ID, "ext/") && strings.HasPrefix(to.ID, "ext/"):
		// Pod-scoped capture: treat as egress from the capture target.
		to = externalNode(dstIP)
		from = cap
	}
	return from, to
}

func ipMapRef(cap TopologyNode, ipMap map[string]podRef) (podRef, bool) {
	pod := cap.Pod
	if pod == "" {
		pod = cap.Label
	}
	if cap.Namespace == "" || pod == "" {
		return podRef{}, false
	}
	for _, ref := range ipMap {
		if ref.Namespace == cap.Namespace && ref.Name == pod {
			return ref, true
		}
	}
	return podRef{Namespace: cap.Namespace, Name: pod, OwnerKind: cap.OwnerKind}, true
}

func trackedPodIPIndex(tracked []TrackedPod) map[string]podRef {
	out := make(map[string]podRef)
	for _, p := range tracked {
		ref := podRef{Namespace: p.Namespace, Name: p.Name, OwnerKind: p.OwnerKind}
		ips := p.PodIPs
		if len(ips) == 0 && p.PodIP != "" {
			ips = []string{p.PodIP}
		}
		for _, ip := range ips {
			if ip != "" {
				out[ip] = ref
			}
		}
	}
	return out
}

func nodeForIP(ip string, ipMap map[string]podRef) TopologyNode {
	if ref, ok := ipMap[ip]; ok {
		return podNode(ref)
	}
	return externalNode(ip)
}

func podNode(ref podRef) TopologyNode {
	id := ref.Namespace + "/" + ref.Name
	kind := "Pod"
	if ref.OwnerKind != "" {
		kind = ref.OwnerKind
	}
	return TopologyNode{
		ID: id, Label: ref.Name, Kind: kind,
		Namespace: ref.Namespace, Pod: ref.Name, OwnerKind: ref.OwnerKind,
	}
}

func externalNode(ip string) TopologyNode {
	if ip == "" {
		return TopologyNode{}
	}
	id := "ext/" + ip
	return TopologyNode{ID: id, Label: ip, Kind: "External"}
}

func nodeFromCapturePod(capturePod string, tracked []TrackedPod) TopologyNode {
	ns, name := splitCapturePod(capturePod)
	if ns != "" && name != "" {
		for _, p := range tracked {
			if p.Namespace == ns && p.Name == name {
				return podNode(podRef{Namespace: ns, Name: name, OwnerKind: p.OwnerKind})
			}
		}
		return TopologyNode{
			ID: ns + "/" + name, Label: name, Kind: "Pod", Namespace: ns, Pod: name,
		}
	}
	return TopologyNode{}
}

func splitCapturePod(s string) (namespace, name string) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "/"); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", ""
}
