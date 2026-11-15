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

const multispanNonplanarSplineShellNativeFormat = "plainsolid-step-multispan-nonplanar-spline-shell-source"

type multispanNonplanarSplineShellEnvelope struct {
	Format                 string
	Version                int
	Source                 string
	BodyID                 int64
	Faces, Edges, Surfaces []int64
	PCurves                []MultispanNURBSPCurveEvidence
	Canonical              string
	Native                 json.RawMessage
}

func multispanNonplanarSplineShellClaim(ctx context.Context, m MultispanNonplanarSplineShellSource) (multispanNonplanarSplineShellEnvelope, error) {
	var zero multispanNonplanarSplineShellEnvelope
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if len(m.Model.Bodies) != 1 || len(m.faces) != 6 || len(m.edges) != 12 || len(m.curves) != 24 {
		return zero, geom.ErrInvalid
	}
	var canonical, native bytes.Buffer
	if e := WriteSplineShells(&canonical, m.Model); e != nil {
		return zero, e
	}
	if e := trimmed.WriteJSON(&native, m.Model.Bodies[0].Solid); e != nil {
		return zero, e
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return multispanNonplanarSplineShellEnvelope{multispanNonplanarSplineShellNativeFormat, 1, m.original, m.bodyID, m.SourceFaces(), m.SourceEdges(), append([]int64(nil), m.surfaces...), m.PCurves(), canonical.String(), append(json.RawMessage(nil), bytes.TrimSpace(native.Bytes())...)}, nil
}
func checkedMultispanNonplanarSplineShell(ctx context.Context, m MultispanNonplanarSplineShellSource, l Limits) (multispanNonplanarSplineShellEnvelope, error) {
	q, e := multispanNonplanarSplineShellClaim(ctx, m)
	if e != nil {
		return q, e
	}
	p, e := ReadMultispanNonplanarSplineShellSource(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return q, e
	}
	want, e := multispanNonplanarSplineShellClaim(ctx, p)
	if e != nil {
		return q, e
	}
	if !reflect.DeepEqual(q, want) {
		return q, geom.ErrInvalid
	}
	return q, nil
}
func WriteMultispanNonplanarSplineShellSource(ctx context.Context, w io.Writer, m MultispanNonplanarSplineShellSource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedMultispanNonplanarSplineShell(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Source)
	if e == nil && n != len(q.Source) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteMultispanNonplanarSplineShellSourceJSON(ctx context.Context, w io.Writer, m MultispanNonplanarSplineShellSource) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedMultispanNonplanarSplineShell(ctx, m, Limits{})
	if e != nil {
		return e
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 16<<20 {
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
func ReadMultispanNonplanarSplineShellSourceJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (MultispanNonplanarSplineShellSource, error) {
	var zero MultispanNonplanarSplineShellSource
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
	var q multispanNonplanarSplineShellEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != multispanNonplanarSplineShellNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Faces) != 6 || len(q.Edges) != 12 || len(q.Surfaces) != 6 || len(q.PCurves) != 24 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadMultispanNonplanarSplineShellSource(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := multispanNonplanarSplineShellClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
