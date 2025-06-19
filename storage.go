package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type storage struct {
	sync.Mutex
	verbose   bool
	requests  map[reqId]requestMeta
	resources map[resId]resource
	scopes    map[scopeId]scope
	traces    map[traceId]*trace
	logs      []log
	metrics   map[hashId]*metric
}

func newStorage(verbose bool) *storage {
	st := &storage{verbose: verbose}
	st.reset()
	return st
}

func (st *storage) reset() {
	st.Lock()
	defer st.Unlock()
	st.requests = map[reqId]requestMeta{}
	st.resources = map[resId]resource{}
	st.scopes = map[scopeId]scope{}
	st.traces = map[traceId]*trace{}
	st.logs = nil
	st.metrics = map[hashId]*metric{}
}

func (st *storage) receiveRequestMeta(req requestMeta) reqId {
	reqId := reqId(hashValue(req))
	st.Lock()
	if _, ok := st.requests[reqId]; !ok {
		st.requests[reqId] = req
	}
	st.Unlock()

	if st.verbose {
		fmt.Printf("req: %s\n", jsonToString(req))
	}

	return reqId
}

func (st *storage) receiveResource(r pcommon.Resource, schemaUrl string) resId {
	res := resource{
		attr:        convertMap(r.Attributes()),
		attrDropped: r.DroppedAttributesCount(),
		schema:      schemaUrl,
	}

	resId := resId(hashValue(res))
	st.Lock()
	if res2, ok := st.resources[resId]; ok {
		res = res2
	} else {
		st.resources[resId] = res
	}
	st.Unlock()

	if st.verbose {
		fmt.Printf("res: %s\n", jsonToString(res))
	}

	return resId
}

func (st *storage) receiveScope(sc pcommon.InstrumentationScope, schemaUrl string) scopeId {
	scope := scope{
		name:        sc.Name(),
		version:     sc.Version(),
		attr:        convertMap(sc.Attributes()),
		attrDropped: sc.DroppedAttributesCount(),
		schema:      schemaUrl,
	}

	scopeId := scopeId(hashValue(scope))
	st.Lock()
	if scope2, ok := st.scopes[scopeId]; ok {
		scope = scope2
	} else {
		st.scopes[scopeId] = scope
	}
	st.Unlock()

	if st.verbose {
		fmt.Printf("  scope: %s\n", jsonToString(scope))
	}

	return scopeId
}

func (st *storage) receiveTraces(t ptrace.Traces, req requestMeta) {
	reqId := st.receiveRequestMeta(req)

	rss := t.ResourceSpans()
	for i := range rss.Len() {
		rs := rss.At(i)

		resId := st.receiveResource(rs.Resource(), rs.SchemaUrl())

		scss := rs.ScopeSpans()
		for j := range scss.Len() {
			scs := scss.At(j)

			scopeId := st.receiveScope(scs.Scope(), scs.SchemaUrl())

			sps := scs.Spans()
			for k := range sps.Len() {
				sp := sps.At(k)

				tid := traceId(sp.TraceID())
				sid := spanId(sp.SpanID())
				sp2 := span{
					spanSummary: spanSummary{
						parent: spanId(sp.ParentSpanID()),
						name:   sp.Name(),
						start:  timestampValue(sp.StartTimestamp()),
						end:    timestampValue(sp.EndTimestamp()),
					},
					req:           reqId,
					res:           resId,
					scope:         scopeId,
					statusMsg:     sp.Status().Message(),
					kind:          sp.Kind().String(),
					attr:          convertMap(sp.Attributes()),
					attrDropped:   sp.DroppedAttributesCount(),
					state:         sp.TraceState().AsRaw(),
					flags:         flagsValue(sp.Flags()),
					eventsDropped: sp.DroppedEventsCount(),
					linksDropped:  sp.DroppedLinksCount(),
				}
				if code := sp.Status().Code(); code != ptrace.StatusCodeUnset {
					sp2.status = code.String()
				}
				es := sp.Events()
				for i := range es.Len() {
					e := es.At(i)
					sp2.events = append(sp2.events, event{
						name:        e.Name(),
						time:        timestampValue(e.Timestamp()),
						attr:        convertMap(e.Attributes()),
						attrDropped: e.DroppedAttributesCount(),
					})
				}
				ls := sp.Links()
				for i := range ls.Len() {
					l := ls.At(i)
					sp2.links = append(sp2.links, link{
						trace:       traceId(l.TraceID()),
						span:        spanId(l.SpanID()),
						attr:        convertMap(l.Attributes()),
						attrDropped: l.DroppedAttributesCount(),
						state:       l.TraceState().AsRaw(),
					})
				}

				st.Lock()
				tr, ok := st.traces[tid]
				if !ok {
					tr = &trace{
						spans: make(map[spanId]span),
					}
					st.traces[tid] = tr
				}
				if _, ok := tr.spans[sid]; ok {
					fmt.Fprintf(os.Stderr, "Warning: span %x received twice\n", sid)
				} else {
					tr.spans[sid] = sp2
				}
				st.Unlock()

				if st.verbose {
					fmt.Printf("    span: %s\n", jsonToString(sp2))
				}
			}
		}
	}
}

