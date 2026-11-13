package geom

import (
	"context"
	"math/big"
)

// RawCylinder is a circular finite cylinder with axial model line
// Origin + t*Axis. The axis need not be normalized. Min and Max are the
// original raw axial parameters, rather than model distances.
type RawCylinder struct {
	Origin, Axis     Vec3
	Radius, Min, Max float64
}

const (
	RawCylinderFirstSide BoreBoundaryMask = 1 << iota
	RawCylinderFirstMin
	RawCylinderFirstMax
	RawCylinderSecondSide
	RawCylinderSecondMin
	RawCylinderSecondMax
)

// CylinderPairCommon is an immutable exact material query for any two finite
// circular cylinders. It retains lower-dimensional contacts; the query does
// not assert that their intersection is a regularized solid.
type CylinderPairCommon struct{ s [2]exactRawCylinder }

type exactRawCylinder struct {
	origin, axis             [3]*big.Rat
	norm2, radius2, min, max *big.Rat
}

func NewCylinderPairCommon(a, b RawCylinder) (CylinderPairCommon, error) {
	var out CylinderPairCommon
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for i, c := range [2]RawCylinder{a, b} {
		if !c.Origin.IsFinite() || !c.Axis.IsFinite() || c.Axis == (Vec3{}) || !finite(c.Radius) || !finite(c.Min) || !finite(c.Max) || c.Radius <= 0 || c.Min >= c.Max {
			return CylinderPairCommon{}, ErrInvalid
		}
		s := exactRawCylinder{
			origin: [3]*big.Rat{q(c.Origin.X), q(c.Origin.Y), q(c.Origin.Z)},
			axis:   [3]*big.Rat{q(c.Axis.X), q(c.Axis.Y), q(c.Axis.Z)},
			norm2:  new(big.Rat), radius2: new(big.Rat).Mul(q(c.Radius), q(c.Radius)), min: q(c.Min), max: q(c.Max),
		}
		for _, v := range s.axis {
			s.norm2.Add(s.norm2, new(big.Rat).Mul(v, v))
		}
		out.s[i] = s
	}
	return out, nil
}

func (s CylinderPairCommon) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	if s.s[0].norm2 == nil || !p.IsFinite() {
		return BoreOutside, 0, ErrInvalid
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	vec := [3]*big.Rat{q(p.X), q(p.Y), q(p.Z)}
	signs := make([]int, 0, 6)
	for _, c := range s.s {
		dot, sq := new(big.Rat), new(big.Rat)
		for j := range 3 {
			x := new(big.Rat).Sub(vec[j], c.origin[j])
			dot.Add(dot, new(big.Rat).Mul(x, c.axis[j]))
			sq.Add(sq, new(big.Rat).Mul(x, x))
		}
		distance := new(big.Rat).Sub(new(big.Rat).Mul(c.norm2, new(big.Rat).Sub(sq, c.radius2)), new(big.Rat).Mul(dot, dot))
		signs = append(signs, distance.Sign(), new(big.Rat).Mul(c.min, c.norm2).Cmp(dot), dot.Cmp(new(big.Rat).Mul(c.max, c.norm2)))
	}
	l, m := perpendicularCylinderLocation(signs)
	return l, m, nil
}

// IntersectLineBoundary returns isolated hits and continuous contact intervals.
// Every algebraic endpoint retains all active cylinder supports. Direction
// and the result parameter stay in the caller's unnormalized line chart.
func (s CylinderPairCommon) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	return s.intersectLine(ctx, origin, direction, minT, maxT, o, perpendicularCylinderLocation)
}

// CylinderMaterialLine is the complete filled intersection of a finite line
// with two finite cylinder bodies. Boundary masks identify exposed supports
// along intervals and at endpoints; a zero interior mask denotes solid
// interior. Points contains isolated lower-dimensional contacts.
type CylinderMaterialLine struct {
	Intervals []BoreBoundaryInterval
	Points    []BoreLineHit
}

// ClipLine retains material intervals as well as isolated contacts.
func (s CylinderPairCommon) ClipLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderMaterialLine, error) {
	r, e := s.intersectLine(ctx, origin, direction, minT, maxT, o, func(signs []int) (BorePointLocation, BoreBoundaryMask) {
		l, m := perpendicularCylinderLocation(signs)
		if l == BoreInside {
			l = BoreBoundary
		}
		return l, m
	})
	if e != nil {
		return CylinderMaterialLine{}, e
	}
	return CylinderMaterialLine{Intervals: r.Intervals, Points: r.Hits}, nil
}

func (s CylinderPairCommon) intersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (BoreLineBoundary, error) {
	var empty BoreLineBoundary
	if ctx == nil || s.s[0].norm2 == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	o0, d0 := [3]*big.Rat{q(origin.X), q(origin.Y), q(origin.Z)}, [3]*big.Rat{q(direction.X), q(direction.Y), q(direction.Z)}
	ps := make([]rationalPolynomial, 0, 6)
	for _, c := range s.s {
		dot := rationalPolynomial{new(big.Rat), new(big.Rat)}
		sq := rationalPolynomial{new(big.Rat).Neg(c.radius2), new(big.Rat), new(big.Rat)}
		for j := range 3 {
			a := new(big.Rat).Sub(o0[j], c.origin[j])
			dot[0].Add(dot[0], new(big.Rat).Mul(a, c.axis[j]))
			dot[1].Add(dot[1], new(big.Rat).Mul(d0[j], c.axis[j]))
			v, e := polynomialProduct(rationalPolynomial{a, d0[j]}, rationalPolynomial{a, d0[j]}, b)
			if e != nil {
				return empty, e
			}
			for k, x := range v {
				sq[k].Add(sq[k], x)
			}
		}
		dot2, e := polynomialProduct(dot, dot, b)
		if e != nil {
			return empty, e
		}
		for k := range sq {
			sq[k].Mul(sq[k], c.norm2)
			if k < len(dot2) {
				sq[k].Sub(sq[k], dot2[k])
			}
		}
		ps = append(ps, trimPolynomial(sq),
			trimPolynomial(rationalPolynomial{new(big.Rat).Sub(new(big.Rat).Mul(c.min, c.norm2), dot[0]), new(big.Rat).Neg(dot[1])}),
			trimPolynomial(rationalPolynomial{new(big.Rat).Sub(dot[0], new(big.Rat).Mul(c.max, c.norm2)), dot[1]}))
	}
	for _, p := range ps {
		for _, v := range p {
			if e := b.check(v); e != nil {
				return empty, e
			}
		}
	}
	return intersectMaterialBoundary(ctx, ps, minT, maxT, o, b, ro, classify)
}
