package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SphereDisjointCylinderUnion preserves two independently closed analytic
// bodies, ordered sphere then cylinder. Volume coefficients are exact ideal
// volumes divided by π. Native evaluation retains its usual sewing tolerance.
type SphereDisjointCylinderUnion struct {
	Solids                   [2]Solid
	VolumePiCoefficients     [2]*big.Rat
	TotalVolumePiCoefficient *big.Rat
}

// SphereDisjointFiniteCylinderUnion proves a positive separation exceeding
// eight tolerances using exact squared distance from the sphere center to the
// finite cylinder solid. Radial distance is max(sqrt(dx²+dy²)-r,0), axial
// distance is distance(0,[lower,upper]); neither square root is rounded.
// Tangency and insufficient clearance reject atomically. Coordinates refer
// to a global Z-axis cylinder offset relative to the sphere center.
func SphereDisjointFiniteCylinderUnion(ctx context.Context, center geom.Vec3, R, r float64, offset geom.Vec2, lower, upper, tolerance float64) (SphereDisjointCylinderUnion, error) {
	fail := func(e error) (SphereDisjointCylinderUnion, error) { return SphereDisjointCylinderUnion{}, e }
	if ctx == nil || !center.IsFinite() || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if _, e := geom.NewOffAxisSphereFiniteCylinderCommon(R, r, offset, lower, upper); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	d2 := new(big.Rat).Add(sq(q(offset.X)), sq(q(offset.Y)))
	r2 := sq(q(r))
	axial := new(big.Rat)
	if lower > 0 {
		axial = q(lower)
	} else if upper < 0 {
		axial = new(big.Rat).Neg(q(upper))
	}
	safeR := new(big.Rat).Add(q(R), new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance)))
	excess := new(big.Rat).Sub(sq(axial), sq(safeR))
	separated := excess.Sign() > 0
	if !separated && d2.Cmp(r2) > 0 {
		// (d-r)²+axial²>safeR² iff positive A=d²+r²+
		// axial²-safeR² satisfies A²>4r²d².
		a := new(big.Rat).Add(new(big.Rat).Add(d2, r2), excess)
		rhs := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(r2, d2))
		separated = a.Sign() > 0 && sq(a).Cmp(rhs) > 0
	}
	if !separated {
		return fail(geom.ErrDomain)
	}
	heightRat := new(big.Rat).Sub(q(upper), q(lower))
	height, e := geom.RepresentedExactScalar(heightRat)
	if e != nil {
		return fail(e)
	}
	world := geom.Vec3{}
	coords := []*float64{&world.X, &world.Y, &world.Z}
	for i, xy := range [][2]float64{{center.X, offset.X}, {center.Y, offset.Y}, {center.Z, lower}} {
		v, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(xy[0]), q(xy[1])))
		if e != nil {
			return fail(e)
		}
		*coords[i] = v
	}
	frame, e := geom.NewFrame(world, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	sphere, e := Sphere(center, R)
	if e != nil {
		return fail(e)
	}
	cylinder, e := Cylinder(frame, r, height)
	if e != nil {
		return fail(e)
	}
	// Revalidate both at the explicitly requested tolerance.
	sphere, e = NewSolid(sphere.Vertices(), sphere.Edges(), sphere.Faces(), tolerance)
	if e != nil {
		return fail(e)
	}
	cylinder, e = NewSolid(cylinder.Vertices(), cylinder.Edges(), cylinder.Faces(), tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	a := new(big.Rat).Mul(big.NewRat(4, 3), new(big.Rat).Mul(sq(q(R)), q(R)))
	b := new(big.Rat).Mul(r2, heightRat)
	return SphereDisjointCylinderUnion{Solids: [2]Solid{sphere, cylinder}, VolumePiCoefficients: [2]*big.Rat{a, b}, TotalVolumePiCoefficient: new(big.Rat).Add(a, b)}, nil
}
