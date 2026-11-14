package geom

import (
	"context"
	"math/big"
)

// EccentricCylinderTorusCavityMaterial retains a cylinder at XY=0 and a torus
// centered at (dx,dy,0). Strict exact containment rejects all support contacts.
// Point/line masks keep original torus/wall/lower/upper identities. No coaxial
// distance or mass formula is exposed through this source.
type EccentricCylinderTorusCavityMaterial struct {
	source TorusCylinderMaterial
	offset [2]*big.Rat
}

func NewEccentricCylinderTorusCavityMaterial(R, r, c, lower, upper, dx, dy float64) (EccentricCylinderTorusCavityMaterial, error) {
	var zero EccentricCylinderTorusCavityMaterial
	if !finite(dx) || !finite(dy) {
		return zero, ErrInvalid
	}
	base, e := NewEncompassingTorusCylinderMaterial(R, r, c, lower, upper, TorusCylinderUnion)
	if e != nil {
		return zero, e
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x, y := q(dx), q(dy)
	d2 := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
	gap := base.clearances[0]
	if new(big.Rat).Mul(gap, gap).Cmp(d2) <= 0 {
		return zero, ErrDomain
	}
	return EccentricCylinderTorusCavityMaterial{base.source, [2]*big.Rat{x, y}}, nil
}
func (s EccentricCylinderTorusCavityMaterial) polynomials(ctx context.Context, p, d [3]*big.Rat, b *polynomialBudget) ([]rationalPolynomial, error) {
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, v := range s.offset {
		a.check(v)
	}
	shifted := p
	for i := 0; i < 2; i++ {
		shifted[i] = a.sub(p[i], s.offset[i])
	}
	if a.err != nil {
		return nil, a.err
	}
	original, e := s.source.polynomials(ctx, p, d, b)
	if e != nil {
		return nil, e
	}
	torus, e := s.source.polynomials(ctx, shifted, d, b)
	if e != nil {
		return nil, e
	}
	original[0] = torus[0]
	return original, nil
}
func (s EccentricCylinderTorusCavityMaterial) location(v []int) (BorePointLocation, BoreBoundaryMask) {
	if v[0] < 0 || v[1] > 0 || v[2] > 0 || v[3] > 0 {
		return BoreOutside, 0
	}
	var mask BoreBoundaryMask
	for i, x := range v {
		if x == 0 {
			mask |= BoreBoundaryMask(1 << i)
		}
	}
	if mask != 0 {
		return BoreBoundary, mask
	}
	return BoreInside, 0
}
func (s EccentricCylinderTorusCavityMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
	if ctx == nil || s.source.R == nil {
		return BoreOutside, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return BoreOutside, 0, e
	}
	for _, v := range p {
		if v == nil {
			return BoreOutside, 0, ErrInvalid
		}
	}
	limits, e := patchOptions(o)
	if e != nil {
		return BoreOutside, 0, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	return s.classifyWithBudget(ctx, p, b)
}
func (s EccentricCylinderTorusCavityMaterial) classifyWithBudget(ctx context.Context, p [3]*big.Rat, b *polynomialBudget) (BorePointLocation, BoreBoundaryMask, error) {
	ps, e := s.polynomials(ctx, p, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, b)
	if e != nil {
		return BoreOutside, 0, e
	}
	v := make([]int, len(ps))
	for i := range ps {
		q, e := polynomialValue(ps[i], new(big.Rat), b)
		if e != nil {
			return BoreOutside, 0, e
		}
		v[i] = q.Sign()
	}
	l, m := s.location(v)
	if e := ctx.Err(); e != nil {
		return BoreOutside, 0, e
	}
	return l, m, nil
}

func (s EccentricCylinderTorusCavityMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
	var z TorusCylinderLine
	if ctx == nil || s.source.R == nil || min == nil || max == nil || min.Cmp(max) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return z, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return z, e
	}
	nonzero := false
	for i := range p {
		if p[i] == nil || d[i] == nil {
			return z, ErrInvalid
		}
		nonzero = nonzero || d[i].Sign() != 0
	}
	if !nonzero {
		return z, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return z, ErrInvalid
	}
	limits, e := patchOptions(o.Surface)
	if e != nil {
		return z, e
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	if e = b.check(min); e != nil {
		return z, e
	}
	if e = b.check(max); e != nil {
		return z, e
	}
	ps, e := s.polynomials(ctx, p, d, b)
	if e != nil {
		return z, e
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	out, e := intersectMaterialBoundaryRational(ctx, ps, min, max, o, b, ro, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		l, m := s.location(v)
		if l == BoreInside {
			l = BoreBoundary
		}
		return l, m
	})
	if e != nil {
		return z, e
	}
	if e = ctx.Err(); e != nil {
		return z, e
	}
	return TorusCylinderLine{out.Intervals, out.Hits}, nil
}
