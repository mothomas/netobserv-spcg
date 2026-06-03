package portal

import (
	"github.com/netobserv/spcg/internal/pcap"
)

func boundedTopologyFromSession(sess *pcap.Session) (pcap.FlowTopology, bool) {
	return pcap.BuildBoundedTopology(sess.Events(), sess.TrackedPods())
}
