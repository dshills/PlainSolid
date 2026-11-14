package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const deepSHUONativeFormat = "plainsolid-step-occt-depth-three-shuo"

type deepSHUOEnvelope struct {
	occurrenceColorsEnvelope
	Paths []DeepSHUOColoredPath
	SHUO  DeepSHUOColor
}

func deepSHUOClaims(ctx context.Context, m OCCTDeepSHUOAssembly) (deepSHUOEnvelope, error) {
	if len(m.paths) == 0 || len(m.paths) > 64 {
		return deepSHUOEnvelope{}, geom.ErrInvalid
	}
	q, e := occurrenceColorsClaims(ctx, m.model)
	if e != nil {
		return deepSHUOEnvelope{}, e
	}
	q.Format = deepSHUONativeFormat
	return deepSHUOEnvelope{q, m.Paths(), m.Override()}, nil
}

// WriteOCCTDeepSHUOAssemblyJSON stages a distinct version-one source-bound
// envelope. Original STEP is re-proved once and all supplied geometry, placement,
// product-local color and contextual ancestry claims must match that proof.
func WriteOCCTDeepSHUOAssemblyJSON(ctx context.Context, w io.Writer, m OCCTDeepSHUOAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	actual, e := deepSHUOClaims(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewBufferString(actual.Source), Limits{})
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	expected, e := deepSHUOClaims(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(actual)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > 8<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

// ReadOCCTDeepSHUOAssemblyJSON rebuilds the native planar geometry and
// appearance through the original bounded STEP reader exactly once. Stored
// native boundaries and claims are evidence to compare, never repair inputs.
// Cancellation is observed around the bounded legacy source proof, not inside it.
func ReadOCCTDeepSHUOAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OCCTDeepSHUOAssembly, error) {
	var zero OCCTDeepSHUOAssembly
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
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	var q deepSHUOEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != deepSHUONativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Products) > 16 || len(q.Definitions) > 16 || len(q.Overrides) > 64 || len(q.Occurrences) > 4096 || len(q.Paths) > 64 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	expected, e := deepSHUOClaims(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
