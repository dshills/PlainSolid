package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const nestedOccurrenceColorsNativeFormat = "plainsolid-step-nested-contextual-appearance"

type nestedOccurrenceColorsEnvelope struct {
	occurrenceColorsEnvelope
	Paths []NestedColoredPath
}

func nestedOccurrenceColorsClaims(ctx context.Context, m NestedOccurrenceColoredAssembly) (nestedOccurrenceColorsEnvelope, error) {
	if len(m.paths) == 0 || len(m.paths) > 64 {
		return nestedOccurrenceColorsEnvelope{}, geom.ErrInvalid
	}
	q, e := occurrenceColorsClaims(ctx, m.model)
	if e != nil {
		return nestedOccurrenceColorsEnvelope{}, e
	}
	q.Format = nestedOccurrenceColorsNativeFormat
	return nestedOccurrenceColorsEnvelope{q, m.Paths()}, nil
}

// WriteNestedOccurrenceColoredAssemblyJSON stages a distinct version-one source-bound
// envelope. Original STEP is re-proved once and all supplied geometry, placement,
// product-local color and contextual ancestry claims must match that proof.
func WriteNestedOccurrenceColoredAssemblyJSON(ctx context.Context, w io.Writer, m NestedOccurrenceColoredAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	actual, e := nestedOccurrenceColorsClaims(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewBufferString(actual.Source), Limits{})
	if e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	expected, e := nestedOccurrenceColorsClaims(ctx, proof)
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

// ReadNestedOccurrenceColoredAssemblyJSON rebuilds the native planar geometry and
// appearance through the original bounded STEP reader exactly once. Stored
// native boundaries and claims are evidence to compare, never repair inputs.
// Cancellation is observed around the bounded legacy source proof, not inside it.
func ReadNestedOccurrenceColoredAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NestedOccurrenceColoredAssembly, error) {
	var zero NestedOccurrenceColoredAssembly
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
	var q nestedOccurrenceColorsEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nestedOccurrenceColorsNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Products) > 16 || len(q.Definitions) > 16 || len(q.Overrides) > 64 || len(q.Occurrences) > 4096 || len(q.Paths) > 64 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	expected, e := nestedOccurrenceColorsClaims(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
