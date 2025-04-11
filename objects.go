package main

import (
	"cmp"
	"context"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"hash/fnv"
	"io"
	"net/http"
	"slices"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func hashToBytes(x uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, x)
}
func hashToString(x uint64) string {
	return hex.EncodeToString(hashToBytes(x))
}
func hashHash(x uint64, h hash.Hash64) {
	forceWrite(h, hashToBytes(x))
}

type hashId uint64
type traceId [16]byte

type reqId uint64
type resId uint64
type scopeId uint64
type spanId [8]byte
type traceSpanId struct {
	traceId
	spanId
}

func (reqId reqId) hashInto(h hash.Hash64) {
	hashHash(uint64(reqId), h)
}
func (rid resId) hashInto(h hash.Hash64) {
	hashHash(uint64(rid), h)
}
func (sid scopeId) hashInto(h hash.Hash64) {
	hashHash(uint64(sid), h)
}
func (v spanId) hashInto(h hash.Hash64) {
	forceWrite(h, v[:])
}
func (v traceSpanId) hashInto(h hash.Hash64) {
	forceWrite(h, v.traceId[:])
	forceWrite(h, v.spanId[:])
}

func (v reqId) toJson(w io.Writer) {
	forcePrintf(w, `{"_req":"%x"}`, hashToBytes(uint64(v)))
}
func (v resId) toJson(w io.Writer) {
	forcePrintf(w, `{"_res":"%x"}`, hashToBytes(uint64(v)))
}
func (v scopeId) toJson(w io.Writer) {
	forcePrintf(w, `{"_scope":"%x"}`, hashToBytes(uint64(v)))
}
func (v spanId) toJson(w io.Writer) {
	forcePrintf(w, `{"_span":"%x"}`, v[:])
}
func (v traceSpanId) toJson(w io.Writer) {
	forcePrintf(w, `{"_span":"%x","_trace":"%x"}`, v.spanId[:], v.traceId[:])
}

func (tid traceId) notEmpty() bool {
	return tid != traceId{}
}
func (sid spanId) notEmpty() bool {
	return sid != spanId{}
}

func (tid traceId) toString() string {
	return hex.EncodeToString(tid[:])
}
func (sid spanId) toString() string {
	return hex.EncodeToString(sid[:])
}

type requestMeta struct {
	transport string
	peer      string
	headers   map[string][]string
}

var _ hashableValue = requestMeta{}

func grpcRequest(ctx context.Context) requestMeta {
	var req requestMeta
	req.transport = "grpc"
	if p, ok := peer.FromContext(ctx); ok {
		req.peer = p.Addr.String()
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		req.headers = md.Copy()
		delete(req.headers, "Content-Length")
	}
	return req
}

func httpRequest(r *http.Request) requestMeta {
	var req requestMeta
	req.transport = "http"
	req.peer = r.RemoteAddr
	req.headers = r.Header.Clone()
	delete(req.headers, "Content-Length")
	return req
}

type kvs = struct {
	k  string
	vs []string
}

func (r requestMeta) sortedHeaders() []kvs {
	headers := []kvs{}
	for k, vs := range r.headers {
		headers = append(headers, kvs{k: k, vs: vs})
	}
	slices.SortFunc(headers, func(a kvs, b kvs) int {
		return cmp.Compare(a.k, b.k)
	})
	return headers
}

func (r requestMeta) hashInto(h hash.Hash64) {
	forceWriteString(h, r.transport)
	forceWriteString(h, r.peer)
	for _, kvs := range r.sortedHeaders() {
		forceWriteString(h, kvs.k)
		for _, v := range kvs.vs {
			forceWriteString(h, v)
		}
	}
}
func (r requestMeta) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("transport", stringValue(r.transport))
	if r.peer != "" {
		m.pair("peer", stringValue(r.peer))
	}
	if r.headers != nil {
		m2 := m.submap("headers")
		for _, kvs := range r.sortedHeaders() {
			if len(kvs.vs) == 1 {
				m2.pair(kvs.k, stringValue(kvs.vs[0]))
			} else {
				a := m2.array(kvs.k)
				for _, v := range kvs.vs {
					a.item(stringValue(v))
				}
				a.done()
			}
		}
		m2.done()
	}
}

type resource struct {
	attr        mapValue
	attrDropped uint32
	schema      string
}

var _ hashableValue = resource{}

