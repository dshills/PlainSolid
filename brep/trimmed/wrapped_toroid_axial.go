package trimmed

import (
	"context"
	"plainsolid/geom"
)

// IntersectCanonicalToroidAxial returns exact rational Z-root brackets for a
// full, unplaced, one-face canonical circular-profile wrapped toroid boundary.
// The face must retain its four full-domain straight p-curves; clipped faces,
// placements and edited seam partitions fail explicitly. This intentionally
// narrow adapter does not infer that arbitrary trimmed wrapped faces constitute
// the complete toroid. Root multiplicity and budgets follow the geom query.
func (s Solid) IntersectCanonicalToroidAxial(ctx context.Context, x, y float64, o geom.ToroidAxialOptions) ([]geom.ToroidAxialRoot, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(s.faces) != 1 {
		return nil, geom.ErrConvergence
	}
	f := s.faces[0]
	w, ok := f.Surface.(*geom.WrappedSurfacePatch)
	if !ok || f.Reversed || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return nil, geom.ErrConvergence
	}
	corners := insetCorners(w.Domain())
	for i, u := range f.Outer {
		if u.Curve != nil || u.UVStart != corners[i] || u.UVEnd != corners[(i+1)%4] {
			return nil, geom.ErrConvergence
		}
	}
	return geom.IntersectWrappedCircularToroidAxial(ctx, w, x, y, o)
}
