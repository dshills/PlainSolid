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

const rationalSpeedNonplanarSplineShellNativeFormat = "plainsolid-step-rational-speed-nonplanar-spline-shell-source"

type rationalSpeedNonplanarSplineShellEnvelope struct {
	Format                 string
	Version                int
	Source                 string
	BodyID                 int64
	Faces, Edges, Surfaces []int64
	PCurves                []SplineShellPCurveEvidence
	ParameterMaps          []RationalSplineSpeedEvidence
	Canonical              string
	Native                 json.RawMessage
}

func rationalSpeedNonplanarSplineShellClaim(ctx context.Context, m RationalSpeedNonplanarSplineShellSource) (rationalSpeedNonplanarSplineShellEnvelope, error) {
	var zero rationalSpeedNonplanarSplineShellEnvelope
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if len(m.Model.Bodies) != 1 || len(m.faces) != 6 || len(m.edges) != 12 || len(m.curves) != 24 || len(m.maps) != 36 {
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
	return rationalSpeedNonplanarSplineShellEnvelope{rationalSpeedNonplanarSplineShellNativeFormat, 1, m.original, m.bodyID, m.SourceFaces(), m.SourceEdges(), append([]int64(nil), m.surfaces...), m.PCurves(), m.ParameterMaps(), canonical.String(), append(json.RawMessage(nil), bytes.TrimSpace(native.Bytes())...)}, nil
}
func checkedRationalSpeedNonplanarSplineShell(ctx context.Context, m RationalSpeedNonplanarSplineShellSource, l Limits) (rationalSpeedNonplanarSplineShellEnvelope, error) {
	q, e := rationalSpeedNonplanarSplineShellClaim(ctx, m)
	if e != nil {
		return q, e
	}
	p, e := ReadRationalSpeedNonplanarSplineShellSource(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return q, e
	}
	want, e := rationalSpeedNonplanarSplineShellClaim(ctx, p)
	if e != nil {
		return q, e
	}
	if !reflect.DeepEqual(q, want) {
		return q, geom.ErrInvalid
	}
	return q, nil
}
func WriteRationalSpeedNonplanarSplineShellSource(ctx context.Context, w io.Writer, m RationalSpeedNonplanarSplineShellSource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedRationalSpeedNonplanarSplineShell(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Source)
	if e == nil && n != len(q.Source) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteRationalSpeedNonplanarSplineShellSourceJSON(ctx context.Context, w io.Writer, m RationalSpeedNonplanarSplineShellSource) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedRationalSpeedNonplanarSplineShell(ctx, m, Limits{})
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
func ReadRationalSpeedNonplanarSplineShellSourceJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalSpeedNonplanarSplineShellSource, error) {
	var zero RationalSpeedNonplanarSplineShellSource
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
	var q rationalSpeedNonplanarSplineShellEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != rationalSpeedNonplanarSplineShellNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Faces) != 6 || len(q.Edges) != 12 || len(q.Surfaces) != 6 || len(q.PCurves) != 24 || len(q.ParameterMaps) != 36 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadRationalSpeedNonplanarSplineShellSource(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := rationalSpeedNonplanarSplineShellClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
