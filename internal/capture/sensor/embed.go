package sensor

import _ "embed"

// NetObserv packet-capture DaemonSet template (from netobserv-cli res/packet-capture.yml).
//
//go:embed manifests/packet-capture-daemonset.yaml
var packetCaptureDaemonSetTemplate string

//go:embed manifests/collector-pipeline-config.json
var collectorPipelineConfigJSON string

//go:embed manifests/flow-filter.json
var flowFilterTemplateJSON string
