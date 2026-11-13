package geom

import (
	"context"
	"errors"
	"math/big"
)

// GraphPointContactDiscoveryOptions bounds finite exact witness proposals and
// their global proofs. MaxCandidates defaults to128; other limits use the same
// defaults as GraphPointContactOptions and are cumulative across attempts.
type GraphPointContactDiscoveryOptions struct {
	GraphPointContactOptions
	MaxCandidates int
}

// SplineGraphPointContactDiscovery proves a contact only through Contact.Proven.
// Candidate exhaustion is not an absence certificate: unproposed irrational or
// higher-degree stationary points may exist. ErrConvergence retains diagnostics.
type SplineGraphPointContactDiscovery struct {
	Contact                                          SplineGraphPointContact
	CandidatesTested, ProofAttempts, ExactOperations int
}

// DiscoverSplineGraphPointContact tests the exact rectangle center, exact
// quadratic stationary/edge-stationary points, then corners and union-knot
// vertices. Quadratic candidates are derived only after all higher polynomial
// coefficients are proved zero; no approximate Newton root is promoted to exact.
// Every accepted witness is rechecked by CertifySplineGraphPointContact.
func DiscoverSplineGraphPointContact(ctx context.Context, a, c *SurfacePatch, o GraphPointContactDiscoveryOptions) (r SplineGraphPointContactDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 1 || o.MaxDepth < 0 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	p, ok, e := commonXYDifference(a, c, b)
	if e != nil {
		return r, e
	}
	if !ok {
		remaining := o.MaxOperations - b.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		p, e = NewGeneralSurfaceHeightDifferencePatch(ctx, a, c, GeneralSurfaceHeightOptions{MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return r, e
		}
		b.operations += p.operations
	}
	seen := map[string]bool{}
	try := func(uv [2]*big.Rat) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		for _, x := range uv {
			if e := b.check(x); e != nil {
				return false, e
			}
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
		jet, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return false, e
		}
		if jet.Point[2].Sign() != 0 {
			return false, nil
		}
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return false, ErrConvergence
		}
		proofOptions := o.GraphPointContactOptions
		proofOptions.MaxOperations = remaining
		r.ProofAttempts++
		proof, e := CertifySplineGraphPointContact(ctx, a, c, uv, proofOptions)
		b.operations += proof.ExactOperations
		r.Contact = proof
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

func quadraticGraphWitnesses(p *SurfacePatch, b *polynomialBudget) (out [][2]*big.Rat, err error) {
	rows := make([]rationalPolynomial, p.countU)
	for i := 0; i < p.countU; i++ {
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		rows[i], err = bernsteinToPower(values, b)
		if err != nil {
			return nil, err
		}
	}
	var coefficient [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			coefficient[i][j] = new(big.Rat)
		}
	}
	for j := 0; j < p.countV; j++ {
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		column, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		for i, value := range column {
			if i+j > 2 {
				if value.Sign() != 0 {
					return nil, nil
				}
			} else {
				coefficient[i][j].Set(value)
			}
		}
	}
	du, dv := coefficient[1][0], coefficient[0][1]
	aa, dd, cross := new(big.Rat).Mul(big.NewRat(2, 1), coefficient[2][0]), new(big.Rat).Mul(big.NewRat(2, 1), coefficient[0][2]), coefficient[1][1]
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(aa, dd), new(big.Rat).Mul(cross, cross))
	if e := b.check(determinant); e != nil {
		return nil, e
	}
	d := p.Domain()
	one := big.NewRat(1, 1)
	add := func(u, v *big.Rat) error {
		if u.Sign() < 0 || u.Cmp(one) > 0 || v.Sign() < 0 || v.Cmp(one) > 0 {
			return nil
		}
		uv := [2]*big.Rat{parameterMap(ParameterSpan{Start: d.UMin, End: d.UMax}, u), parameterMap(ParameterSpan{Start: d.VMin, End: d.VMax}, v)}
		for _, x := range uv {
			if e := b.check(x); e != nil {
				return e
			}
		}
		out = append(out, uv)
		return nil
	}
	if determinant.Sign() != 0 {
		u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(cross, dv), new(big.Rat).Mul(dd, du)), determinant)
		v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(cross, du), new(big.Rat).Mul(aa, dv)), determinant)
		if e := add(u, v); e != nil {
			return nil, e
		}
	}
	for _, fixed := range []*big.Rat{new(big.Rat), one} {
		if dd.Sign() != 0 {
			v := new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).Add(dv, new(big.Rat).Mul(cross, fixed))), dd)
			if e := add(fixed, v); e != nil {
				return nil, e
			}
		}
		if aa.Sign() != 0 {
			u := new(big.Rat).Quo(new(big.Rat).Neg(new(big.Rat).Add(du, new(big.Rat).Mul(cross, fixed))), aa)
			if e := add(u, fixed); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}
