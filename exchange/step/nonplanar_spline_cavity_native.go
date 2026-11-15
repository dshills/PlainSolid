package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

const nonplanarSplineCavityNativeFormat = "plainsolid-step-nonplanar-spline-cavity-source"

type nonplanarSplineCavityEnvelope struct {
	Format                           string
	Version                          int
	Source                           string
	BodyID, OrientedID               int64
	ShellIDs                         [2]int64
	Faces, Edges                     [2][]int64
	PCurves                          [2][]SplineShellPCurveEvidence
	History                          trimmed.SplineShellBooleanHistory
	Generated                        string
	Native, OuterNative, InnerNative json.RawMessage
}

func nonplanarSplineCavityClaim(ctx context.Context, m NonplanarSplineCavitySource) (nonplanarSplineCavityEnvelope, error) {
	var zero nonplanarSplineCavityEnvelope
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if m.Outer.Name != m.Name || m.Inner.Name != m.Name {
		return zero, geom.ErrInvalid
	}
	generated, e := nonplanarCavityGenerated(ctx, m)
	if e != nil {
		return zero, e
	}
	var native, outer, inner bytes.Buffer
	for _, p := range []struct {
		w *bytes.Buffer
		s trimmed.Solid
	}{{&native, m.Solid}, {&outer, m.Outer.Solid}, {&inner, m.Inner.Solid}} {
		if e := trimmed.WriteJSON(p.w, p.s); e != nil {
			return zero, e
		}
	}
	q := nonplanarSplineCavityEnvelope{Format: nonplanarSplineCavityNativeFormat, Version: 1, Source: m.original, BodyID: m.bodyID, OrientedID: m.orientedID, ShellIDs: m.shellIDs, Faces: m.SourceFaces(), Edges: m.SourceEdges(), PCurves: m.curves, History: m.History(), Generated: string(generated), Native: bytes.TrimSpace(native.Bytes()), OuterNative: bytes.TrimSpace(outer.Bytes()), InnerNative: bytes.TrimSpace(inner.Bytes())}
	return q, nil
}
func checkedNonplanarSplineCavity(ctx context.Context, m NonplanarSplineCavitySource, l Limits) (nonplanarSplineCavityEnvelope, error) {
	q, e := nonplanarSplineCavityClaim(ctx, m)
	if e != nil {
		return q, e
	}
	p, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return q, e
	}
	want, e := nonplanarSplineCavityClaim(ctx, p)
	if e != nil {
		return q, e
	}
	if !reflect.DeepEqual(q, want) {
		return q, geom.ErrInvalid
	}
	return q, nil
}
func WriteNonplanarSplineCavitySource(ctx context.Context, w io.Writer, m NonplanarSplineCavitySource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineCavity(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Source)
	if e == nil && n != len(q.Source) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNonplanarSplineCavitySourceJSON(ctx context.Context, w io.Writer, m NonplanarSplineCavitySource) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineCavity(ctx, m, Limits{})
	if e != nil {
		return e
	}
	raw, e := json.Marshal(q)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > 16<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
func ReadNonplanarSplineCavitySourceJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NonplanarSplineCavitySource, error) {
	var zero NonplanarSplineCavitySource
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return zero, geom.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var q nonplanarSplineCavityEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nonplanarSplineCavityNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNonplanarSplineCavitySource(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := nonplanarSplineCavityClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
