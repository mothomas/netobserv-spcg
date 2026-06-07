package trace

import (
	"context"
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	gvrBGPAdvertisement = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "bgpadvertisements"}
	gvrL2Advertisement  = schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "l2advertisements"}
)

type metalLBIndex struct {
	pools map[string]unstructured.Unstructured
	bgpAd []unstructured.Unstructured
	l2Ad  []unstructured.Unstructured
	peers map[string]unstructured.Unstructured
}

func (c *Catalog) loadMetalLBIndex(ctx context.Context) (*metalLBIndex, error) {
	if c.DC == nil {
		return nil, nil
	}
	idx := &metalLBIndex{
		pools: map[string]unstructured.Unstructured{},
		peers: map[string]unstructured.Unstructured{},
	}
	for _, ns := range []string{"metallb-system", "openshift-metallb", "metallb"} {
		if pools, err := c.DC.list(ctx, gvrIPPool, ns); err == nil {
			for _, p := range pools {
				idx.pools[p.GetName()] = p
			}
		}
		if peers, err := c.DC.list(ctx, gvrBGPPeer, ns); err == nil {
			for _, p := range peers {
				idx.peers[p.GetName()] = p
			}
		}
		if ads, err := c.DC.list(ctx, gvrBGPAdvertisement, ns); err == nil {
			idx.bgpAd = append(idx.bgpAd, ads...)
		}
		if ads, err := c.DC.list(ctx, gvrL2Advertisement, ns); err == nil {
			idx.l2Ad = append(idx.l2Ad, ads...)
		}
	}
	return idx, nil
}

func (c *Catalog) discoverMetalLB(ctx context.Context, services map[string]corev1.Service, b *graphBuilder) error {
	idx, err := c.loadMetalLBIndex(ctx)
	if err != nil || idx == nil {
		return nil
	}
	for _, svc := range services {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		vip := loadBalancerVIP(&svc)
		if vip == "" {
			continue
		}
		svcID := nodeID("service", svc.Namespace, svc.Name)
		poolName, poolCIDR := resolvePoolForVIP(idx.pools, &svc, vip)
		if poolName == "" {
			c.wireMetalLBVIPOnly(b, svc, vip, svcID)
			continue
		}
		poolID := nodeID("metallb-pool", "metallb-system", poolName)
		b.addNode(poolID, poolName, "metallb-pool", "metallb-system", false, false, rankForKind("metallb-pool"), poolCIDR)
		b.addPath("ingress", poolName, svc.Namespace, "metallb-pool", "discovered", vip)

		lbID := nodeID("loadbalancer", svc.Namespace, svc.Name+"-vip")
		b.addNode(lbID, vip, "loadbalancer-external", svc.Namespace, false, false, rankForKind("loadbalancer-external"), "MetalLB VIP · "+poolName)
		b.addEdge(lbID, poolID, "direct", true, poolName)
		b.addEdge(poolID, svcID, "direct", true, "")

		extID := nodeID("external", "", "client-lb")
		if !b.hasNode(extID) {
			b.addNode(extID, "Client", "external-client", "", false, false, rankForKind("external-client"), "via LoadBalancer")
		}
		b.addEdge(extID, lbID, "ingress", true, "")

		for _, ad := range advertisementsForPool(idx, poolName) {
			adKind := ad.GetKind()
			adID := nodeID("metallb-ad", ad.GetNamespace(), ad.GetName())
			proto := strings.TrimPrefix(adKind, "BGP")
			if proto == adKind {
				proto = "L2"
			}
			b.addNode(adID, ad.GetName(), "metallb-advertisement", ad.GetNamespace(), false, false, rankForKind("metallb-advertisement"), proto+" advertisement")
			b.addEdge(poolID, adID, "direct", true, proto)

			for _, peer := range peersForAdvertisement(ad, idx.peers) {
				peerID := nodeID("bgp-peer", peer.GetNamespace(), peer.GetName())
				peerAddr, _, _ := unstructured.NestedString(peer.Object, "spec", "peerAddress")
				srcAddr, _, _ := unstructured.NestedString(peer.Object, "spec", "sourceAddress")
				detail := peerAddr
				if srcAddr != "" {
					detail = srcAddr + " → " + peerAddr
				}
				b.addNode(peerID, peer.GetName(), "bgp-peer", peer.GetNamespace(), false, false, rankForKind("bgp-peer"), detail)
				b.addPath("ingress", peer.GetName(), peer.GetNamespace(), "bgp-peer", "discovered", detail)
				b.addEdge(adID, peerID, "direct", true, "BGP")
			}
		}
	}
	return nil
}

