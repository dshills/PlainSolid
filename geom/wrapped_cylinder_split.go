package geom

import (
	"context"
	"math/big"
)

// SplitWrappedCylinderAtZ partitions the lateral support of the canonical
// rational extrusion at the exactly represented model height z. Neither
// original U parameterization nor rational coefficients are rounded.
// The two pieces use V=[0,1], preserving the original circumferential
// parameters and seam continuity order. The cut must be strictly interior.
func SplitWrappedCylinderAtZ(ctx context.Context, w *WrappedSurfacePatch, z float64, o SurfacePatchOptions) (lower, upper *WrappedSurfacePatch, err error) {
	if ctx == nil || !finite(z) {
		return nil, nil, ErrInvalid
	}
	q, e := CertifyWrappedCylinderEmbedding(ctx, w, o)
	if e != nil {
		return nil, nil, e
	}
	o, _ = patchOptions(o)
	level := new(big.Rat).SetFloat64(z)
	if level.Cmp(q.LowerZ) <= 0 || level.Cmp(q.UpperZ) >= 0 {
		return nil, nil, ErrDomain
	}
	b := &polynomialBudget{operations: q.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	a, e := reheightWrappedCylinder(ctx, w, [2]*big.Rat{q.LowerZ, level}, o, b)
	if e != nil {
		return nil, nil, e
	}
	c, e := reheightWrappedCylinder(ctx, w, [2]*big.Rat{level, q.UpperZ}, o, b)
	if e != nil {
		return nil, nil, e
	}
	return a, c, nil
}

// ReheightWrappedCylinder preserves the exact rational XY profile and its
// U parameterization while replacing the axial interval. The new heights
// are read-only rationals, and the resulting V domain is [0,1].
func ReheightWrappedCylinder(ctx context.Context, w *WrappedSurfacePatch, lower, upper *big.Rat, o SurfacePatchOptions) (*WrappedSurfacePatch, error) {
	if lower == nil || upper == nil || lower.Cmp(upper) >= 0 {
		return nil, ErrDomain
	}
	q, e := CertifyWrappedCylinderEmbedding(ctx, w, o)
	if e != nil {
		return nil, e
	}
	o, _ = patchOptions(o)
	b := &polynomialBudget{operations: q.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	return reheightWrappedCylinder(ctx, w, [2]*big.Rat{lower, upper}, o, b)
}

func reheightWrappedCylinder(ctx context.Context, w *WrappedSurfacePatch, heights [2]*big.Rat, o SurfacePatchOptions, b *polynomialBudget) (*WrappedSurfacePatch, error) {
	p := *w.patch
	p.options = o
	p.knotsV = []float64{0, 0, 1, 1}
	p.net = append([]exactHomogeneous(nil), p.net...)
	for i := range p.net {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		p.net[i][2] = new(big.Rat).Mul(heights[i%2], p.net[i][3])
		if e := b.check(p.net[i][2]); e != nil {
			return nil, e
		}
	}
	if e := p.computeBounds(b); e != nil {
		return nil, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	p.operations = b.operations
	result, seam, e := NewWrappedSurfacePatch(ctx, &p, WrappedSurfaceOptions{
		WrapU: true, ContinuityOrder: w.options.ContinuityOrder,
		MaxOperations: remaining, MaxPoles: o.MaxPoles, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	b.operations += seam.ExactOperations
	return result, e
}
