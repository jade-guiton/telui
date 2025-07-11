package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"

	"github.com/jade-guiton/telui/static"
)

func writeGzipJson(w http.ResponseWriter, producer func(io.Writer)) {
	hd := w.Header()
	hd.Set("Content-Type", "application/json")
	hd.Set("Content-Encoding", "gzip")
	w2 := gzip.NewWriter(w)
	defer w2.Flush()
	w3 := bufio.NewWriter(w2)
	defer w3.Flush()
	producer(w3)
}

func parseSpanId(traceIdStr string, spanIdStr string) (traceId, spanId, bool) {
	traceIdBytes, err := hex.DecodeString(traceIdStr)
	if err != nil || len(traceIdBytes) != 16 {
		return traceId{}, spanId{}, false
	}
	tid := traceId(traceIdBytes)
	spanIdBytes, err := hex.DecodeString(spanIdStr)
	if err != nil || len(spanIdBytes) != 8 {
		return traceId{}, spanId{}, false
	}
	sid := spanId(spanIdBytes)
	return tid, sid, true
}

func parseHashId(hashIdStr string) (uint64, bool) {
	hashIdBytes, err := hex.DecodeString(hashIdStr)
	if err != nil || len(hashIdBytes) != 8 {
		return 0, false
	}
	hid := binary.BigEndian.Uint64(hashIdBytes)
	return hid, true
}

func serveUi(st *storage, port int) (stopFunc, error) {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.FileServerFS(static.StaticFs))

	mux.HandleFunc("GET /api/traces", func(w http.ResponseWriter, r *http.Request) {
		writeGzipJson(w, func(w io.Writer) {
			m := mapify(w)
			defer m.done()
			st.Lock()
			defer st.Unlock()
			for id, trace := range st.traces {
				m.pair(id.toString(), trace)
			}
		})
	})
	mux.HandleFunc("GET /api/span/{traceId}/{spanId}", func(w http.ResponseWriter, r *http.Request) {
		tid, sid, ok := parseSpanId(r.PathValue("traceId"), r.PathValue("spanId"))
		if !ok {
			writeError(w, http.StatusBadRequest)
			return
		}
		st.Lock()
		defer st.Unlock()
		trace, ok := st.traces[tid]
		var span span
		if ok {
			span, ok = trace.spans[sid]
		}
		if !ok {
			writeError(w, http.StatusNotFound)
		}
		writeGzipJson(w, func(w io.Writer) {
			m := mapify(w)
			m.pair("span", span)
			m.pair("scope", st.scopes[span.scope])
			m.pair("resource", st.resources[span.res])
			m.pair("request", st.requests[span.req])
			m.done()
		})
	})

	mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) {
		writeGzipJson(w, func(w io.Writer) {
			a := arrayify(w)
			st.Lock()
			defer st.Unlock()
			for _, log := range st.logs {
				a.item(log.logSummary)
			}
			a.done()
		})
	})
	mux.HandleFunc("GET /api/log/{logId}", func(w http.ResponseWriter, r *http.Request) {
		logIdStr := r.PathValue("logId")
		logId, err := strconv.Atoi(logIdStr)
		if err != nil || logId < 0 {
			writeError(w, http.StatusBadRequest)
			return
		}
		st.Lock()
		defer st.Unlock()
		if logId >= len(st.logs) {
			writeError(w, http.StatusNotFound)
			return
		}
		writeGzipJson(w, func(w io.Writer) {
			m := mapify(w)
			defer m.done()
			log := st.logs[logId]
			m.pair("log", log)
			m.pair("scope", st.scopes[log.scope])
			m.pair("resource", st.resources[log.res])
			m.pair("request", st.requests[log.req])
		})
	})

	mux.HandleFunc("GET /api/metrics", func(w http.ResponseWriter, r *http.Request) {
		writeGzipJson(w, func(w io.Writer) {
			m := mapify(w)
			st.Lock()
			defer st.Unlock()

			m2 := m.submap("metrics")
			for mid, metric := range st.metrics {
				m3 := m2.submap(hashToString(uint64(mid)))
				metric.metricIdentity.toJson(&m3)
				m3.pair("desc", stringValue(metric.desc))
				m3.done()
			}
			m2.done()

			m2 = m.submap("resources")
			for rid, res := range st.resources {
				m2.pair(hashToString(uint64(rid)), res)
			}
			m2.done()

			m2 = m.submap("scopes")
			for sid, res := range st.scopes {
				m2.pair(hashToString(uint64(sid)), res)
			}
			m2.done()

			m.done()
		})
	})
	mux.HandleFunc("GET /api/metric/{metricId}", func(w http.ResponseWriter, r *http.Request) {
		mid, ok := parseHashId(r.PathValue("metricId"))
		if !ok {
			writeError(w, http.StatusBadRequest)
			return
		}
		st.Lock()
		defer st.Unlock()
		metric, ok := st.metrics[hashId(mid)]
		if !ok {
			writeError(w, http.StatusNotFound)
			return
		}
		requests := map[reqId]struct{}{}
		for _, stream := range metric.streams {
			for _, pt := range stream.points {
				requests[pt.getPoint().req] = struct{}{}
			}
		}
		writeGzipJson(w, func(w io.Writer) {
			m := mapify(w)
			m.pair("metric", metric)
			m2 := m.submap("requests")
			for reqId, _ := range requests {
				m2.pair(hashToString(uint64(reqId)), st.requests[reqId])
			}
			m2.done()
			m.done()
		})
	})

	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		st.reset()
	})

	server := http.Server{Handler: mux}
	err := serveLocalhost(&server, "UI", port)
	if err != nil {
		return nil, err
	}
	return func() {
		server.Shutdown(context.Background())
	}, nil
}
