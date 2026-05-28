package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cloud-flow/services/data-plane"
)

func main() {
	cfg := dataplane.DefaultConfig()
	flag.StringVar(&cfg.GrpcAddr, "grpc-addr", cfg.GrpcAddr, "gRPC listen address")
	flag.StringVar(&cfg.MetricsAddr, "metrics-addr", cfg.MetricsAddr, "Metrics HTTP listen address")
	flag.Parse()

	svc, err := dataplane.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
		os.Exit(1)
	}
	if err := svc.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	svc.Stop()
}
