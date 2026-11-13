package intersect

import (
	"context"
	"plainsolid/geom"
)

// IntersectQuadraticTrimProjected proves exact branch/trim contacts in chart 0 (A) or
// 1 (B). The trim is a normalized exact rational quadratic UV Bezier. It uses
// original model supports and the immutable segment tube, never display samples.
// axes selects model projection; returned model events remain in original XYZ.
func (s TubeSegment) IntersectQuadraticTrimProjected(ctx context.Context, chart int, trim *geom.ExactSplineCurve, axes [2]int, o geom.TubeTrimIntersectionOptions) (r geom.ProjectedTubeTrimIntersection, err error) {
	if ctx == nil || s.a == nil || s.b == nil || s.pa == nil || chart < 0 || chart > 1 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, geom.ErrInvalid
	}
	used := 0
	defer func() { r.ExactOperations += used }()
	var supports [2]*geom.SurfacePatch
	for i, surface := range []*geom.BSplineSurface{s.a, s.b} {
		remaining := o.MaxOperations - used
		if remaining < 1 {
			return r, geom.ErrConvergence
		}
		p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		used += p.ExactOperations()
		supports[i] = p
	}
	o.MaxOperations -= used
	if o.MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	return s.proof.IntersectQuadraticTrimProjected(ctx, supports[0], supports[1], chart, trim, axes, o)
}