func (r resource) hashInto(h hash.Hash64) {
	r.attr.hashInto(h)
	forceBinaryWrite(h, r.attrDropped)
	forceWriteString(h, r.schema)
}
func (r resource) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	if r.attr.notEmpty() {
		m.pair("attr", r.attr)
	}
	if r.attrDropped != 0 {
		m.pair("attr.dropped", intValue(r.attrDropped))
	}
	if r.schema != "" {
		m.pair("schema", stringValue(r.schema))
	}
}

type scope struct {
	name        string
	version     string
	attr        mapValue
	attrDropped uint32
	schema      string
}

var _ hashableValue = scope{}

func (s scope) hashInto(h hash.Hash64) {
	forceWriteString(h, s.name)
	forceWriteString(h, s.version)
	s.attr.hashInto(h)
	forceBinaryWrite(h, s.attrDropped)
	forceWriteString(h, s.schema)
}
func (s scope) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	if s.name != "" {
		m.pair("name", stringValue(s.name))
	}
	if s.version != "" {
		m.pair("version", stringValue(s.version))
	}
	if s.attr.notEmpty() {
		m.pair("attr", s.attr)
	}
	if s.attrDropped != 0 {
		m.pair("attr.dropped", intValue(s.attrDropped))
	}
	if s.schema != "" {
		m.pair("schema", stringValue(s.schema))
	}
}

type trace struct {
	spans map[spanId]span
}

var _ value = trace{}

func (t trace) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	for id, span := range t.spans {
		m2 := m.submap(id.toString())
		span.spanSummary.toJson(&m2)
		m2.done()
	}
}

type spanSummary struct {
	parent spanId
	name   string
	status string
	start  timestampValue
	end    timestampValue
}

func (ss spanSummary) toJson(m *mapifier) {
	if ss.parent.notEmpty() {
		m.pair("parent", ss.parent)
	}
	m.pair("name", stringValue(ss.name))
	if ss.status != "" {
		m.pair("status", stringValue(ss.status))
	}
	m.pair("start", ss.start)
	m.pair("end", ss.end)
}

type span struct {
	req   reqId
	res   resId
	scope scopeId
	spanSummary
	statusMsg     string
	kind          string
	attr          mapValue
	attrDropped   uint32
	state         string
	flags         flagsValue
	events        []event
	eventsDropped uint32
	links         []link
	linksDropped  uint32
}

var _ value = span{}

func (s span) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("req", s.req)
	m.pair("res", s.res)
	m.pair("scope", s.scope)
	s.spanSummary.toJson(&m)
	m.pair("kind", stringValue(s.kind))
	if s.statusMsg != "" {
		m.pair("status.msg", stringValue(s.statusMsg))
	}
	if s.attr.notEmpty() {
		m.pair("attr", s.attr)
	}
	if s.attrDropped != 0 {
		m.pair("attr.dropped", intValue(s.attrDropped))
	}
	if s.state != "" {
		m.pair("state", stringValue(s.state))
	}
	if s.flags != 0x100 { // 0x100 means "no W3C trace flag, parent is not remote"
		m.pair("flags", s.flags)
	}
	if len(s.events) > 0 {
		a := m.array("events")
		for _, e := range s.events {
			a.item(e)
		}
		a.done()
	}
	if s.eventsDropped != 0 {
		m.pair("events.dropped", intValue(s.eventsDropped))
	}
	if len(s.links) > 0 {
		a := m.array("links")
		for _, l := range s.links {
			a.item(l)
		}
		a.done()
	}
	if s.linksDropped != 0 {
		m.pair("links.dropped", intValue(s.linksDropped))
	}
}

type event struct {
	name        string
	time        timestampValue
	attr        mapValue
	attrDropped uint32
}

func (e event) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("name", stringValue(e.name))
	if e.time.notEmpty() {
		m.pair("time", e.time)
	}
	if e.attr.notEmpty() {
		m.pair("attr", e.attr)
	}
	if e.attrDropped != 0 {
		m.pair("attr.dropped", intValue(e.attrDropped))
	}
}

type link struct {
	trace       traceId
	span        spanId
	attr        mapValue
	attrDropped uint32
	state       string
}

