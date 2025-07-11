package main

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"

	_ "google.golang.org/grpc/encoding/gzip"
)

type traceServer struct {
	ptraceotlp.UnimplementedGRPCServer
	st *storage
}

func (ts *traceServer) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	ts.st.receiveTraces(req.Traces(), grpcRequest(ctx))
	return ptraceotlp.NewExportResponse(), nil
}

type logServer struct {
	plogotlp.UnimplementedGRPCServer
	st *storage
}

func (ls *logServer) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	ls.st.receiveLogs(req.Logs(), grpcRequest(ctx))
	return plogotlp.NewExportResponse(), nil
}

type metricServer struct {
	pmetricotlp.UnimplementedGRPCServer
	st *storage
}

func (ms *metricServer) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	ms.st.receiveMetrics(req.Metrics(), grpcRequest(ctx))
	return pmetricotlp.NewExportResponse(), nil
}

func serveOtlpGrpc(storage *storage, port int) (stopFunc, error) {
	grpcServer := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(grpcServer, &traceServer{st: storage})
	plogotlp.RegisterGRPCServer(grpcServer, &logServer{st: storage})
	pmetricotlp.RegisterGRPCServer(grpcServer, &metricServer{st: storage})

	err := serveLocalhost(grpcServer, "OTLP/gRPC", port)
	if err != nil {
		return nil, err
	}
	return func() {
		grpcServer.GracefulStop()
	}, nil
}
