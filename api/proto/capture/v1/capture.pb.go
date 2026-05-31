package capturev1

// Hand-maintained message types matching capture.proto (run `make proto` to regenerate with protoc).

type TargetPodRequest struct {
	SessionId     string `json:"session_id,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	PodName       string `json:"pod_name,omitempty"`
	PodUid        string `json:"pod_uid,omitempty"`
	WorkloadKind  string `json:"workload_kind,omitempty"`
	WorkloadName  string `json:"workload_name,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
	Port          int32  `json:"port,omitempty"`
}

func (x *TargetPodRequest) Reset()         { *x = TargetPodRequest{} }
func (x *TargetPodRequest) String() string { return x.PodName }
func (*TargetPodRequest) ProtoMessage()    {}

func (x *TargetPodRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}
func (x *TargetPodRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}
func (x *TargetPodRequest) GetPodName() string {
	if x != nil {
		return x.PodName
	}
	return ""
}
func (x *TargetPodRequest) GetPodUid() string {
	if x != nil {
		return x.PodUid
	}
	return ""
}
func (x *TargetPodRequest) GetWorkloadKind() string {
	if x != nil {
		return x.WorkloadKind
	}
	return ""
}
func (x *TargetPodRequest) GetWorkloadName() string {
	if x != nil {
		return x.WorkloadName
	}
	return ""
}
func (x *TargetPodRequest) GetLabelSelector() string {
	if x != nil {
		return x.LabelSelector
	}
	return ""
}
func (x *TargetPodRequest) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

type CaptureChunk struct {
	SessionId       string `json:"session_id,omitempty"`
	PodName         string `json:"pod_name,omitempty"`
	PodUid          string `json:"pod_uid,omitempty"`
	Data            []byte `json:"data,omitempty"`
	Sequence        uint64 `json:"sequence,omitempty"`
	StitchedRestart bool   `json:"stitched_restart,omitempty"`
	PacketsPerSec   uint64 `json:"packets_per_sec,omitempty"`
	CumulativeBytes uint64 `json:"cumulative_bytes,omitempty"`
}

func (x *CaptureChunk) Reset()         { *x = CaptureChunk{} }
func (x *CaptureChunk) String() string { return x.PodName }
func (*CaptureChunk) ProtoMessage()    {}

func (x *CaptureChunk) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}
func (x *CaptureChunk) GetPodName() string {
	if x != nil {
		return x.PodName
	}
	return ""
}
func (x *CaptureChunk) GetPodUid() string {
	if x != nil {
		return x.PodUid
	}
	return ""
}
func (x *CaptureChunk) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}
func (x *CaptureChunk) GetSequence() uint64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}
func (x *CaptureChunk) GetStitchedRestart() bool {
	if x != nil {
		return x.StitchedRestart
	}
	return false
}
func (x *CaptureChunk) GetPacketsPerSec() uint64 {
	if x != nil {
		return x.PacketsPerSec
	}
	return 0
}
func (x *CaptureChunk) GetCumulativeBytes() uint64 {
	if x != nil {
		return x.CumulativeBytes
	}
	return 0
}
