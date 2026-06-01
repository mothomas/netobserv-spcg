package sensor

import (
	"encoding/binary"
	"fmt"
	"net"

	spcgk8s "github.com/netobserv/spcg/internal/k8s"
)

// PacketMatchesPods returns true when the flow involves a selected pod (K8s enrich and/or pod IP).
func PacketMatchesPods(meta FlowMetadata, pods []spcgk8s.PodDetail) bool {
	_, _, ok := CapturePodFromMeta(meta, pods)
	return ok
}

// CapturePodFromMeta picks the tracked pod on src/dst K8s fields or L3 addresses.
func CapturePodFromMeta(meta FlowMetadata, pods []spcgk8s.PodDetail) (namespace, name string, ok bool) {
	if len(pods) == 0 {
		return "", "", false
	}
	if meta != nil {
		srcNS, _ := meta["SrcK8S_Namespace"].(string)
		srcName, _ := meta["SrcK8S_Name"].(string)
		dstNS, _ := meta["DstK8S_Namespace"].(string)
		dstName, _ := meta["DstK8S_Name"].(string)
		for _, p := range pods {
			if p.Namespace == "" || p.Name == "" {
				continue
			}
			if srcNS == p.Namespace && srcName == p.Name {
				return p.Namespace, p.Name, true
			}
			if dstNS == p.Namespace && dstName == p.Name {
				return p.Namespace, p.Name, true
			}
		}
		srcIP := metaIPString(meta, "SrcAddr")
		dstIP := metaIPString(meta, "DstAddr")
		for _, p := range pods {
			ips := p.PodIPs
			if len(ips) == 0 && p.PodIP != "" {
				ips = []string{p.PodIP}
			}
			for _, ip := range ips {
				if ip == "" {
					continue
				}
				if ip == srcIP || ip == dstIP {
					return p.Namespace, p.Name, true
				}
			}
		}
	}
	return "", "", false
}

func metaIPString(meta FlowMetadata, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		if len(t) == 4 {
			return net.IP(t).String()
		}
		if len(t) == 16 {
			return net.IP(t).String()
		}
	case float64:
		// JSON numbers for uint32 addresses
		return uint32ToIP(uint32(t))
	}
	return fmt.Sprint(v)
}

func uint32ToIP(n uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], n)
	return net.IP(b[:]).String()
}
