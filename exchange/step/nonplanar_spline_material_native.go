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

const nonplanarSplineMaterialFormat = "plainsolid-step-nonplanar-spline-material"

type nonplanarSplineMaterialEnvelope struct {
	Format       string
	Version      int
	Kind         NonplanarSplineMaterialKind
	Source, Name string
	LengthUnit   float64
	Provenance   NonplanarSplineMaterialProvenance
	Native       json.RawMessage
}

func nonplanarSplineMaterialClaim(ctx context.Context, m NonplanarSplineMaterial) (nonplanarSplineMaterialEnvelope, error) {
	var zero nonplanarSplineMaterialEnvelope
	if ctx == nil || m.payload == nil || m.source == "" {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	var b bytes.Buffer
	if e := trimmed.WriteJSON(&b, m.solid); e != nil {
		return zero, e
	}
	return nonplanarSplineMaterialEnvelope{nonplanarSplineMaterialFormat, 1, m.kind, m.source, m.name, m.unit, m.Provenance(), bytes.TrimSpace(b.Bytes())}, nil
}
func checkedNonplanarSplineMaterial(ctx context.Context, m NonplanarSplineMaterial, l Limits) (nonplanarSplineMaterialEnvelope, error) {
	q, e := nonplanarSplineMaterialClaim(ctx, m)
	if e != nil {
		return q, e
	}
	p, e := ReadNonplanarSplineMaterial(ctx, bytes.NewBufferString(m.source), l)
	if e != nil {
		return q, e
	}
	want, e := nonplanarSplineMaterialClaim(ctx, p)
	if e != nil {
		return q, e
	}
	if !reflect.DeepEqual(q, want) {
		return q, geom.ErrInvalid
	}
	return q, nil
}
func WriteNonplanarSplineMaterialSource(ctx context.Context, w io.Writer, m NonplanarSplineMaterial, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineMaterial(ctx, m, l)
	if e != nil {
		return e
	}
	n, e := io.WriteString(w, q.Source)
	if e == nil && n != len(q.Source) {
		e = io.ErrShortWrite
	}
	return e
}

// WriteNonplanarSplineMaterial emits normalized standard polynomial material.
// Rational-speed and multispan-UV originals become quadratic-shell generated
// representations; their original source mode remains available separately.
func WriteNonplanarSplineMaterial(ctx context.Context, w io.Writer, m NonplanarSplineMaterial, l Limits) error {
	if w == nil {
		return geom.ErrInvalid
	}
	if _, e := checkedNonplanarSplineMaterial(ctx, m, l); e != nil {
		return e
	}
	var out []byte
	var e error
	var model SplineShellModel
	switch q := m.payload.(type) {
	case NonplanarSplineShellSource:
		model = q.Model
	case CubicNonplanarSplineShellSource:
		model = q.Model
	case MultispanNonplanarSplineShellSource:
		model = q.Model
	case RationalSpeedNonplanarSplineShellSource:
		model = q.Model
	case NonplanarSplineCavitySource:
		out, e = nonplanarCavityGenerated(ctx, q)
	case NonplanarSplineVoidsSource:
		out, e = nonplanarVoidsGenerated(ctx, q)
	default:
		return geom.ErrInvalid
	}
	if e != nil {
		return e
	}
	if model.Bodies != nil {
		f, err := nonplanarPolynomialShellFile(ctx, model.Bodies, model.LengthUnit)
		if err != nil {
			return err
		}
		out = serializeWrappedProofFile(f)
	}
	if _, e = ReadNonplanarSplineMaterial(ctx, bytes.NewReader(out), l); e != nil {
		return e
	}
	n, e := w.Write(out)
	if e == nil && n != len(out) {
		e = io.ErrShortWrite
	}
	return e
}
func WriteNonplanarSplineMaterialJSON(ctx context.Context, w io.Writer, m NonplanarSplineMaterial) error {
	if w == nil {
		return geom.ErrInvalid
	}
	q, e := checkedNonplanarSplineMaterial(ctx, m, Limits{})
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
func ReadNonplanarSplineMaterialJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NonplanarSplineMaterial, error) {
	var zero NonplanarSplineMaterial
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
	var q nonplanarSplineMaterialEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nonplanarSplineMaterialFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNonplanarSplineMaterial(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := nonplanarSplineMaterialClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}
