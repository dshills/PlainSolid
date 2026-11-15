package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"reflect"
)

// KnottedRationalNURBSPlanarAssembly authenticates affine rational tensor supports
// original rational affine owner pcurves and continuous straight model splines.
// Each rational support has one original simple internal knot in each axis.
type KnottedRationalNURBSPlanarAssembly struct {
	Geometry ColoredAssembly
	original string
	planar   MultispanNURBSPlanarAssembly
	supports []KnottedRationalSurfaceEvidence
	uv       []RationalAffinePCurveEvidence
	models   []AffineModelSplineEvidence
}

func (m KnottedRationalNURBSPlanarAssembly) OriginalSTEP() string { return m.original }
func (m KnottedRationalNURBSPlanarAssembly) RationalSupports() []KnottedRationalSurfaceEvidence {
	return append([]KnottedRationalSurfaceEvidence(nil), m.supports...)
}
func (m KnottedRationalNURBSPlanarAssembly) PCurves() []RationalAffinePCurveEvidence {
	return append([]RationalAffinePCurveEvidence(nil), m.uv...)
}

func ReadKnottedRationalNURBSPlanarAssembly(ctx context.Context, r io.Reader, l Limits) (KnottedRationalNURBSPlanarAssembly, error) {
	var zero KnottedRationalNURBSPlanarAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 4096)
	l.MaxValues = min(l.MaxValues, 65536)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	models, e := affineModelSplineView(ctx, f)
	if e != nil {
		return zero, e
	}
	supports, e := knottedRationalAffineSurfaceView(ctx, f)
	if e != nil {
		return zero, e
	}
	uv, e := rationalAffinePCurveView(ctx, f)
	if e != nil {
		return zero, e
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return zero, e
	}
	p, e := ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewReader(b), l)
	if e != nil {
		return zero, e
	}
	original, e := ReadAssembly(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	p.Geometry.Assembly = original
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return KnottedRationalNURBSPlanarAssembly{p.Geometry, string(raw), p, supports, uv, models}, nil
}
func WriteKnottedRationalNURBSPlanarAssemblySource(ctx context.Context, w io.Writer, m KnottedRationalNURBSPlanarAssembly, l Limits) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	p, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewBufferString(m.original), l)
	if e != nil {
		return e
	}
	a, e := knottedRationalNURBSNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	b, e := knottedRationalNURBSNativeClaim(ctx, p)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(a, b) {
		return geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := io.WriteString(w, m.original)
	if e == nil && n != len(m.original) {
		e = io.ErrShortWrite
	}
	return e
}

func (m KnottedRationalNURBSPlanarAssembly) ModelCurves() []AffineModelSplineEvidence {
	return append([]AffineModelSplineEvidence(nil), m.models...)
}
