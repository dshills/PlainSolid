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

const nonplanarSplineVoidsNativeFormat = "plainsolid-step-nonplanar-spline-multiple-voids-source"

type nonplanarSplineVoidsEnvelope struct {
	Format                string
	Version               int
	Source                string
	BodyID                int64
	ShellIDs, OrientedIDs []int64
	Faces, Edges          [][]int64
	PCurves               [][]SplineShellPCurveEvidence
	NativeFaces           [][]int
	Generated             string
	Native                json.RawMessage
	BodiesNative          []json.RawMessage
}

func nonplanarSplineVoidsClaim(ctx context.Context, m NonplanarSplineVoidsSource) (nonplanarSplineVoidsEnvelope, error) {
	var zero nonplanarSplineVoidsEnvelope
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if m.Outer.Name != m.Name {
		return zero, geom.ErrInvalid
	}
	for _, b := range m.Voids {
		if b.Name != m.Name {
			return zero, geom.ErrInvalid
		}
	}
	generated, e := nonplanarVoidsGenerated(ctx, m)
	if e != nil {
		return zero, e
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		return zero, e
	}
	q := nonplanarSplineVoidsEnvelope{Format: nonplanarSplineVoidsNativeFormat, Version: 1, Source: m.original, BodyID: m.bodyID, ShellIDs: m.SourceShells(), OrientedIDs: m.SourceVoidOccurrences(), Faces: m.SourceFaces(), Edges: m.SourceEdges(), PCurves: m.PCurves(), NativeFaces: m.NativeFaces(), Generated: string(generated), Native: bytes.TrimSpace(native.Bytes())}
	bodies := append([]SplineShellBody{m.Outer}, m.Voids...)
	for _, b := range bodies {
		var raw bytes.Buffer
		if e = trimmed.WriteJSON(&raw, b.Solid); e != nil {
			return zero, e
		}
		q.BodiesNative = append(q.BodiesNative, bytes.TrimSpace(raw.Bytes()))
	}
	return q, nil
}
func checkedNonplanarSplineVoids(ctx context.Context, m NonplanarSplineVoidsSource, l Limits) (nonplanarSplineVoidsEnvelope, error) {
	q, e := nonplanarSplineVoidsClaim(ctx, m)
	if e != nil {
		return q, e
	}
	p, e := ReadNonplanarSplineVoidsSource(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return q, e
	}
	want, e := nonplanarSplineVoidsClaim(ctx, p)
	if e != nil {
		return q, e
	}
	if !reflect.DeepEqual(q, want) {
		return q, geom.ErrInvalid
	}
	return q, nil
}
func WriteNonplanarSplineVoidsSource(ctx context.Context, w io.Writer, m NonplanarSplineVoidsSource, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineVoids(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Source)
	if e == nil && n != len(q.Source) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNonplanarSplineVoidsSourceJSON(ctx context.Context, w io.Writer, m NonplanarSplineVoidsSource) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineVoids(ctx, m, Limits{})
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
func ReadNonplanarSplineVoidsSourceJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NonplanarSplineVoidsSource, error) {
	var zero NonplanarSplineVoidsSource
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
	var q nonplanarSplineVoidsEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nonplanarSplineVoidsNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.ShellIDs) < 3 || len(q.ShellIDs) > 5 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNonplanarSplineVoidsSource(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := nonplanarSplineVoidsClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