func (st *storage) receiveLogs(l plog.Logs, req requestMeta) {
	reqId := st.receiveRequestMeta(req)

	rls := l.ResourceLogs()
	for i := range rls.Len() {
		rl := rls.At(i)

		resId := st.receiveResource(rl.Resource(), rl.SchemaUrl())

		scls := rl.ScopeLogs()
		for j := range scls.Len() {
			scl := scls.At(j)

			scopeId := st.receiveScope(scl.Scope(), scl.SchemaUrl())

			lrs := scl.LogRecords()
			for k := range lrs.Len() {
				lr := lrs.At(k)

				log := log{
					req:         reqId,
					res:         resId,
					scope:       scopeId,
					time:        timestampValue(lr.Timestamp()),
					timeObs:     timestampValue(lr.ObservedTimestamp()),
					sevText:     lr.SeverityText(),
					event:       lr.EventName(),
					attr:        convertMap(lr.Attributes()),
					attrDropped: lr.DroppedAttributesCount(),
					flags:       flagsValue(lr.Flags()),
					trace:       traceId(lr.TraceID()),
					span:        spanId(lr.SpanID()),
				}
				if sev := lr.SeverityNumber(); sev != plog.SeverityNumberUnspecified {
					log.sev = sev.String()
				}
				if body := lr.Body(); body.Type() != pcommon.ValueTypeEmpty {
					log.body = convertValue(body)
				}

				if log.event != "" {
					log.simpleBody = log.event
				} else if log.body != nil {
					if logStr, ok := log.body.(stringValue); ok {
						log.simpleBody = string(logStr)
					} else {
						log.simpleBody = jsonToString(log.body)
					}
				} else {
					log.simpleBody = "<no body>"
				}
				const maxLogLength = 130
				nlIdx := strings.IndexByte(log.simpleBody[:min(maxLogLength, len(log.simpleBody))], '\n')
				if len(log.simpleBody) > maxLogLength || nlIdx != -1 {
					i := maxLogLength
					if nlIdx != -1 {
						i = nlIdx
					}
					for !utf8.RuneStart(log.simpleBody[i]) {
						i--
					}
					log.simpleBody = log.simpleBody[:i] + " [...]"
				}

				if log.time.notEmpty() {
					log.simpleTime = log.time
				} else if log.timeObs.notEmpty() {
					log.simpleTime = log.timeObs
				} else {
					log.simpleTime = timestampValue(time.Now().UnixNano())
				}

				st.Lock()
				st.logs = append(st.logs, log)
				st.Unlock()

				if st.verbose {
					fmt.Printf("    log: %s\n", jsonToString(log))
				}
			}
		}
	}
}

type pointGetter interface {
	Attributes() pcommon.Map
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Flags() pmetric.DataPointFlags
}

var _ pointGetter = pmetric.NumberDataPoint{}
var _ pointGetter = pmetric.HistogramDataPoint{}
var _ pointGetter = pmetric.ExponentialHistogramDataPoint{}
var _ pointGetter = pmetric.SummaryDataPoint{}

