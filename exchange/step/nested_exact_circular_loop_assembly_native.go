package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const nestedExactLoopAssemblyNativeFormat = "plainsolid-step-nested-exact-circular-loop-assembly"

type nestedExactLoopAssemblyNativeEnvelope struct {
	exactLoopAssemblyNativeEnvelope
	GroupID   int64
	Instances []NestedExactCircularLoopInstance
}

func nestedExactLoopAssemblyNativeClaim(ctx context.Context, m NestedExactCircularLoopAssembly) (nestedExactLoopAssemblyNativeEnvelope, error) {
	q, e := exactLoopAssemblyNativeClaimMode(ctx, m.ExactCircularLoopAssembly, true)
	q.Format = nestedExactLoopAssemblyNativeFormat
	if len(m.Instances) < 2 || len(m.Instances) > 16 {
		return nestedExactLoopAssemblyNativeEnvelope{}, geom.ErrInvalid
	}
	return nestedExactLoopAssemblyNativeEnvelope{q, m.GroupID, m.Instances}, e
}

// WriteNestedExactCircularLoopAssemblyJSON stores the original source, one local
// native solid and all exact shared occurrence claims after full source reproof.
// It is a distinct version-one format, not an extension of standalone tags.
func WriteNestedExactCircularLoopAssemblyJSON(ctx context.Context, w io.Writer, m NestedExactCircularLoopAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := nestedExactLoopAssemblyNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewBufferString(q.Source), Limits{})
	if e != nil {
		return e
	}
	want, e := nestedExactLoopAssemblyNativeClaim(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, want) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 8<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadNestedExactCircularLoopAssemblyJSON rebuilds from original STEP once and compares
// native local geometry, source role IDs, product ownership and exact rigid uses.
// Failed validation returns no partial material or occurrence result.
func ReadNestedExactCircularLoopAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NestedExactCircularLoopAssembly, error) {
	var zero NestedExactCircularLoopAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 8 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 8<<20 {
		return zero, geom.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var q nestedExactLoopAssemblyNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nestedExactLoopAssemblyNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNestedExactCircularLoopAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := nestedExactLoopAssemblyNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	q.Body.Native, e = independentCompact(q.Body.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
