package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
)

type stopFunc func()

func (s stopFunc) stop() {
	if s != nil {
		s()
	}
}

func start() error {
	grpcPort := flag.Int("grpc", 4317, "Port for OTLP/gRPC server (0 to disable)")
	httpPort := flag.Int("http", 4318, "Port for OTLP/HTTP server (0 to disable)")
	uiPort := flag.Int("ui", 8080, "Port for web interface")
	verbose := flag.Bool("verbose", false, "Log incoming data")

	flag.Parse()

	storage := newStorage(*verbose)

	if *grpcPort != 0 {
		otlpGrpc, err := serveOtlpGrpc(storage, fmt.Sprintf("localhost:%d", *grpcPort))
		if err != nil {
			return err
		}
		defer otlpGrpc.stop()
	}

	if *httpPort != 0 {
		otlpHttp, err := serveOtlpHttp(storage, fmt.Sprintf("localhost:%d", *httpPort))
		if err != nil {
			return err
		}
		defer otlpHttp.stop()
	}

	api, err := serveUi(storage, fmt.Sprintf("localhost:%d", *uiPort))
	if err != nil {
		return err
	}
	defer api.stop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Printf("\nStopping.\n")

	return nil
}

func main() {
	err := start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}
