package geom

import (
	"context"
	"errors"
	"math/big"
)

// SplineGraphOrdering proves Side*(A.Z-B.Z)>=MarginLower>0 throughout the
// closed common chart. MarginLower is exact; Margin is its outward conversion,
// whose float lower endpoint may underflow to zero without invalidating proof.
// Convexity is the retained shifted-numerator KKT/Hessian certificate; its Point
// belongs to the auxiliary zero-height contact, not the original surfaces.
type SplineGraphOrdering struct {
	Proven                                     bool
	Side                                       int
	MinimumNumerator, WeightUpper, MarginLower *big.Rat
	Margin                                     CertifiedScalarRange
	Convexity                                  SplineGraphPointContact
	ExactOperations                            int
}

func orderingDifference(ctx context.Context, a, c *SurfacePatch, maxCells int, b *polynomialBudget) (*SurfacePatch, error) {
	p, ok, e := commonXYDifference(a, c, b)
	if e != nil {
		return nil, e
	}
	if ok {
		return p, nil
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	p, e = NewGeneralSurfaceHeightDifferencePatch(ctx, a, c, GeneralSurfaceHeightOptions{MaxCells: maxCells, MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	if e != nil {
		return nil, e
	}
	b.operations += p.operations
	return p, nil
}

// CertifySplineGraphOrdering uses a rational minimum witness for the homogeneous
// height numerator. It proves C1 numerator compatibility, uniform definite
// Hessian, and closed-domain KKT signs through the existing contact certifier.
// A positive numerator minimum divided by a global positive denominator upper
// bound gives a strict geometric height margin, without a sampled estimate.
func CertifySplineGraphOrdering(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o GraphPointContactOptions) (r SplineGraphOrdering, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || uv[0] == nil || uv[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for _, x := range uv {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	var root *SurfacePatch
	for _, cell := range cells {
		d := cell.Domain()
		if uv[0].Cmp(new(big.Rat).SetFloat64(d.UMin)) >= 0 && uv[0].Cmp(new(big.Rat).SetFloat64(d.UMax)) <= 0 && uv[1].Cmp(new(big.Rat).SetFloat64(d.VMin)) >= 0 && uv[1].Cmp(new(big.Rat).SetFloat64(d.VMax)) <= 0 {
			root = cell
			break
		}
	}
	if root == nil {
		return r, ErrDomain
	}
	d := root.Domain()
	u := new(big.Rat).Quo(new(big.Rat).Sub(uv[0], new(big.Rat).SetFloat64(d.UMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin)))
	v := new(big.Rat).Quo(new(big.Rat).Sub(uv[1], new(big.Rat).SetFloat64(d.VMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin)))
	values := make([]*big.Rat, len(root.net))
	for i, h := range root.net {
		values[i] = h[2]
	}
	minimum, e := tensorScalarValue(values, root.countU, root.countV, u, v, b)
	if e != nil {
		return r, e
	}
	if minimum.Sign() == 0 {
		return r, ErrConvergence
	}
	shifted, zero := *p, *p
	shifted.net = make([]exactHomogeneous, len(p.net))
	zero.net = make([]exactHomogeneous, len(p.net))
	weightUpper := new(big.Rat)
	for i, h := range p.net {
		if h[3].Cmp(weightUpper) > 0 {
			weightUpper.Set(h[3])
		}
		for axis := 0; axis < 4; axis++ {
			shifted.net[i][axis] = new(big.Rat).Set(h[axis])
			zero.net[i][axis] = new(big.Rat).Set(h[axis])
		}
		shifted.net[i][2].Sub(shifted.net[i][2], minimum)
		zero.net[i][2].SetInt64(0)
		if e := b.check(shifted.net[i][2]); e != nil {
			return r, e
		}
	}
	if e := shifted.computeBounds(b); e != nil {
		return r, e
	}
	if e := zero.computeBounds(b); e != nil {
		return r, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	proofOptions := o
	proofOptions.MaxOperations = remaining
	proof, e := CertifySplineGraphPointContact(ctx, &shifted, &zero, uv, proofOptions)
	b.operations += proof.ExactOperations
	r.Convexity = proof
	if e != nil {
		return r, e
	}
	if !proof.Proven || proof.Side != minimum.Sign() {
		return r, ErrConvergence
	}
	margin := new(big.Rat).Quo(new(big.Rat).Abs(minimum), weightUpper)
	if e := b.check(margin); e != nil {
		return r, e
	}
	r.MinimumNumerator = new(big.Rat).Set(minimum)
	r.WeightUpper = weightUpper
	r.MarginLower = margin
	r.Margin = CertifiedScalarRange{Min: rationalFloatBound(margin, true), Max: rationalFloatBound(margin, false), Sign: 1}
	r.Side = minimum.Sign()
	r.Proven = true
	return r, nil
}

// SplineGraphOrderingDiscovery reports finite exact proposal search. Failure to
// find a witness does not prove that surfaces intersect or lack strict ordering.
type SplineGraphOrderingDiscovery struct {
	Ordering                          SplineGraphOrdering
	CandidatesTested, ExactOperations int
}

func DiscoverSplineGraphOrdering(ctx context.Context, a, c *SurfacePatch, o GraphPointContactDiscoveryOptions) (r SplineGraphOrderingDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCandidates < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	seen := map[string]bool{}
	try := func(uv [2]*big.Rat) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		key := uv[0].RatString() + "," + uv[1].RatString()
		if seen[key] {
			return false, nil
		}
		seen[key] = true
		if r.CandidatesTested >= o.MaxCandidates {
			return false, ErrConvergence
		}
		r.CandidatesTested++
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return false, ErrConvergence
		}
		local := o.GraphPointContactOptions
		local.MaxOperations = remaining
		proof, e := CertifySplineGraphOrdering(ctx, a, c, uv, local)
		b.operations += proof.ExactOperations
		r.Ordering = proof
		if e == nil && proof.Proven {
			return true, nil
		}
		if e != nil && !errors.Is(e, ErrConvergence) {
			return false, e
		}
		return false, nil
	}
	d := p.Domain()
	center := [2]*big.Rat{new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax))), new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)))}
	if done, e := try(center); e != nil || done {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	for _, cell := range cells {
		candidates, e := quadraticGraphWitnesses(cell, b)
		if e != nil {
			return r, e
		}
		for _, uv := range candidates {
			if done, e := try(uv); e != nil || done {
				return r, e
			}
		}
	}
	for _, cell := range cells {
		d := cell.Domain()
		for _, u := range []float64{d.UMin, d.UMax} {
			for _, v := range []float64{d.VMin, d.VMax} {
				if done, e := try([2]*big.Rat{new(big.Rat).SetFloat64(u), new(big.Rat).SetFloat64(v)}); e != nil || done {
					return r, e
				}
			}
		}
	}
	return r, ErrConvergence
}