func (l link) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("span", traceSpanId{
		traceId: l.trace,
		spanId:  l.span,
	})
	if l.attr.notEmpty() {
		m.pair("attr", l.attr)
	}
	if l.attrDropped != 0 {
		m.pair("attr.dropped", intValue(l.attrDropped))
	}
	if l.state != "" {
		m.pair("state", stringValue(l.state))
	}
}

type logSummary struct {
	sev        string
	simpleTime timestampValue
	simpleBody string
}

var _ value = log{}

func (ls logSummary) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("time", ls.simpleTime)
	m.pair("sev", stringValue(ls.sev))
	m.pair("body", stringValue(ls.simpleBody))
}

type log struct {
	logSummary
	req         reqId
	res         resId
	scope       scopeId
	time        timestampValue
	timeObs     timestampValue
	sevText     string
	event       string
	body        value
	attr        mapValue
	attrDropped uint32
	flags       flagsValue
	trace       traceId
	span        spanId
}

var _ value = log{}

func (l log) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	m.pair("req", l.req)
	m.pair("res", l.res)
	m.pair("scope", l.scope)
	if l.time.notEmpty() {
		m.pair("time", l.time)
	}
	if l.timeObs.notEmpty() {
		m.pair("time.obs", l.timeObs)
	}
	if l.sev != "" {
		m.pair("sev", stringValue(l.sev))
	}
	if l.sevText != "" {
		m.pair("sev.text", stringValue(l.sevText))
	}
	if l.event != "" {
		m.pair("event", stringValue(l.event))
	}
	if l.body != nil {
		m.pair("body", l.body)
	}
	if l.attr.notEmpty() {
		m.pair("attr", l.attr)
	}
	if l.attrDropped != 0 {
		m.pair("attr.dropped", intValue(l.attrDropped))
	}
	if l.flags != 0 {
		m.pair("flags", l.flags)
	}
	if l.trace.notEmpty() {
		if l.span.notEmpty() {
			m.pair("span", traceSpanId{
				traceId: l.trace,
				spanId:  l.span,
			})
		} else {
			m.pair("trace", stringValue(l.trace.toString()))
		}
	}
}

type metricIdentity struct {
	res   resId
	scope scopeId
	name  string
	type_ string
	unit  string
	tempo string
	mono  bool
}

func getMetricId(mi metricIdentity) hashId {
	h := fnv.New64a()
	mi.res.hashInto(h)
	mi.scope.hashInto(h)
	forceWriteString(h, mi.name)
	forceWriteString(h, mi.type_)
	forceWriteString(h, mi.unit)
	forceWriteString(h, mi.tempo)
	forceBinaryWrite(h, mi.mono)
	return hashId(h.Sum64())
}

func (mi metricIdentity) toJson(m *mapifier) {
	m.pair("res", mi.res)
	m.pair("scope", mi.scope)
	m.pair("name", stringValue(mi.name))
	m.pair("type", stringValue(mi.type_))
	if mi.unit != "" {
		m.pair("unit", stringValue(mi.unit))
	}
	if mi.tempo != "" {
		m.pair("tempo", stringValue(mi.tempo))
	}
	if mi.type_ == "Sum" {
		m.pair("mono", boolValue(mi.mono))
	}
}

type metric struct {
	metricIdentity
	desc     string
	meta     mapValue
	conflict bool
	streams  map[hashId]*metricStream
}

var _ value = metric{}

func (me metric) toJson(w io.Writer) {
	ma := mapify(w)
	defer ma.done()
	me.metricIdentity.toJson(&ma)
	if me.desc != "" {
		ma.pair("desc", stringValue(me.desc))
	}
	if me.meta.notEmpty() {
		ma.pair("meta", me.meta)
	}
	ma2 := ma.submap("streams")
	defer ma2.done()
	for stid, st := range me.streams {
		ma2.pair(hashToString(uint64(stid)), st)
	}
}

type metricStream struct {
	attr   mapValue
	points []pointlike
}

func (ms metricStream) toJson(w io.Writer) {
	ma := mapify(w)
	defer ma.done()
	ma.pair("attr", ms.attr)
	if len(ms.points) != 0 {
		a := ma.array("pts")
		for _, pt := range ms.points {
			a.item(pt)
		}
		a.done()
	}
}

type point struct {
	time      timestampValue
	timeStart timestampValue
	flags     flagsValue
	req       reqId
}

