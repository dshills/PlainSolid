package geom

import (
	"context"
	"math/big"
)

// RationalCurveSurfaceHit proves one proposed contact, not completeness or
// uniqueness in the supplied box. Regularity is withheld at interior knots:
// one-sided derivative evaluation alone cannot prove a smooth intersection.
type RationalCurveSurfaceHit struct {
	Contact, Regular bool
	Direction        int
	Parameters       [3]*big.Rat // original curve t, surface u, surface v
	Point            [3]*big.Rat
	Bounds           Box
	ExactOperations  int
}

// CertifyCurveSurfaceRationalHit revalidates a rational proposal against both
// original supports and the closed hit enclosure. It does not trust the hit's
// reported Direction, Bounds, or provenance. Contact=false with nil error proves
// only that this proposal is not a contact. A nonzero Direction has the sign of
// (Su cross Sv) dot C', and certifies local transversality at a smooth point.
func CertifyCurveSurfaceRationalHit(ctx context.Context, curve *ExactSplineCurve, surface *SurfacePatch, hit CurveSurfaceHit, t, u, v *big.Rat, o SurfacePatchEvaluationOptions) (r RationalCurveSurfaceHit, err error) {
	if ctx == nil || curve == nil || curve.patch == nil || surface == nil || t == nil || u == nil || v == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	inside := func(q *big.Rat, lo, hi float64) bool {
		return finite(lo) && finite(hi) && lo <= hi && q.Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && q.Cmp(new(big.Rat).SetFloat64(hi)) <= 0
	}
	for _, q := range []*big.Rat{t, u, v} {
		if e := b.check(q); e != nil {
			return r, e
		}
	}
	if !inside(t, hit.CurveParameter.Min, hit.CurveParameter.Max) || !inside(t, hit.CurveCell.Start, hit.CurveCell.End) || !inside(u, hit.UV.Min.X, hit.UV.Max.X) || !inside(v, hit.UV.Min.Y, hit.UV.Max.Y) || !inside(u, hit.SurfaceCell.UMin, hit.SurfaceCell.UMax) || !inside(v, hit.SurfaceCell.VMin, hit.SurfaceCell.VMax) {
		return r, ErrDomain
	}
	evaluate := func(p *SurfacePatch, a, c *big.Rat) (RationalSurfacePatchJet, error) {
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return RationalSurfacePatchJet{}, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, a, c, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		return jet, e
	}
	cp, e := evaluate(curve.patch, t, new(big.Rat).SetFloat64(curve.patch.Domain().VMin))
	if e != nil {
		return r, e
	}
	sp, e := evaluate(surface, u, v)
	if e != nil {
		return r, e
	}
	for i, q := range []*big.Rat{t, u, v} {
		r.Parameters[i] = new(big.Rat).Set(q)
	}
	for i := 0; i < 3; i++ {
		if cp.Point[i].Cmp(sp.Point[i]) != 0 {
			return r, nil
		}
	}
	r.Contact = true
	r.Point = cp.Point
	r.Bounds = cp.PointBounds
	atKnot := func(q *big.Rat, knots []float64, lo, hi float64) bool {
		for _, k := range knots {
			if k > lo && k < hi && q.Cmp(new(big.Rat).SetFloat64(k)) == 0 {
				return true
			}
		}
		return false
	}
	d, cd := surface.Domain(), curve.patch.Domain()
	if atKnot(t, curve.patch.knotsU, cd.UMin, cd.UMax) || atKnot(u, surface.knotsU, d.UMin, d.UMax) || atKnot(v, surface.knotsV, d.VMin, d.VMax) {
		return r, nil
	}
	determinant := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(sp.Du[j], sp.Dv[k]), new(big.Rat).Mul(sp.Du[k], sp.Dv[j]))
		term := new(big.Rat).Mul(cross, cp.Du[i])
		if e := b.check(term); e != nil {
			return r, e
		}
		determinant.Add(determinant, term)
	}
	if e := b.check(determinant); e != nil {
		return r, e
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.Direction = determinant.Sign()
	r.Regular = r.Direction != 0
	return r, nil
}
