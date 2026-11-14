package geom

import (
	"context"
	"math/big"
)

// CylinderTorusCavityMaterial is a finite cylinder minus a strictly internal
// ring torus. All four original supports remain; torus orientation is inward.
// Line proof budgets and atomic failure match TorusCylinderMaterial.
type CylinderTorusCavityMaterial struct{ source TorusCylinderMaterial }

func NewCylinderTorusCavityMaterial(R, r, c, lower, upper float64) (CylinderTorusCavityMaterial, error) {
	q, e := NewEncompassingTorusCylinderMaterial(R, r, c, lower, upper, TorusCylinderUnion)
	if e != nil {
		return CylinderTorusCavityMaterial{}, e
	}
	return CylinderTorusCavityMaterial{q.source}, nil
}
func (s CylinderTorusCavityMaterial) location(v []int) (BorePointLocation, BoreBoundaryMask) {
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
func (s CylinderTorusCavityMaterial) ClassifyRational(ctx context.Context, p [3]*big.Rat, o SurfacePatchOptions) (BorePointLocation, BoreBoundaryMask, error) {
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
func (s CylinderTorusCavityMaterial) classifyWithBudget(ctx context.Context, p [3]*big.Rat, b *polynomialBudget) (BorePointLocation, BoreBoundaryMask, error) {
	ps, e := s.source.polynomials(ctx, p, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, b)
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

func (s CylinderTorusCavityMaterial) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o ToroidLineOptions) (TorusCylinderLine, error) {
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
	ps, e := s.source.polynomials(ctx, p, d, b)
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
