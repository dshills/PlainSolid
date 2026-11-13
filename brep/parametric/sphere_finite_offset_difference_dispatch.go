package parametric

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

type SphereFiniteOffsetDifferenceKind uint8

const (
	SphereOffsetUnchanged SphereFiniteOffsetDifferenceKind = iota
	SphereOffsetEmpty
	SphereOffsetCavity
	SphereOffsetNorthBlind
	SphereOffsetSouthBlind
	SphereOffsetThrough
)

// SphereFiniteOffsetDifference is a bounded native dispatch result. An empty
// material has a zero Solid and nonnil zero volume coefficient. The coefficient
// is nil for curved transverse cases whose volume is not rational times π;
// callers may use Solid.MassProperties with its numerical integration contract.
type SphereFiniteOffsetDifference struct {
	Solid                                                          Solid
	Kind                                                           SphereFiniteOffsetDifferenceKind
	SphericalFaces, CylindricalFaces, LowerCapFaces, UpperCapFaces []int
	VolumePiCoefficient                                            *big.Rat
}

// SubtractFiniteOffsetCylinder assembles supported exact-support regimes of an
// axis-aligned sphere minus a finite cylinder offset along positive global X.
// Disjoint, enclosing, cavity, north/south blind and through cases are covered.
// A tool end beyond a sphere pole is required to exceed it by 8 tolerances
// before dispatching to an infinite-end constructor. Partial rim/cap cuts and
// unproved contacts reject rather than silently extending the finite cutter.
func SubtractFiniteOffsetCylinder(ctx context.Context, center geom.Vec3, R, r, offset, lower, upper, tolerance float64) (SphereFiniteOffsetDifference, error) {
	fail := func(e error) (SphereFiniteOffsetDifference, error) { return SphereFiniteOffsetDifference{}, e }
	if ctx == nil || !center.IsFinite() || !finite(offset) || offset < 0 || !finite(tolerance) || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if _, e := geom.NewOffAxisSphereFiniteCylinderCommon(R, r, geom.Vec2{X: offset}, lower, upper); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	// Entire sphere covered: regularization removes any isolated tangent remnants.
	if new(big.Rat).Sub(q(r), q(R)).Cmp(q(offset)) >= 0 && q(lower).Cmp(new(big.Rat).Neg(q(R))) <= 0 && q(upper).Cmp(q(R)) >= 0 {
		return SphereFiniteOffsetDifference{Kind: SphereOffsetEmpty, VolumePiCoefficient: new(big.Rat)}, nil
	}
	disjoint, e := SphereDisjointFiniteCylinderUnion(ctx, center, R, r, geom.Vec2{X: offset}, lower, upper, tolerance)
	if e == nil {
		return SphereFiniteOffsetDifference{Solid: disjoint.Solids[0], Kind: SphereOffsetUnchanged, SphericalFaces: []int{0}, VolumePiCoefficient: new(big.Rat).Set(disjoint.VolumePiCoefficients[0])}, nil
	}
	if !errors.Is(e, geom.ErrDomain) {
		return fail(e)
	}
	cavity, e := SphereEnclosedCylinderCavity(ctx, center, R, r, geom.Vec2{X: offset}, lower, upper, tolerance)
	if e == nil {
		return SphereFiniteOffsetDifference{Solid: cavity.Solid, Kind: SphereOffsetCavity, SphericalFaces: []int{cavity.SphereFace}, CylindricalFaces: []int{cavity.CylinderWall}, LowerCapFaces: []int{cavity.LowerFloor}, UpperCapFaces: []int{cavity.UpperFloor}, VolumePiCoefficient: cavity.VolumePiCoefficient}, nil
	}
	if !errors.Is(e, geom.ErrDomain) {
		return fail(e)
	}
	guard := new(big.Rat).Add(q(R), new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance)))
	north := q(upper).Cmp(guard) > 0
	south := q(lower).Cmp(new(big.Rat).Neg(guard)) < 0
	frame, e := geom.NewFrame(center, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return fail(e)
	}
	if north && south {
		out, e := SphereOffAxisThroughBore(ctx, frame, R, r, offset, tolerance)
		if e != nil {
			return fail(e)
		}
		return SphereFiniteOffsetDifference{Solid: out.Solid, Kind: SphereOffsetThrough, SphericalFaces: append(append([]int(nil), out.LowerSphericalFaces...), out.UpperSphericalFaces...), CylindricalFaces: out.CylindricalFaces}, nil
	}
	if north {
		out, e := SphereOffAxisBlindBore(ctx, frame, R, r, offset, lower, tolerance)
		if e != nil {
			return fail(e)
		}
		return SphereFiniteOffsetDifference{Solid: out.Solid, Kind: SphereOffsetNorthBlind, SphericalFaces: out.SphericalFaces, CylindricalFaces: out.CylindricalFaces, LowerCapFaces: out.FloorFaces}, nil
	}
	if south {
		reverse, e := geom.NewFrame(center, geom.Vec3{X: 1}, geom.Vec3{Z: -1})
		if e != nil {
			return fail(e)
		}
		out, e := SphereOffAxisBlindBore(ctx, reverse, R, r, offset, -upper, tolerance)
		if e != nil {
			return fail(e)
		}
		return SphereFiniteOffsetDifference{Solid: out.Solid, Kind: SphereOffsetSouthBlind, SphericalFaces: out.SphericalFaces, CylindricalFaces: out.CylindricalFaces, UpperCapFaces: out.FloorFaces}, nil
	}
	return fail(geom.ErrDomain)
}
