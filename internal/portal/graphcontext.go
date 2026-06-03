package portal

import (
	"fmt"
	"strings"

	"github.com/netobserv/spcg/internal/ai"
	"github.com/netobserv/spcg/internal/pcap"
)

func buildScrubbedGraphContext(scrub *ai.Scrubber, topo pcap.FlowTopology) string {
	if len(topo.Nodes) == 0 && len(topo.Edges) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Scrubbed flow graph (tenant capture topology synced to graph store):\n")
	for _, n := range topo.Nodes {
		line := scrub.Scrub(n.ID)
		if n.Pod != "" {
			line += " pod=" + scrub.Scrub(n.Pod)
		}
		if n.Namespace != "" {
			line += " ns=" + scrub.Scrub(n.Namespace)
		}
		if n.HostIP != "" {
			line += " ip=" + scrub.Scrub(n.HostIP)
		}
		b.WriteString("- node " + line + "\n")
	}
	for _, e := range topo.Edges {
		ports := ""
		if e.SrcPort > 0 || e.DstPort > 0 {
			ports = fmt.Sprintf(" ports=%d→%d", e.SrcPort, e.DstPort)
		}
		b.WriteString(fmt.Sprintf("- flow %s -> %s proto=%s health=%s packets=%d bytes=%d%s\n",
			scrub.Scrub(e.From), scrub.Scrub(e.To), e.Proto, e.Health, e.Packets, e.Bytes, ports))
		if e.DropCause != "" {
			b.WriteString("  drop: " + scrub.Scrub(e.DropCause) + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}
