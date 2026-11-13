package geom

import (
	"context"
	"math/big"
)

// SpherePairPlaneMaterial is an immutable exact chart view of the original
// three-dimensional regularized set. It does not re-regularize planar slices.
// In particular isolated poles can retain boundary classification.
type SpherePairPlaneMaterial struct {
	source SpherePairMaterial
	origin [3]*big.Rat
	basis  [2][3]*big.Rat
	valid  bool
}

// PlaneMaterial validates a raw plane and retains its rational Gram chart.
// Coordinates agree with PlaneSupports. No rounded frame is constructed.
func (s SpherePairMaterial) PlaneMaterial(ctx context.Context, origin, normal Vec3, o SurfacePatchEvaluationOptions) (SpherePairPlaneMaterial, error) {
	p, e := s.PlaneSupports(ctx, origin, normal, o)
	if e != nil {
		return SpherePairPlaneMaterial{}, e
	}
	return SpherePairPlaneMaterial{source: s, origin: p.Origin, basis: p.Basis, valid: true}, nil
}

// ClassifyRationalUV embeds caller-owned rational UV exactly and applies the
// original global material classifier. It returns only exposed source masks,
// not all coincident support incidences. Failure returns Outside with mask zero.
func (s SpherePairPlaneMaterial) ClassifyRationalUV(ctx context.Context, uv [2]*big.Rat, o SurfacePatchEvaluationOptions) (BorePointLocation, BoreBoundaryMask, error) {
	fail := func(e error) (BorePointLocation, BoreBoundaryMask, error) { return BoreOutside, 0, e }
	if ctx == nil || !s.valid || uv[0] == nil || uv[1] == nil {
		return fail(ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return fail(ErrInvalid)
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	for _, q := range uv {
		if e := b.check(q); e != nil {
			return fail(e)
		}
	}
	var xyz [3]*big.Rat
	for j := 0; j < 3; j++ {
		xyz[j] = new(big.Rat).Set(s.origin[j])
		if e := b.check(xyz[j]); e != nil {
			return fail(e)
		}
		for k := 0; k < 2; k++ {
			term := new(big.Rat).Mul(s.basis[k][j], uv[k])
			if e := b.check(term); e != nil {
				return fail(e)
			}
			xyz[j].Add(xyz[j], term)
			if e := b.check(xyz[j]); e != nil {
				return fail(e)
			}
		}
	}
	signs := make([]int, 2)
	for i := 0; i < 2; i++ {
		residual := new(big.Rat).Neg(s.source.radius2[i])
		if e := b.check(residual); e != nil {
			return fail(e)
		}
		for j := 0; j < 3; j++ {
			d := new(big.Rat).Sub(xyz[j], s.source.center[i][j])
			if e := b.check(d); e != nil {
				return fail(e)
			}
			d.Mul(d, d)
			if e := b.check(d); e != nil {
				return fail(e)
			}
			residual.Add(residual, d)
			if e := b.check(residual); e != nil {
				return fail(e)
			}
		}
		signs[i] = residual.Sign()
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	loc, mask := s.source.location(signs)
	return loc, mask, nil
}
