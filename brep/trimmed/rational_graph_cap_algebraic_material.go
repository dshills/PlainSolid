//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateMaterial classifies a represented point in the intersection of the
// original certified stock with the two oriented negative halfspaces. Cut
// plane signs are evaluated exactly; the original stock uses its existing
// exact face-membership and certified ray checks.
func (c AlgebraicGraphCapPair) LocateMaterial(ctx context.Context, point geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if ctx == nil || c.caps[0].recipe == nil || c.caps[1].recipe == nil || !point.IsFinite() || o.MaxRays < 0 || o.MaxRays > 12 {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	if o.MaxRays == 0 {
		o.MaxRays = 12
	}
	p := [3]*big.Rat{ratInset(point.X), ratInset(point.Y), ratInset(point.Z)}
	boundary := false
	for _, cap := range c.caps {
		plane := cap.regions.plane
		side := plane.dot(p).Cmp(plane.offset) * plane.orientation
		if side > 0 {
			return parametric.Outside, nil
		}
		boundary = boundary || side == 0
	}
	s := c.caps[0].recipe.stock
	var box geom.Box
	for i, f := range s.faces {
		patch, e := exactFaceSupportPatch(f, o.Embedding.Patches)
		if e != nil {
			return parametric.Outside, e
		}
		uv, e := faceUVBounds(f)
		if e != nil {
			return parametric.Outside, e
		}
		patch, e = patch.Trim(uv)
		if e != nil {
			return parametric.Outside, e
		}
		fb, e := patch.Bounds()
		if e != nil {
			return parametric.Outside, e
		}
		if i == 0 {
			box = fb
		} else {
			box, e = box.Union(fb)
			if e != nil {
				return parametric.Outside, e
			}
		}
	}
	result, e := locateOriginalBooleanOperand(ctx, s, box, point, o)
	if e != nil || result != parametric.Inside || !boundary {
		return result, e
	}
	return parametric.Boundary, nil
}
