package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const multispanNURBSNativeFormat = "plainsolid-step-multispan-nurbs-planar-assembly"

type multispanNURBSNativeEnvelope struct {
	Format   string
	Version  int
	Source   string
	Geometry nurbsAssemblyGeometryClaims
	Surfaces []MultispanNURBSSurfaceEvidence
	Curves   []MultispanNURBSPCurveEvidence
	Frames   []ObliqueNURBSProductFrame
	Points   []ObliqueNURBSPointEvidence
}

func multispanNURBSNativeClaim(ctx context.Context, m MultispanNURBSPlanarAssembly) (multispanNURBSNativeEnvelope, error) {
	var zero multispanNURBSNativeEnvelope
	if len(m.surfaces) == 0 || len(m.surfaces) > 4096 || len(m.curves) == 0 || len(m.curves) > 4096 {
		return zero, geom.ErrInvalid
	}
	p := ObliqueNURBSPlanarAssembly{Geometry: m.Geometry, original: m.original, frames: m.frames, points: m.points}
	q, e := nurbsAssemblyNativeClaims(ctx, nurbsNativeModel{oblique: &p})
	if e != nil {
		return zero, e
	}
	return multispanNURBSNativeEnvelope{multispanNURBSNativeFormat, 1, m.original, q.Geometry, m.Surfaces(), m.PCurves(), p.ProductFrames(), p.SourcePoints()}, nil
}

// WriteMultispanNURBSPlanarAssemblyJSON preserves original multispan bases and
// all derived material, appearance and ownership claims under a distinct tag.
func WriteMultispanNURBSPlanarAssemblyJSON(ctx context.Context, w io.Writer, m MultispanNURBSPlanarAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	actual, e := multispanNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	p, e := ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewBufferString(actual.Source), Limits{})
	if e != nil {
		return e
	}
	want, e := multispanNURBSNativeClaim(ctx, p)
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

// ReadMultispanNURBSPlanarAssemblyJSON constructs from original source once,
// never from cached geometry or alleged affine identities.
func ReadMultispanNURBSPlanarAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (MultispanNURBSPlanarAssembly, error) {
	var zero MultispanNURBSPlanarAssembly
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
	var q multispanNURBSNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != multispanNURBSNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 || len(q.Surfaces) > 4096 || len(q.Curves) > 4096 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	if !nurbsAssemblyEnvelopeBound(nurbsAssemblyEnvelope{Format: nurbsPlanarAssemblyNativeFormat, Version: 1, Mode: nurbsNativeOblique, OriginalSource: q.Source, Geometry: q.Geometry, Oblique: &nurbsObliqueNativeClaims{Frames: q.Frames, Points: q.Points}}) {
		return zero, geom.ErrInvalid
	}
	m, e := ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := multispanNURBSNativeClaim(ctx, m)
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
