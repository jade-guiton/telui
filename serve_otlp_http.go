package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(http.StatusText(status)))
}

type requestObject interface {
	UnmarshalProto([]byte) error
	UnmarshalJSON([]byte) error
}
type responseObject interface {
	MarshalProto() ([]byte, error)
	MarshalJSON() ([]byte, error)
}
type responder func() error

func readOtlpRequest(w http.ResponseWriter, r *http.Request, req requestObject, res responseObject) (responder, error) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("HTTP method not allowed")
	}

	var body []byte
	var err error
	switch r.Header.Get("Content-Encoding") {
	case "":
		body, err = io.ReadAll(r.Body)
	case "gzip":
		var reader *gzip.Reader
		reader, err = gzip.NewReader(r.Body)
		if err == nil {
			body, err = io.ReadAll(reader)
		}
	default:
		writeError(w, http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported encoding")
	}
	if err == nil {
		err = r.Body.Close()
	} else {
		_ = r.Body.Close()
	}
	if err != nil {
		writeError(w, http.StatusBadRequest)
		return nil, err
	}

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/x-protobuf":
		err = req.UnmarshalProto(body)
	case "application/json":
		err = req.UnmarshalJSON(body)
	default:
		writeError(w, http.StatusUnsupportedMediaType)
		return nil, fmt.Errorf("unsupported content type")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest)
		return nil, err
	}

	return func() error {
		var resBody []byte
		var err error
		switch contentType {
		case "application/x-protobuf":
			resBody, err = res.MarshalProto()
		case "application/json":
			resBody, err = res.MarshalJSON()
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError)
			return err
		}
		w.Header().Set("Content-Type", contentType)
		_, err = w.Write(resBody)
		return err
	}, nil
}

func serveOtlpHttp(storage *storage, endpoint string) (stopFunc, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		req := ptraceotlp.NewExportRequest()
		res := ptraceotlp.NewExportResponse()
		ack, err := readOtlpRequest(w, r, &req, &res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid trace request from %s: %v", r.RemoteAddr, err)
			return
		}
		storage.receiveTraces(req.Traces())
		if err = ack(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to respond to %s: %v", r.RemoteAddr, err)
		}
	})

	mux.HandleFunc("/v1/logs", func(w http.ResponseWriter, r *http.Request) {
		req := plogotlp.NewExportRequest()
		res := plogotlp.NewExportResponse()
		ack, err := readOtlpRequest(w, r, &req, &res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid log request from %s: %v", r.RemoteAddr, err)
			return
		}
		storage.receiveLogs(req.Logs())
		if err = ack(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to respond to %s: %v", r.RemoteAddr, err)
		}
	})
	mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		req := pmetricotlp.NewExportRequest()
		res := pmetricotlp.NewExportResponse()
		ack, err := readOtlpRequest(w, r, &req, &res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid metric request from %s: %v", r.RemoteAddr, err)
			return
		}
		storage.receiveMetrics(req.Metrics())
		if err = ack(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to respond to %s: %v", r.RemoteAddr, err)
		}
	})

	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		return nil, err
	}
	server := http.Server{Handler: mux}
	fmt.Printf("Started OTLP/HTTP endpoint at http://%s\n", endpoint)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Fatal error in OTLP/HTTP server: %v\n", err)
		}
	}()
	return func() {
		server.Shutdown(context.Background())
	}, nil
}
