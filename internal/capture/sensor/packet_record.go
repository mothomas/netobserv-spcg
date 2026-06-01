package sensor

// FlowMetadata carries L3/L4 hints for pod scoping (from frame parse or K8s enrich).
type FlowMetadata map[string]interface{}

// PacketRecord is one PCA packet plus optional netobserv flow context.
type PacketRecord struct {
	Data []byte
	Meta FlowMetadata
}