func (p point) toJson(m *mapifier) {
	m.pair("time", p.time)
	if p.timeStart.notEmpty() {
		m.pair("time.start", p.timeStart)
	}
	if p.flags != 0 {
		m.pair("flags", p.flags)
	}
	m.pair("req", p.req)
}

type pointlike interface {
	value
	getPoint() point
}

func (p point) getPoint() point {
	return p
}

type numberPoint struct {
	point
	value     value
	exemplars []exemplar
}

func (np numberPoint) toJson(w io.Writer) {
	ma := mapify(w)
	defer ma.done()
	np.point.toJson(&ma)
	if np.value != nil {
		ma.pair("val", np.value)
	}
	if len(np.exemplars) > 0 {
		a := ma.array("examplars")
		for _, e := range np.exemplars {
			a.item(e)
		}
		a.done()
	}
}

type exemplar struct {
	time  timestampValue
	value value
	attr  mapValue
	trace traceId
	span  spanId
}

func (e exemplar) toJson(w io.Writer) {
	ma := mapify(w)
	defer ma.done()
	ma.pair("time", e.time)
	ma.pair("value", e.value)
	if e.attr.notEmpty() {
		ma.pair("attr", e.attr)
	}
	if e.trace.notEmpty() || e.span.notEmpty() {
		ma.pair("span", traceSpanId{
			traceId: e.trace,
			spanId:  e.span,
		})
	}
}

type histolikePoint struct {
	point
	count uint64
	has   struct {
		sum bool
		min bool
		max bool
	}
	sum       float64
	min       float64
	max       float64
	exemplars []exemplar
}

func (hlp histolikePoint) toJson(m *mapifier) {
	hlp.point.toJson(m)
	m.pair("cnt", uintValue(hlp.count))
	if hlp.has.sum {
		m.pair("sum", doubleValue(hlp.sum))
	}
	if hlp.has.min {
		m.pair("min", doubleValue(hlp.min))
	}
	if hlp.has.max {
		m.pair("max", doubleValue(hlp.max))
	}
	if len(hlp.exemplars) > 0 {
		a := m.array("examplars")
		for _, e := range hlp.exemplars {
			a.item(e)
		}
		a.done()
	}
}

type histogramPoint struct {
	histolikePoint
	buckets []uint64
	bounds  []float64
}

func (hp histogramPoint) toJson(w io.Writer) {
	ma := mapify(w)
	defer ma.done()
	hp.histolikePoint.toJson(&ma)
	if hp.buckets != nil {
		a := ma.array("buckets")
		for _, b := range hp.buckets {
			a.item(uintValue(b))
		}
		a.done()
	}
	if hp.bounds != nil {
		a := ma.array("bounds")
		for _, b := range hp.bounds {
			a.item(doubleValue(b))
		}
		a.done()
	}
}

type exponentialHistogramPoint struct {
	histolikePoint
	scale    int32
	zeros    uint64
	zeroThre float64
	pos      exponentialBuckets
	neg      exponentialBuckets
}

type exponentialBuckets struct {
	off     int32
	buckets []uint64
}

func (ehp exponentialHistogramPoint) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	ehp.histolikePoint.toJson(&m)
	m.pair("scale", intValue(ehp.scale))
	m.pair("zeros", uintValue(ehp.scale))
	if ehp.zeroThre != 0 {
		m.pair("zeros.thre", doubleValue(ehp.zeroThre))
	}

	m.pair("pos.off", intValue(ehp.pos.off))
	a := m.array("pos")
	for _, b := range ehp.pos.buckets {
		a.item(uintValue(b))
	}
	a.done()

	m.pair("neg.off", intValue(ehp.neg.off))
	a = m.array("neg")
	for _, b := range ehp.neg.buckets {
		a.item(uintValue(b))
	}
	a.done()
}

type summaryPoint struct {
	point
	count     uint64
	sum       float64
	quantiles []quantileValue
}

type quantileValue struct {
	q float64
	v float64
}

func (sp summaryPoint) toJson(w io.Writer) {
	m := mapify(w)
	defer m.done()
	sp.point.toJson(&m)
	m.pair("cnt", uintValue(sp.count))
	m.pair("sum", doubleValue(sp.sum))
	a := m.array("quantiles")
	defer a.done()
	for _, qv := range sp.quantiles {
		m2 := a.submap()
		m2.pair("q", doubleValue(qv.q))
		m2.pair("v", doubleValue(qv.v))
		m2.done()
	}
}
