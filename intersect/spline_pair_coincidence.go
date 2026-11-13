package intersect

import (
	"context"
	"errors"

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
	pa, e := exactCoincidencePatch(a, o.Patches)
	if e != nil {
		return r, e
	}
	pb, e := exactCoincidencePatch(b, o.Patches)
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

func exactCoincidencePatch(p Patch, o geom.SurfacePatchOptions) (*geom.SurfacePatch, error) {
	s, ok := p.Surface.(*geom.BSplineSurface)
	if !ok {
		return nil, geom.ErrInvalid
	}
	x, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		return nil, e
	}
	return x.Trim(geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y})
}
