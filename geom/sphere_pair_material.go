package geom

import (
	"context"
	"math/big"
)

// SphereSetOperation selects regularized material (closure of set interior).
type SphereSetOperation int

const (
	SphereSetUnion SphereSetOperation = iota
	SphereSetIntersection
	SphereSetDifference
)
const (
	SpherePairFirst BoreBoundaryMask = 1 << iota
	SpherePairSecond
)

// SpherePairMaterial retains exact represented centers/radii privately. It is
// a material predicate, not a manifold B-rep certificate. External touching
// unions and internally tangent cavities can have singular boundary points.
// Regularized external tangent intersections and fully removed differences
// are empty; no lower-dimensional remnants are presented as material.
type SpherePairMaterial struct {
	center  [2][3]*big.Rat
	radius2 [2]*big.Rat
	op      SphereSetOperation
	mode    int
}

// modes: 0 ordinary,1 first only,2 second only,3 empty,4 identical supports.
// NewSpherePairMaterial proves exact separation/containment modes before
// exposing an immutable regularized Union, Intersection, or A-minus-B query.
func NewSpherePairMaterial(a Vec3, ra float64, b Vec3, rb float64, op SphereSetOperation) (SpherePairMaterial, error) {
	var s SpherePairMaterial
	if !a.IsFinite() || !b.IsFinite() || !finite(ra) || !finite(rb) || ra <= 0 || rb <= 0 || op < SphereSetUnion || op > SphereSetDifference {
		return s, ErrInvalid
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	s.op = op
	s.center = [2][3]*big.Rat{{q(a.X), q(a.Y), q(a.Z)}, {q(b.X), q(b.Y), q(b.Z)}}
	s.radius2 = [2]*big.Rat{new(big.Rat).Mul(q(ra), q(ra)), new(big.Rat).Mul(q(rb), q(rb))}
	d2 := new(big.Rat)
	for i := 0; i < 3; i++ {
		v := new(big.Rat).Sub(s.center[0][i], s.center[1][i])
		d2.Add(d2, new(big.Rat).Mul(v, v))
	}
	sum := new(big.Rat).Add(q(ra), q(rb))
	diff := new(big.Rat).Sub(q(ra), q(rb))
	separate := d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0
	contained := d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0
	if d2.Sign() == 0 && ra == rb {
		s.mode = 4
		if op == SphereSetDifference {
			s.mode = 3
		}
		return s, nil
	}
	switch op {
	case SphereSetUnion:
		if contained {
			if ra > rb {
				s.mode = 1
			} else {
				s.mode = 2
			}
		}
	case SphereSetIntersection:
		if separate {
			s.mode = 3
		} else if contained {
			if ra < rb {
				s.mode = 1
			} else {
				s.mode = 2
			}
		}
	case SphereSetDifference:
		if contained && rb > ra {
			s.mode = 3
		} else if separate {
			s.mode = 1
		}
	}
	return s, nil
}
func (s SpherePairMaterial) location(signs []int) (BorePointLocation, BoreBoundaryMask) {
	if s.mode == 3 {
		return BoreOutside, 0
	}
	single := func(i int) (BorePointLocation, BoreBoundaryMask) {
		if signs[i] > 0 {
			return BoreOutside, 0
		}
		if signs[i] == 0 {
			return BoreBoundary, 1 << uint(i)
		}
		return BoreInside, 0
	}
	if s.mode == 1 {
		return single(0)
	}
	if s.mode == 2 {
		return single(1)
	}
	if s.mode == 4 {
		l, m := single(0)
		if m != 0 {
			m = SpherePairFirst | SpherePairSecond
		}
		return l, m
	}
	a, b := signs[0], signs[1]
	var mask BoreBoundaryMask
	if a == 0 {
		mask |= SpherePairFirst
	}
	if b == 0 {
		mask |= SpherePairSecond
	}
	switch s.op {
	case SphereSetUnion:
		if a < 0 || b < 0 {
			return BoreInside, 0
		}
		if a > 0 && b > 0 {
			return BoreOutside, 0
		}
	case SphereSetIntersection:
		if a > 0 || b > 0 {
			return BoreOutside, 0
		}
		if a < 0 && b < 0 {
			return BoreInside, 0
		}
	case SphereSetDifference:
		if a > 0 || b < 0 {
			return BoreOutside, 0
		}
		if a < 0 && b > 0 {
			return BoreInside, 0
		}
	}
	return BoreBoundary, mask
}

// Classify uses exact represented model coordinates without tolerance tests.
func (s SpherePairMaterial) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	if s.radius2[0] == nil || !p.IsFinite() {
		return BoreOutside, 0, ErrInvalid
	}
	v := [3]float64{p.X, p.Y, p.Z}
	signs := make([]int, 2)
	for i := 0; i < 2; i++ {
		r := new(big.Rat).Neg(s.radius2[i])
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(new(big.Rat).SetFloat64(v[j]), s.center[i][j])
			r.Add(r, new(big.Rat).Mul(d, d))
		}
		signs[i] = r.Sign()
	}
	l, m := s.location(signs)
	return l, m, nil
}

// SpherePairMaterialLine retains exact original raw line parameters. Zero
// interval Boundary denotes interior, while endpoint masks identify exposed
// original sphere supports. Points are isolated within the finite line range.
type SpherePairMaterialLine struct {
	Intervals []BoreBoundaryInterval
	Points    []BoreLineHit
}

// ClipLine partitions the finite line by exact quadratic roots, including
// tangencies. ParameterTolerance affects brackets only. Isolation/sign work is
// bounded; root comparisons receive the supplied bounded options. Failure is
// atomic. Neither a touching set nor a line contact implies manifold topology.
func (s SpherePairMaterial) ClipLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (SpherePairMaterialLine, error) {
	var zero SpherePairMaterialLine
	if ctx == nil || s.radius2[0] == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return zero, ErrInvalid
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	org := [3]float64{origin.X, origin.Y, origin.Z}
	dir := [3]float64{direction.X, direction.Y, direction.Z}
	ps := make([]rationalPolynomial, 2)
	for i := 0; i < 2; i++ {
		p := rationalPolynomial{new(big.Rat).Neg(s.radius2[i]), new(big.Rat), new(big.Rat)}
		for j := 0; j < 3; j++ {
			v := new(big.Rat).Sub(new(big.Rat).SetFloat64(org[j]), s.center[i][j])
			d := new(big.Rat).SetFloat64(dir[j])
			product, e := polynomialProduct(rationalPolynomial{v, d}, rationalPolynomial{v, d}, budget)
			if e != nil {
				return zero, e
			}
			for k, c := range product {
				p[k].Add(p[k], c)
			}
		}
		for _, v := range p {
			if e := budget.check(v); e != nil {
				return zero, e
			}
		}
		ps[i] = trimPolynomial(p)
	}
	r, e := intersectMaterialBoundary(ctx, ps, minT, maxT, o, budget, ro, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		l, m := s.location(v)
		if l == BoreInside {
			l = BoreBoundary
		}
		return l, m
	})
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return SpherePairMaterialLine{Intervals: r.Intervals, Points: r.Hits}, nil
}
