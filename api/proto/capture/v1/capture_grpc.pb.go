package capturev1

import (
	"context"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const CaptureService_StreamPackets_FullMethodName = "/capture.v1.CaptureService/StreamPackets"

type CaptureServiceClient interface {
	StreamPackets(ctx context.Context, opts ...grpc.CallOption) (CaptureService_StreamPacketsClient, error)
}

type captureServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewCaptureServiceClient(cc grpc.ClientConnInterface) CaptureServiceClient {
	return &captureServiceClient{cc}
}

func (c *captureServiceClient) StreamPackets(ctx context.Context, opts ...grpc.CallOption) (CaptureService_StreamPacketsClient, error) {
	stream, err := c.cc.NewStream(ctx, &CaptureService_ServiceDesc.Streams[0], CaptureService_StreamPackets_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	return &captureServiceStreamPacketsClient{ClientStream: stream}, nil
}

type CaptureService_StreamPacketsClient interface {
	Send(*TargetPodRequest) error
	Recv() (*CaptureChunk, error)
	grpc.ClientStream
}

type captureServiceStreamPacketsClient struct {
	grpc.ClientStream
}

func (x *captureServiceStreamPacketsClient) Send(m *TargetPodRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *captureServiceStreamPacketsClient) Recv() (*CaptureChunk, error) {
	m := new(CaptureChunk)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type CaptureServiceServer interface {
	StreamPackets(CaptureService_StreamPacketsServer) error
	mustEmbedUnimplementedCaptureServiceServer()
}

type UnimplementedCaptureServiceServer struct{}

func (UnimplementedCaptureServiceServer) StreamPackets(CaptureService_StreamPacketsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamPackets not implemented")
}
func (UnimplementedCaptureServiceServer) mustEmbedUnimplementedCaptureServiceServer() {}

type CaptureService_StreamPacketsServer interface {
	Send(*CaptureChunk) error
	Recv() (*TargetPodRequest, error)
	grpc.ServerStream
}

type captureServiceStreamPacketsServer struct {
	grpc.ServerStream
}

func (x *captureServiceStreamPacketsServer) Send(m *CaptureChunk) error {
	return x.ServerStream.SendMsg(m)
}

func (x *captureServiceStreamPacketsServer) Recv() (*TargetPodRequest, error) {
	m := new(TargetPodRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func RegisterCaptureServiceServer(s grpc.ServiceRegistrar, srv CaptureServiceServer) {
	s.RegisterService(&CaptureService_ServiceDesc, srv)
}

var CaptureService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "capture.v1.CaptureService",
	HandlerType: (*CaptureServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamPackets",
			Handler:       _CaptureService_StreamPackets_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "capture/v1/capture.proto",
}

func _CaptureService_StreamPackets_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(CaptureServiceServer).StreamPackets(&captureServiceStreamPacketsServer{ServerStream: stream})
}

// Embed for forward compatibility
type UnsafeCaptureServiceServer interface {
	mustEmbedUnimplementedCaptureServiceServer()
}