type pointSlice[T pointGetter] interface {
	At(i int) T
	Len() int
}

func receivePoints[T pointGetter](st *storage, reqId reqId, m *metric, ps pointSlice[T], makePoint func(point, T) pointlike) {
	for i := range ps.Len() {
		dp := ps.At(i)
		attr := convertMap(dp.Attributes())
		msId := hashId(hashValue(attr))
		st.Lock()
		ms, ok := m.streams[msId]
		if !ok {
			ms = &metricStream{attr: attr}
			m.streams[msId] = ms
		}
		st.Unlock()

		point := makePoint(point{
			time:      timestampValue(dp.Timestamp()),
			timeStart: timestampValue(dp.StartTimestamp()),
			flags:     flagsValue(dp.Flags()),
			req:       reqId,
		}, dp)

		st.Lock()
		ms.points = append(ms.points, point)
		st.Unlock()
	}
}

func convertExemplars(es pmetric.ExemplarSlice) []exemplar {
	var exemplars []exemplar
	if es.Len() != 0 {
		exemplars = make([]exemplar, 0, es.Len())
	}
	for j := range es.Len() {
		e := es.At(j)
		examplar := exemplar{
			time:  timestampValue(e.Timestamp()),
			attr:  convertMap(e.FilteredAttributes()),
			span:  spanId(e.SpanID()),
			trace: traceId(e.TraceID()),
		}

		switch e.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			examplar.value = intValue(e.IntValue())
		case pmetric.ExemplarValueTypeDouble:
			examplar.value = doubleValue(e.DoubleValue())
		default:
			panic("unknown metric examplar value type")
		}

		exemplars = append(exemplars, examplar)
	}
	return exemplars
}

func (st *storage) receiveNumberPoints(reqId reqId, m *metric, ndps pmetric.NumberDataPointSlice) {
	receivePoints(st, reqId, m, ndps, func(p point, ndp pmetric.NumberDataPoint) pointlike {
		numberPoint := numberPoint{
			point:     p,
			exemplars: convertExemplars(ndp.Exemplars()),
		}

		switch ndp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			numberPoint.value = intValue(ndp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			numberPoint.value = doubleValue(ndp.DoubleValue())
		case pmetric.NumberDataPointValueTypeEmpty:
			numberPoint.value = nil
		default:
			panic("unknown metric point value type")
		}

		return numberPoint
	})
}

type histolikePointGetter interface {
	pointGetter
	Count() uint64
	HasSum() bool
	HasMin() bool
	HasMax() bool
	Sum() float64
	Min() float64
	Max() float64
	Exemplars() pmetric.ExemplarSlice
}

var _ histolikePointGetter = pmetric.HistogramDataPoint{}
var _ histolikePointGetter = pmetric.ExponentialHistogramDataPoint{}

func receiveHistolikePoints[T histolikePointGetter](st *storage, reqId reqId, m *metric, ps pointSlice[T], makePoint func(histolikePoint, T) pointlike) {
	receivePoints(st, reqId, m, ps, func(p point, dp T) pointlike {
		return makePoint(histolikePoint{
			point: p,
			count: dp.Count(),
			has: struct {
				sum bool
				min bool
				max bool
			}{
				sum: dp.HasSum(),
				min: dp.HasMin(),
				max: dp.HasMax(),
			},
			sum:       dp.Sum(),
			min:       dp.Min(),
			max:       dp.Max(),
			exemplars: convertExemplars(dp.Exemplars()),
		}, dp)
	})
}

