package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type value interface {
	toJson(w io.Writer)
}

type hashableValue interface {
	value
	hashInto(h hash.Hash64)
}

type boolValue bool
type intValue int64
type uintValue uint64
type timestampValue uint64
type flagsValue uint32
type doubleValue float64
type stringValue string
type bytesValue string
type arrayValue struct {
	Items []hashableValue
}
type mapValue struct {
	Pairs []pair
}
type pair struct {
	K string
	V hashableValue
}

func (bv bytesValue) String() string {
	return hex.EncodeToString([]byte(bv))
}

func (a *arrayValue) add(v hashableValue) {
	a.Items = append(a.Items, v)
}
func (m *mapValue) add(k string, v hashableValue) {
	m.Pairs = append(m.Pairs, pair{K: k, V: v})
}
func (m mapValue) notEmpty() bool {
	return len(m.Pairs) > 0
}
func (t timestampValue) notEmpty() bool {
	return t != 0
}

func convertValue(v pcommon.Value) hashableValue {
	switch v.Type() {
	case pcommon.ValueTypeBool:
		return boolValue(v.Bool())
	case pcommon.ValueTypeInt:
		return intValue(v.Int())
	case pcommon.ValueTypeDouble:
		return doubleValue(v.Double())
	case pcommon.ValueTypeStr:
		return stringValue(v.Str())
	case pcommon.ValueTypeBytes:
		return bytesValue(string(v.Bytes().AsRaw()))
	case pcommon.ValueTypeSlice:
		s := v.Slice()
		var a arrayValue
		for i := range s.Len() {
			a.add(convertValue(s.At(i)))
		}
		return a
	case pcommon.ValueTypeMap:
		return convertMap(v.Map())
	default:
		panic("unknown value type")
	}
}
func convertMap(m pcommon.Map) (m2 mapValue) {
	for k, v := range m.Range {
		m2.add(k, convertValue(v))
	}
	slices.SortFunc(m2.Pairs, func(p1 pair, p2 pair) int {
		return strings.Compare(p1.K, p2.K)
	})
	return
}

func forceWrite(w io.Writer, b []byte) {
	_, err := w.Write(b)
	if err != nil {
		panic("failed to write")
	}
}
func forceWriteString(w io.Writer, s string) {
	_, err := io.WriteString(w, s)
	if err != nil {
		panic("failed to write")
	}
}
func forcePrintf(w io.Writer, f string, args ...any) {
	_, err := fmt.Fprintf(w, f, args...)
	if err != nil {
		panic("failed to write")
	}
}
func forceBinaryWrite(w io.Writer, v any) {
	err := binary.Write(w, binary.BigEndian, v)
	if err != nil {
		panic("failed to write")
	}
}

func (v boolValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, bool(v))
}
func (v intValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, int64(v))
}
func (v uintValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, uint64(v))
}
func (v timestampValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, uint64(v))
}
func (v flagsValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, uint32(v))
}
func (v doubleValue) hashInto(h hash.Hash64) {
	forceBinaryWrite(h, float64(v))
}
func (v stringValue) hashInto(h hash.Hash64) {
	forceWrite(h, []byte(v))
}
func (v bytesValue) hashInto(h hash.Hash64) {
	forceWrite(h, []byte(v))
}
func (v arrayValue) hashInto(h hash.Hash64) {
	for _, x := range v.Items {
		x.hashInto(h)
	}
}
func (v mapValue) hashInto(h hash.Hash64) {
	for _, p := range v.Pairs {
		forceBinaryWrite(h, []byte(p.K))
		p.V.hashInto(h)
	}
}

func (v boolValue) toJson(w io.Writer) {
	if v {
		forceWriteString(w, "true")
	} else {
		forceWriteString(w, "false")
	}
}
func (v intValue) toJson(w io.Writer) {
	forcePrintf(w, `{"_int":"%d"}`, int64(v))
}
func (v uintValue) toJson(w io.Writer) {
	forcePrintf(w, `{"_int":"%d"}`, uint64(v))
}
func (v timestampValue) toJson(w io.Writer) {
	forcePrintf(w, `{"_ts":"%d"}`, uint64(v))
}
func (v flagsValue) toJson(w io.Writer) {
	forcePrintf(w, `{"_flg":"%x"}`, uint32(v))
}
func (v doubleValue) toJson(w io.Writer) {
	forcePrintf(w, "%g", float64(v))
}
func (v stringValue) toJson(w io.Writer) {
	if !utf8.ValidString(string(v)) {
		panic("invalid utf-8 in string")
	}
	forceWriteString(w, "\"")
	for _, c := range v {
		switch c {
		case '"':
			forceWriteString(w, "\\\"")
		case '\\':
			forceWriteString(w, "\\\\")
		case '\r':
			forceWriteString(w, "\\r")
		case '\n':
			forceWriteString(w, "\\n")
		case '\t':
			forceWriteString(w, "\\t")
		default:
			if c < 0x20 {
				forcePrintf(w, "\\u%04x", c)
			} else {
				forcePrintf(w, "%c", c)
			}
		}
	}
	forceWriteString(w, "\"")
}
func (v bytesValue) toJson(w io.Writer) {
	forcePrintf(w, `{"_byt":"%x"}`, []byte(v))
}
func (v arrayValue) toJson(w io.Writer) {
	a := arrayify(w)
	defer a.done()
	for _, x := range v.Items {
		a.item(x)
	}
}
func (v mapValue) toJson(w io.Writer) {
	m := mapify(w)
	for _, p := range v.Pairs {
		m.key(p.K)
		p.V.toJson(w)
	}
	m.done()
}

type mapifier struct {
	w io.Writer
	i int
}
type arrayifier struct {
	w io.Writer
	i int
}

func mapify(w io.Writer) mapifier {
	forcePrintf(w, "{")
	return mapifier{w: w, i: 0}
}

func (m *mapifier) key(k string) {
	if m.i != 0 {
		forceWriteString(m.w, ",")
	}
	m.i++
	if strings.HasPrefix(k, "_") {
		k = "_" + k
	}
	stringValue(k).toJson(m.w)
	forceWriteString(m.w, ":")
}

func (m *mapifier) pair(k string, v value) {
	m.key(k)
	v.toJson(m.w)
}

func (m *mapifier) array(k string) arrayifier {
	m.key(k)
	return arrayify(m.w)
}

func (m *mapifier) submap(k string) mapifier {
	m.key(k)
	return mapify(m.w)
}

func (m *mapifier) done() {
	forceWriteString(m.w, "}")
}

func arrayify(w io.Writer) arrayifier {
	forceWriteString(w, "[")
	return arrayifier{w: w, i: 0}
}

func (a *arrayifier) key() {
	if a.i != 0 {
		forceWriteString(a.w, ",")
	}
	a.i++
}

func (a *arrayifier) item(v value) {
	a.key()
	v.toJson(a.w)
}

func (a *arrayifier) submap() mapifier {
	a.key()
	return mapify(a.w)
}

func (a *arrayifier) done() {
	forceWriteString(a.w, "]")
}

func hashValue(v hashableValue) uint64 {
	h := fnv.New64a()
	v.hashInto(h)
	return h.Sum64()
}

func jsonToString(v value) string {
	b := strings.Builder{}
	v.toJson(&b)
	return b.String()
}
