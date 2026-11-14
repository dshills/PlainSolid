package intersect

import (
	"context"
	"errors"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineCoincidenceOptions bounds a search over affine rescaling, transposition
// and reversal of two rectangular spline charts. MaxOperations is cumulative
// across the eight comparisons (default 2M). MaxCells and MaxCoefficientBits
// use the defaults of geom.CompareExactSurfaces when zero.
type SplineCoincidenceOptions struct {
	Patches                                     geom.SurfacePatchOptions
	MaxOperations, MaxCells, MaxCoefficientBits int
}

// SplineCoincidence is a proof of equality under Map when Found is true. A
// complete negative result rejects only these eight maps. Neither result
// proves chart injectivity or decides partial geometric overlap.
type SplineCoincidence struct {
	Map                        SplineChartMap
	Bounds                     geom.Box
	Found, Complete            bool
	Attempted, ExactOperations int
}

// FindSplineCoincidence looks for an exact chart map between the two supplied
// spline rectangles. Identity of rational functions is proved on a common
// knot grid; no positional tolerance is used. It returns the first proof in
// deterministic order even if a degenerate chart admits multiple maps.
func FindSplineCoincidence(ctx context.Context, a, b Patch, o SplineCoincidenceOptions) (r SplineCoincidence, err error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	pa, e := exactSplinePatch(a, o.Patches)
	if e != nil {
		return r, e
	}
	pb, e := exactSplinePatch(b, o.Patches)
	if e != nil {
		return r, e
	}
	return FindExactSplineCoincidence(ctx, pa, pb, o)
}

// FindExactSplineCoincidence compares exact spline descriptors, including
// already transformed or restricted geometry. The Patches option does not
// apply because the caller supplies the exact descriptors.
func FindExactSplineCoincidence(ctx context.Context, pa, pb *geom.SurfacePatch, o SplineCoincidenceOptions) (r SplineCoincidence, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if ctx == nil || o.MaxOperations < 1 || o.MaxCells < 0 || o.MaxCoefficientBits != 0 && o.MaxCoefficientBits < 64 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	ba, e := pa.Bounds()
	if e != nil {
		return r, e
	}
	bb, e := pb.Bounds()
	if e != nil {
		return r, e
	}
	bounds, overlap := ba.Intersect(bb)
	if !overlap {
		r.Complete = true
		return r, nil
	}
	limited := false
	for i := range 8 {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return r, geom.ErrConvergence
		}
		m := SplineChartMap{DomainA: pa.Domain(), DomainB: pb.Domain(),
			Transposed: i&4 != 0, ReverseU: i&1 != 0, ReverseV: i&2 != 0}
		// The comparator reverses B after reordering its axes into A's
		// coordinate order. The public map reverses the coordinates in B.
		reverseU, reverseV := m.ReverseU, m.ReverseV
		if m.Transposed {
			reverseU, reverseV = reverseV, reverseU
		}
		q, e := geom.CompareExactSurfaces(ctx, pa, pb, geom.ExactSurfaceComparisonOptions{
			NormalizeDomains: true, SwapB: m.Transposed, ReverseBU: reverseU, ReverseBV: reverseV,
			MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		r.Attempted++
		r.ExactOperations += q.ExactOperations
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) {
				limited = true
				continue
			}
			return r, e
		}
		if q.Equal && q.Complete {
			r.Found, r.Complete, r.Map, r.Bounds = true, true, m, bounds
			return r, nil
		}
	}
	if limited {
		return r, geom.ErrConvergence
	}
	r.Complete = true
	return r, nil
}

func exactSplinePatch(p Patch, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	x, e := exactSplineSupport(p.Surface, o)
	if e != nil {
		return nil, e
	}
	return x.Trim(geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y})
}

func exactSplineSupport(surface geom.Surface, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	base, placement := parametric.SurfaceMapping(surface)
	var patch *geom.SurfacePatch
	var e error
	switch s := base.(type) {
	case *geom.BSplineSurface:
		patch, e = geom.NewSurfacePatch(s, o)
	case *geom.ExactBezierSurface:
		if s == nil || s.ExactPatch() == nil {
			return nil, geom.ErrInvalid
		}
		patch, e = s.ExactPatch().WithLimits(o)
	default:
		return nil, geom.ErrInvalid
	}
	if e != nil {
		return nil, e
	}
	return patch.Transformed(placement)
}

func permuteSplineSupport(surface geom.Surface, order [3]int, o geom.SurfacePatchOptions) (geom.Surface, error) {
	base, placement := parametric.SurfaceMapping(surface)
	if placement != geom.Identity() {
		var m geom.Affine
		var seen [3]bool
		for i, axis := range order {
			if axis < 0 || axis > 2 || seen[axis] {
				return nil, geom.ErrInvalid
			}
			seen[axis], m[i][axis] = true, 1
		}
		// Permute placement rows rather than reconstructing or rounding
		// the original multi-span spline. The world-space seed proof
		// still uses the original support and original placement.
		return parametric.MapSurface(base, m.Mul(placement)), nil
	}
	if s, ok := surface.(*geom.BSplineSurface); ok {
		return s.PermutedModelAxes(order)
	}
	var m geom.Affine
	var seen [3]bool
	for i, axis := range order {
		if axis < 0 || axis > 2 || seen[axis] {
			return nil, geom.ErrInvalid
		}
		seen[axis], m[i][axis] = true, 1
	}
	p, e := exactSplineSupport(surface, o)
	if e != nil {
		return nil, e
	}
	p, e = p.Transformed(m)
	if e != nil {
		return nil, e
	}
	return geom.NewExactBezierSurface(p)
}
