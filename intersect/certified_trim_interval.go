package intersect

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// EncloseSplineUV bounds the exact spline-chart branch between the two
// normalized parameters. At algebraic endpoints it incorporates the full
// certified endpoint enclosures. Planar projections are not supported here.
func (p CertifiedTrim) EncloseSplineUV(ctx context.Context, start, end float64, o geom.SurfaceArcIntervalOptions) (geom.UVBox, bool, error) {
	s := p.segment
	if ctx == nil || p.plane || s.surface == nil || !finite(start) || !finite(end) || start < 0 || end > 1 || start >= end {
		return geom.UVBox{}, false, geom.ErrInvalid
	}
	var lower, upper *big.Rat
	for _, t := range []float64{start, end} {
		for _, b := range []struct{ a, b geom.Vec2 }{{s.ends[0].Parameters.Min, s.ends[1].Parameters.Min}, {s.ends[0].Parameters.Max, s.ends[1].Parameters.Max}} {
			f := exactNormalizedTubeParameter(s.free(b.a), s.free(b.b), t)
			if lower == nil || f.Cmp(lower) < 0 {
				lower = f
			}
			if upper == nil || f.Cmp(upper) > 0 {
				upper = f
			}
		}
	}
	a, b := outwardSplineParameter(lower, true), outwardSplineParameter(upper, false)
	if a == b {
		a, b = math.Nextafter(a, math.Inf(-1)), math.Nextafter(b, math.Inf(1))
	}
	r, e := s.arc.EncloseInterval(ctx, a, b, o)
	return r.Parameters, r.HasBounds, e
}
