package geom

import (
	"context"
	"math/big"
)

// Finite cylinder support masks are distinct from a blind pocket floor.
const (
	BoreLowerCap BoreBoundaryMask = 1 << (iota + 3)
	BoreUpperCap
)

// OffAxisSphereFiniteCylinderCommon is the immutable closed set |p|²≤R²,
// (x-cx)²+(y-cy)²≤r², lower≤z≤upper. Arbitrary cap placement and partial
// cap-disk overlap are supported. Empty and tangent intersections are valid;
// this query does not certify that the set is a regular three-dimensional solid.
type OffAxisSphereFiniteCylinderCommon struct{ R, r, cx, cy, lower, upper *big.Rat }

func NewOffAxisSphereFiniteCylinderCommon(R, r float64, offset Vec2, lower, upper float64) (OffAxisSphereFiniteCylinderCommon, error) {
	if !finite(R) || !finite(r) || !finite(offset.X) || !finite(offset.Y) || !finite(lower) || !finite(upper) || R <= 0 || r <= 0 || lower >= upper {
		return OffAxisSphereFiniteCylinderCommon{}, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	return OffAxisSphereFiniteCylinderCommon{q(R), q(r), q(offset.X), q(offset.Y), q(lower), q(upper)}, nil
}
func finiteCommonLocation(signs []int) (BorePointLocation, BoreBoundaryMask) {
	for _, v := range signs {
		if v > 0 {
			return BoreOutside, 0
		}
	}
	var mask BoreBoundaryMask
	for i, m := range []BoreBoundaryMask{BoreSphere, BoreCylinder, BoreLowerCap, BoreUpperCap} {
		if signs[i] == 0 {
			mask |= m
		}
	}
	if mask != 0 {
		return BoreBoundary, mask
	}
	return BoreInside, 0
}
func (s OffAxisSphereFiniteCylinderCommon) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	return s.classifyMaterial(p, finiteCommonLocation)
}
func (s OffAxisSphereFiniteCylinderCommon) classifyMaterial(p Vec3, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (BorePointLocation, BoreBoundaryMask, error) {
	if s.R == nil || !p.IsFinite() {
		return BoreOutside, 0, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	x, y, z := q(p.X), q(p.Y), q(p.Z)
	a := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Add(sq(x), sq(y)), sq(z)), sq(s.R))
	b := new(big.Rat).Sub(new(big.Rat).Add(sq(new(big.Rat).Sub(x, s.cx)), sq(new(big.Rat).Sub(y, s.cy))), sq(s.r))
	l, m := classify([]int{a.Sign(), b.Sign(), new(big.Rat).Sub(s.lower, z).Sign(), new(big.Rat).Sub(z, s.upper).Sign()})
	return l, m, nil
}

// IntersectLineBoundary reports isolated exact hits and closed continuous
// support-contact intervals on original t∈[minT,maxT]. Caps and cylindrical
// coincidences are included, with additional support masks at interval ends.
// All root classifications use exact rational signs. Options bound isolation
// and arithmetic; each exact comparison receives the same bounded options.
func (s OffAxisSphereFiniteCylinderCommon) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	return s.intersectMaterial(ctx, origin, direction, minT, maxT, o, finiteCommonLocation)
}
func (s OffAxisSphereFiniteCylinderCommon) intersectMaterial(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (BoreLineBoundary, error) {
	fail := func(e error) (BoreLineBoundary, error) { return BoreLineBoundary{}, e }
	if ctx == nil || s.R == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return fail(ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return fail(ErrInvalid)
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return fail(e)
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	poly := []rationalPolynomial{{new(big.Rat), new(big.Rat), new(big.Rat)}, {new(big.Rat), new(big.Rat), new(big.Rat)}, {new(big.Rat).Sub(s.lower, q(origin.Z)), new(big.Rat).Neg(q(direction.Z))}, {new(big.Rat).Sub(q(origin.Z), s.upper), q(direction.Z)}}
	ov, dv := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{direction.X, direction.Y, direction.Z}
	for support := 0; support < 2; support++ {
		n := 3
		if support == 1 {
			n = 2
		}
		for axis := 0; axis < n; axis++ {
			c := new(big.Rat)
			if support == 1 {
				c = []*big.Rat{s.cx, s.cy}[axis]
			}
			line := rationalPolynomial{new(big.Rat).Sub(q(ov[axis]), c), q(dv[axis])}
			sq, e := polynomialProduct(line, line, budget)
			if e != nil {
				return fail(e)
			}
			for j, v := range sq {
				poly[support][j].Add(poly[support][j], v)
			}
		}
		radius := s.R
		if support == 1 {
			radius = s.r
		}
		poly[support][0].Sub(poly[support][0], new(big.Rat).Mul(radius, radius))
	}
	for i := range poly {
		poly[i] = trimPolynomial(poly[i])
		for _, v := range poly[i] {
			if e := budget.check(v); e != nil {
				return fail(e)
			}
		}
	}
	return intersectMaterialBoundary(ctx, poly, minT, maxT, o, budget, ro, classify)
}

// IntersectLine is the isolated-only variant. Continuous contact is explicitly
// unresolved; use IntersectLineBoundary to retain those intervals.
func (s OffAxisSphereFiniteCylinderCommon) IntersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]BoreLineHit, error) {
	r, e := s.IntersectLineBoundary(ctx, origin, direction, minT, maxT, o)
	if e != nil {
		return nil, e
	}
	if len(r.Intervals) > 0 {
		return nil, ErrConvergence
	}
	return r.Hits, nil
}