func (c *Catalog) wireMetalLBVIPOnly(b *graphBuilder, svc corev1.Service, vip, svcID string) {
	lbID := nodeID("loadbalancer", svc.Namespace, svc.Name+"-vip")
	b.addNode(lbID, vip, "loadbalancer-external", svc.Namespace, false, false, rankForKind("loadbalancer-external"), "LoadBalancer VIP")
	b.addEdge(lbID, svcID, "direct", true, "")
	extID := nodeID("external", "", "client-lb")
	if !b.hasNode(extID) {
		b.addNode(extID, "Client", "external-client", "", false, false, rankForKind("external-client"), "via LB")
	}
	b.addEdge(extID, lbID, "ingress", true, "")
}

func loadBalancerVIP(svc *corev1.Service) string {
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			return ing.IP
		}
	}
	return ""
}

func resolvePoolForVIP(pools map[string]unstructured.Unstructured, svc *corev1.Service, vip string) (name, cidr string) {
	if ann := strings.TrimSpace(svc.Annotations["metallb.universe.tf/address-pool"]); ann != "" {
		if p, ok := pools[ann]; ok {
			return ann, poolAddrs(p)
		}
		return ann, ""
	}
	if ann := strings.TrimSpace(svc.Annotations["metallb.io/address-pool"]); ann != "" {
		if p, ok := pools[ann]; ok {
			return ann, poolAddrs(p)
		}
		return ann, ""
	}
	for n, p := range pools {
		if vipInPool(vip, p) {
			return n, poolAddrs(p)
		}
	}
	return "", ""
}

func poolAddrs(p unstructured.Unstructured) string {
	lines, _, _ := unstructured.NestedStringSlice(p.Object, "spec", "addresses")
	if len(lines) > 0 {
		return strings.Join(lines, ", ")
	}
	addrs, _, _ := unstructured.NestedStringSlice(p.Object, "spec", "addressRanges")
	return strings.Join(addrs, ", ")
}

func vipInPool(vip string, pool unstructured.Unstructured) bool {
	ip := net.ParseIP(vip)
	if ip == nil {
		return false
	}
	lines, _, _ := unstructured.NestedStringSlice(pool.Object, "spec", "addresses")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "/") {
			_, cidr, err := net.ParseCIDR(line)
			if err == nil && cidr.Contains(ip) {
				return true
			}
		}
		if strings.Contains(line, "-") {
			parts := strings.SplitN(line, "-", 2)
			if len(parts) == 2 && ipInRange(ip, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])) {
				return true
			}
		}
	}
	return false
}

func ipInRange(ip net.IP, start, end string) bool {
	s := net.ParseIP(start)
	e := net.ParseIP(end)
	if s == nil || e == nil {
		return false
	}
	s16 := ip.To16()
	e16 := e.To16()
	t16 := ip.To16()
	if s16 == nil || e16 == nil || t16 == nil {
		return false
	}
	for i := range s16 {
		if t16[i] < s16[i] || t16[i] > e16[i] {
			return false
		}
	}
	return true
}

func advertisementsForPool(idx *metalLBIndex, poolName string) []unstructured.Unstructured {
	var out []unstructured.Unstructured
	for _, ad := range idx.bgpAd {
		if adMatchesPool(ad, poolName) {
			out = append(out, ad)
		}
	}
	for _, ad := range idx.l2Ad {
		if adMatchesPool(ad, poolName) {
			out = append(out, ad)
		}
	}
	return out
}

func adMatchesPool(ad unstructured.Unstructured, poolName string) bool {
	pools, _, _ := unstructured.NestedStringSlice(ad.Object, "spec", "ipAddressPools")
	for _, p := range pools {
		if p == poolName {
			return true
		}
	}
	return len(pools) == 0
}

func peersForAdvertisement(ad unstructured.Unstructured, peers map[string]unstructured.Unstructured) []unstructured.Unstructured {
	names, _, _ := unstructured.NestedStringSlice(ad.Object, "spec", "peers")
	if len(names) > 0 {
		var out []unstructured.Unstructured
		for _, n := range names {
			if p, ok := peers[n]; ok {
				out = append(out, p)
			}
		}
		return out
	}
	out := make([]unstructured.Unstructured, 0, len(peers))
	for _, p := range peers {
		out = append(out, p)
	}
	return out
}
