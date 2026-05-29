package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cloud-flow/services/topology-engine"
)

func main() {
	cfg := topologyengine.DefaultConfig()
	flag.StringVar(&cfg.GrpcAddr, "grpc-addr", cfg.GrpcAddr, "gRPC listen address")
	flag.Parse()

	// 从环境变量读取配置
	if addr := os.Getenv("CLICKHOUSE_ADDR"); addr != "" {
		cfg.ClickHouseAddr = addr
	}
	if user := os.Getenv("CLICKHOUSE_USER"); user != "" {
		cfg.ClickHouseUser = user
	}
	if password := os.Getenv("CLICKHOUSE_PASSWORD"); password != "" {
		cfg.ClickHousePass = password
	}
	if db := os.Getenv("CLICKHOUSE_DATABASE"); db != "" {
		cfg.ClickHouseDB = db
	}

	svc, err := topologyengine.New(cfg)
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
