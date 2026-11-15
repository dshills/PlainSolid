package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const quadraticNURBSNativeFormat = "plainsolid-step-quadratic-nurbs-planar-assembly"

type quadraticNURBSNativeEnvelope struct {
	Format   string
	Version  int
	Source   string
	Geometry nurbsAssemblyGeometryClaims
	Surfaces []QuadraticNURBSSurfaceEvidence
	Curves   []QuadraticNURBSPCurveEvidence
	Frames   []ObliqueNURBSProductFrame
	Points   []ObliqueNURBSPointEvidence
}

func quadraticNURBSNativeClaim(ctx context.Context, m QuadraticNURBSPlanarAssembly) (quadraticNURBSNativeEnvelope, error) {
	var zero quadraticNURBSNativeEnvelope
	if len(m.surfaces) == 0 || len(m.surfaces) > 4096 || len(m.curves) == 0 || len(m.curves) > 4096 {
		return zero, geom.ErrInvalid
	}
	p := ObliqueNURBSPlanarAssembly{Geometry: m.Geometry, original: m.original, frames: m.frames, points: m.points}
	q, e := nurbsAssemblyNativeClaims(ctx, nurbsNativeModel{oblique: &p})
	if e != nil {
		return zero, e
	}
	return quadraticNURBSNativeEnvelope{quadraticNURBSNativeFormat, 1, m.original, q.Geometry, m.Surfaces(), m.PCurves(), p.ProductFrames(), p.SourcePoints()}, nil
}

// WriteQuadraticNURBSPlanarAssemblyJSON preserves original quadratic bases and
// all derived material, appearance and ownership claims under a distinct tag.
func WriteQuadraticNURBSPlanarAssemblyJSON(ctx context.Context, w io.Writer, m QuadraticNURBSPlanarAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	actual, e := quadraticNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	p, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewBufferString(actual.Source), Limits{})
	if e != nil {
		return e
	}
	want, e := quadraticNURBSNativeClaim(ctx, p)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, want) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(actual)
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

// ReadQuadraticNURBSPlanarAssemblyJSON constructs from original source once,
// never from cached geometry or alleged affine identities.
func ReadQuadraticNURBSPlanarAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (QuadraticNURBSPlanarAssembly, error) {
	var zero QuadraticNURBSPlanarAssembly
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
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	var q quadraticNURBSNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != quadraticNURBSNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Surfaces) > 4096 || len(q.Curves) > 4096 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	if !nurbsAssemblyEnvelopeBound(nurbsAssemblyEnvelope{Format: nurbsPlanarAssemblyNativeFormat, Version: 1, Mode: nurbsNativeOblique, OriginalSource: q.Source, Geometry: q.Geometry, Oblique: &nurbsObliqueNativeClaims{Frames: q.Frames, Points: q.Points}}) {
		return zero, geom.ErrInvalid
	}
	m, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := quadraticNURBSNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	if !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
