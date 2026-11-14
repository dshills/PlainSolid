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

// LocateWrappedCircularFrustumBore locates a represented world point in the
// regularized material defined by two contained circular frustums. It uses
// exact squared radial comparisons. At a common cap plane the inner disk
// interior is open to the exterior; its rim belongs to the annular boundary.
func LocateWrappedCircularFrustumBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, point geom.Vec3,
	o geom.SurfacePatchOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	a, e := geom.CertifyWrappedFrustumEmbedding(ctx, outer, o)
	if e != nil {
		return parametric.Outside, e
	}
	b, e := geom.CertifyWrappedFrustumEmbedding(ctx, inner, o)
	if e != nil {
		return parametric.Outside, e
	}
	if !strictWrappedCircularFrustumBore(a, b) {
		return parametric.Outside, geom.ErrConvergence
	}
	z := ratInset(point.Z)
	u := new(big.Rat).Sub(ratInset(point.X), a.CenterX)
	v := new(big.Rat).Sub(ratInset(point.Y), a.CenterY)
	r2 := new(big.Rat).Add(new(big.Rat).Mul(u, u), new(big.Rat).Mul(v, v))
	radialSign := func(p geom.WrappedFrustumEmbedding) int {
		f := new(big.Rat).Quo(new(big.Rat).Sub(z, p.LowerZ), new(big.Rat).Sub(p.UpperZ, p.LowerZ))
		r := new(big.Rat).Add(p.LowerRadius, new(big.Rat).Mul(f, new(big.Rat).Sub(p.UpperRadius, p.LowerRadius)))
		return r2.Cmp(r.Mul(r, r))
	}
	az, bz := z.Cmp(a.LowerZ), z.Cmp(a.UpperZ)
	if az < 0 || bz > 0 {
		return parametric.Outside, nil
	}
	r := radialSign(a)
	if r > 0 {
		return parametric.Outside, nil
	}
	c, d := z.Cmp(b.LowerZ), z.Cmp(b.UpperZ)
	if c >= 0 && d <= 0 {
		q := radialSign(b)
		if q < 0 && (c != 0 && d != 0 || c == 0 && az == 0 || d == 0 && bz == 0) {
			return parametric.Outside, nil
		}
		if q <= 0 { // inward wall, rim, or retained floor
			return parametric.Boundary, nil
		}
	}
	if r == 0 || az == 0 || bz == 0 {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
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
