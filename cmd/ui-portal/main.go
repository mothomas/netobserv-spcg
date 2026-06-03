package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/netobserv/spcg/internal/auth"
	"github.com/netobserv/spcg/internal/capture/admission"
	graphdb "github.com/netobserv/spcg/internal/graph/neo4j"
	"github.com/netobserv/spcg/internal/portal"
	"github.com/netobserv/spcg/internal/tlsutil"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	listen := envOr("UI_LISTEN", ":8080")
	engineAddr := envOr("ENGINE_GRPC_ADDR", "spcg-backend-engine.pcap-capture.svc.cluster.local:8443")

	var engineCreds credentials.TransportCredentials
	mtls := tlsutil.FromEnv()
	if mtls.CertFile != "" && mtls.KeyFile != "" && mtls.CAFile != "" {
		tlsCfg, err := mtls.ClientTLS()
		if err != nil {
			log.Fatalf("failed configuring mTLS client to engine: %v", err)
		}
		engineCreds = credentials.NewTLS(tlsCfg)
	} else {
		engineCreds = insecure.NewCredentials()
	}

	ctx := context.Background()
	graphStore, err := graphdb.Open(ctx)
	if err != nil {
		log.Fatalf("neo4j: %v", err)
	}
	if graphStore.Enabled() {
		log.Printf("neo4j graph store connected")
	} else {
		graphdb.LogDisabled()
	}

	srv := &portal.Server{
		EngineAddr:    engineAddr,
		EngineTLS:     engineCreds,
		Sessions:      auth.NewStore(0),
		Graph:         graphStore,
		CaptureLimits: admission.LoadFromEnv(),
	}
	portal.SetCaptureGraph(graphStore)

	handler := cors(srv.Routes())
	httpServer := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0,
	}

	go func() {
		log.Printf("ui-portal listening on %s, engine=%s", listen, engineAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http serve failed: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = httpServer.Close()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", envOr("CORS_ORIGIN", "*"))
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Forwarded-User-Token, X-SPCG-Session")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
