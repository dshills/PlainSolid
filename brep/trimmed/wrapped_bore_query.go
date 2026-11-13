package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateWrappedCylinderBore classifies material of a canonical concentric
// subtraction, including through-bores, blind bores and closed cavities.
// The input profiles and height containment are independently proved.
func LocateWrappedCylinderBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, point geom.Vec3,
	o geom.SurfacePatchOptions) (parametric.Location, error) {
	return locateWrappedCylinderBore(ctx, outer, inner, point, o, false)
}

func locateWrappedCylinderBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, point geom.Vec3,
	o geom.SurfacePatchOptions, through bool) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	a, e := geom.CertifyWrappedCylinderEmbedding(ctx, outer, o)
	if e != nil {
		return parametric.Outside, e
	}
	b, e := geom.CertifyWrappedCylinderEmbedding(ctx, inner, o)
	if e != nil {
		return parametric.Outside, e
	}
	if !sameCylinderAxis(a, b) || a.Radius.Cmp(b.Radius) <= 0 || a.LowerZ.Cmp(b.LowerZ) > 0 ||
		a.UpperZ.Cmp(b.UpperZ) < 0 || through && !sameCylinderAxisAndHeight(a, b) {
		return parametric.Outside, geom.ErrConvergence
	}
	equal, e := sameNormalizedCylinderProfile(ctx, outer, inner, a, b, o)
	if e != nil || !equal {
		if e == nil {
			e = geom.ErrConvergence
		}
		return parametric.Outside, e
	}
	x, e := geom.LocateWrappedCylinder(ctx, outer, point, o)
	if e != nil || x == geom.CylinderOutside {
		return parametric.Outside, e
	}
	z := ratInset(point.Z)
	y, e := geom.LocateWrappedCylinder(ctx, inner, point, o)
	if e != nil || y == geom.CylinderInside {
		return parametric.Outside, e
	}
	commonCap := z.Cmp(a.LowerZ) == 0 && z.Cmp(b.LowerZ) == 0 ||
		z.Cmp(a.UpperZ) == 0 && z.Cmp(b.UpperZ) == 0
	if commonCap && y == geom.CylinderBoundary {
		// Within a common cap disk, the bore interior is exterior to
		// the regularized difference. Reuse the exact XY predicate at
		// the interior of a rational slab to distinguish its rim.
		c, e := geom.ReheightWrappedCylinder(ctx, inner, big.NewRat(-1, 1), big.NewRat(1, 1), o)
		if e != nil {
			return parametric.Outside, e
		}
		point.Z = 0
		y, e = geom.LocateWrappedCylinder(ctx, c, point, o)
		if e != nil || y == geom.CylinderInside {
			return parametric.Outside, e
		}
	}
	if x == geom.CylinderBoundary || y == geom.CylinderBoundary {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}