func (st *storage) receiveMetrics(m pmetric.Metrics, req requestMeta) {
	reqId := st.receiveRequestMeta(req)

	rms := m.ResourceMetrics()
	for i := range rms.Len() {
		rm := rms.At(i)

		resId := st.receiveResource(rm.Resource(), rm.SchemaUrl())

		scms := rm.ScopeMetrics()
		for j := range scms.Len() {
			scm := scms.At(j)

			scopeId := st.receiveScope(scm.Scope(), scm.SchemaUrl())

			ms := scm.Metrics()
			for k := range ms.Len() {
				m := ms.At(k)

				mi := metricIdentity{
					res:   resId,
					scope: scopeId,
					name:  m.Name(),
					type_: m.Type().String(),
					unit:  m.Unit(),
				}
				switch m.Type() {
				case pmetric.MetricTypeGauge:
				case pmetric.MetricTypeSum:
					s := m.Sum()
					mi.tempo = s.AggregationTemporality().String()
					mi.mono = s.IsMonotonic()
				case pmetric.MetricTypeHistogram:
					h := m.Histogram()
					mi.tempo = h.AggregationTemporality().String()
				case pmetric.MetricTypeExponentialHistogram:
					eh := m.ExponentialHistogram()
					mi.tempo = eh.AggregationTemporality().String()
				case pmetric.MetricTypeSummary:
				}

				metricId := getMetricId(mi)

				desc := m.Description()
				meta := convertMap(m.Metadata())

				st.Lock()
				m2, ok := st.metrics[metricId]
				if ok {
					if m2.desc != desc || hashValue(m2.meta) != hashValue(meta) {
						if !m2.conflict {
							m2.conflict = true
							fmt.Fprintf(os.Stderr, "Warning: conflicting metadata for metric identity %+v\n", mi)
						}
						if len(desc) > len(m2.desc) {
							m2.desc = desc
						}
					}
				} else {
					m2 = &metric{
						metricIdentity: mi,
						desc:           desc,
						meta:           meta,
						streams:        make(map[hashId]*metricStream),
					}
					st.metrics[metricId] = m2
				}
				st.Unlock()

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					st.receiveNumberPoints(reqId, m2, m.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					st.receiveNumberPoints(reqId, m2, m.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					receiveHistolikePoints(st, reqId, m2, m.Histogram().DataPoints(), func(hlp histolikePoint, hdp pmetric.HistogramDataPoint) pointlike {
						hp := histogramPoint{
							histolikePoint: hlp,
						}

						uis := hdp.BucketCounts()
						if uis.Len() > 0 {
							hp.buckets = make([]uint64, 0, uis.Len())
						}
						for l := range uis.Len() {
							hp.buckets = append(hp.buckets, uis.At(l))
						}

						fs := hdp.ExplicitBounds()
						if fs.Len() > 0 {
							hp.bounds = make([]float64, 0, fs.Len())
						}
						for l := range fs.Len() {
							hp.bounds = append(hp.bounds, fs.At(l))
						}

						return hp
					})
				case pmetric.MetricTypeExponentialHistogram:
					receiveHistolikePoints(st, reqId, m2, m.ExponentialHistogram().DataPoints(), func(hlp histolikePoint, ehdp pmetric.ExponentialHistogramDataPoint) pointlike {
						return exponentialHistogramPoint{
							histolikePoint: hlp,
							scale:          ehdp.Scale(),
							zeros:          ehdp.ZeroCount(),
							zeroThre:       ehdp.ZeroThreshold(),
							pos: exponentialBuckets{
								off:     ehdp.Positive().Offset(),
								buckets: ehdp.Positive().BucketCounts().AsRaw(),
							},
							neg: exponentialBuckets{
								off:     ehdp.Negative().Offset(),
								buckets: ehdp.Negative().BucketCounts().AsRaw(),
							},
						}
					})

				case pmetric.MetricTypeSummary:
					receivePoints(st, reqId, m2, m.Summary().DataPoints(), func(p point, sp pmetric.SummaryDataPoint) pointlike {
						qvs := sp.QuantileValues()
						sp2 := summaryPoint{
							point:     p,
							count:     sp.Count(),
							sum:       sp.Sum(),
							quantiles: make([]quantileValue, 0, qvs.Len()),
						}
						for i := range qvs.Len() {
							qv := qvs.At(i)
							sp2.quantiles = append(sp2.quantiles, quantileValue{
								q: qv.Quantile(),
								v: qv.Value(),
							})
						}
						return sp2
					})
				}

				if st.verbose {
					fmt.Printf("    metric: %s\n", jsonToString(m2))
				}
			}
		}
	}
}
