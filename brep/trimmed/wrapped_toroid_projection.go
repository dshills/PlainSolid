package trimmed

import (
	"context"
	"plainsolid/geom"
)

// ProjectCanonicalCircularToroid certifies exact material location and signed
// squared boundary-distance bounds for an unplaced one-face complete toroid.
// The four straight fundamental p-curves are verified explicitly; arbitrary
// trims, holes, reversed faces, placements and edited seam partitions reject.
// Projection data is algebraic, with explicit nonunique axis/centerline cases.
func (s Solid) ProjectCanonicalCircularToroid(ctx context.Context, q geom.Vec3, o geom.FrustumDistanceOptions) (geom.ToroidProjection, error) {
	empty := geom.ToroidProjection{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if len(s.faces) != 1 {
		return empty, geom.ErrConvergence
	}
	f := s.faces[0]
	w, ok := f.Surface.(*geom.WrappedSurfacePatch)
	if !ok || f.Reversed || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return empty, geom.ErrConvergence
	}
	corners := insetCorners(w.Domain())
	for i, u := range f.Outer {
		if u.Curve != nil || u.UVStart != corners[i] || u.UVEnd != corners[(i+1)%4] {
			return empty, geom.ErrConvergence
		}
	}
	return geom.ProjectWrappedCircularToroid(ctx, w, q, o)
}
