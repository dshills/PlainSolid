package geom

import (
	"context"
	"math/big"
)

// PerpendicularCylinderTorusCavityMaterial has original cylinder axis Z and
// independent torus axis X centered at center. A conservative sphere envelope
// proves radial containment; exact torus axial extent is centerZ±(R+r).
// No acceptance uses rounded support geometry. All contacts reject.
type PerpendicularCylinderTorusCavityMaterial struct {
	source TorusCylinderMaterial
	center [3]*big.Rat
}

func NewPerpendicularCylinderTorusCavityMaterial(R, r, c, lower, upper float64, center Vec3) (PerpendicularCylinderTorusCavityMaterial, error) {
	var zero PerpendicularCylinderTorusCavityMaterial
	for _, v := range []float64{R, r, c, lower, upper, center.X, center.Y, center.Z} {
		if !finite(v) {
			return zero, ErrInvalid
		}
	}
	if R <= r || r <= 0 || c <= 0 {
		return zero, ErrInvalid
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	extent := new(big.Rat).Add(q(R), q(r))
	gap := new(big.Rat).Sub(q(c), extent)
	d2 := new(big.Rat).Add(new(big.Rat).Mul(q(center.X), q(center.X)), new(big.Rat).Mul(q(center.Y), q(center.Y)))
	if gap.Sign() <= 0 || new(big.Rat).Mul(gap, gap).Cmp(d2) <= 0 || new(big.Rat).Sub(q(center.Z), extent).Cmp(q(lower)) <= 0 || new(big.Rat).Add(q(center.Z), extent).Cmp(q(upper)) >= 0 {
		return zero, ErrDomain
	}
	return PerpendicularCylinderTorusCavityMaterial{TorusCylinderMaterial{q(R), q(r), q(c), q(lower), q(upper), TorusCylinderUnion}, [3]*big.Rat{q(center.X), q(center.Y), q(center.Z)}}, nil
}
func (s PerpendicularCylinderTorusCavityMaterial) polynomials(ctx context.Context, p, d [3]*big.Rat, b *polynomialBudget) ([]rationalPolynomial, error) {
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	var local, dir [3]*big.Rat
	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		a.check(s.center[j])
		local[i] = a.sub(p[j], s.center[j])
		dir[i] = d[j]
	}
	if a.err != nil {
		return nil, a.err
	}
	original, e := s.source.polynomials(ctx, p, d, b)
	if e != nil {
		return nil, e
	}
	torus, e := s.source.polynomials(ctx, local, dir, b)
	if e != nil {
		return nil, e
	}
	original[0] = torus[0]
	return original, nil
}
func (s PerpendicularCylinderTorusCavityMaterial) location(v []int) (BorePointLocation, BoreBoundaryMask) {
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
func (s PerpendicularCylinderTorusCavityMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
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
func (s PerpendicularCylinderTorusCavityMaterial) classifyWithBudget(ctx context.Context, p [3]*big.Rat, b *polynomialBudget) (BorePointLocation, BoreBoundaryMask, error) {
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

func (s PerpendicularCylinderTorusCavityMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
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
