package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	capturev1 "github.com/netobserv/spcg/api/proto/capture/v1"
	"github.com/netobserv/spcg/internal/capture"
	spcgk8s "github.com/netobserv/spcg/internal/k8s"
	"github.com/netobserv/spcg/internal/tlsutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/client-go/kubernetes"
)

func main() {
	addr := envOr("ENGINE_GRPC_ADDR", ":8443")

	var client kubernetes.Interface
	client, err := spcgk8s.PrivilegedInCluster()
	if err != nil {
		log.Printf("in-cluster unavailable (%v), falling back to kubeconfig", err)
		client, err = spcgk8s.PrivilegedFromKubeconfig()
		if err != nil {
			log.Fatalf("failed initializing privileged kubernetes client: %v", err)
		}
	}

	engine := capture.NewEngineServer(client)

	var opts []grpc.ServerOption
	mtls := tlsutil.FromEnv()
	if mtls.CertFile != "" && mtls.KeyFile != "" {
		tlsCfg, err := mtls.ServerTLS()
		if err != nil {
			log.Fatalf("failed configuring mTLS server: %v", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	srv := grpc.NewServer(opts...)
	capturev1.RegisterCaptureServiceServer(srv, engine)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed listening on %s: %v", err)
	}

	go func() {
		log.Printf("backend-engine listening on %s", addr)
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("grpc serve failed: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	srv.GracefulStop()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
