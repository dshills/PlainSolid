package geom

import (
	"context"
	"errors"
	"math/big"
	"sort"
)

func discoverMultispanSeparableContact(ctx context.Context, a, c, p *SurfacePatch, o SeparableGraphDiscoveryOptions, b *polynomialBudget) (r SeparableGraphDiscovery, err error) {
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	_, e = graphContactC1(ctx, cells, b)
	if e != nil {
		return r, e
	}
	d := p.Domain()
	var traces [2][]graphAxisTrace
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		ok, e := separableGraphCell(cell, b)
		if e != nil {
			return r, e
		}
		if !ok {
			return r, ErrConvergence
		}
		cd := cell.Domain()
		if cd.VMin == d.VMin {
			values := make([]*big.Rat, cell.countU)
			for i := range values {
				values[i] = cell.net[i*cell.countV][2]
			}
			poly, e := bernsteinToPower(values, b)
			if e != nil {
				return r, e
			}
			traces[0] = append(traces[0], graphAxisTrace{ParameterSpan{Start: cd.UMin, End: cd.UMax}, poly})
		}
		if cd.UMin == d.UMin {
			values := make([]*big.Rat, cell.countV)
			for i := range values {
				values[i] = cell.net[i][2]
			}
			poly, e := bernsteinToPower(values, b)
			if e != nil {
				return r, e
			}
			traces[1] = append(traces[1], graphAxisTrace{ParameterSpan{Start: cd.VMin, End: cd.VMax}, poly})
		}
	}
	var candidates [2][]*big.Rat
	for axis, list := range traces {
		sort.Slice(list, func(i, j int) bool { return list[i].span.Start < list[j].span.Start })
		seen := map[string]bool{}
		add := func(x *big.Rat) {
			key := x.RatString()
			if !seen[key] {
				seen[key] = true
				candidates[axis] = append(candidates[axis], x)
			}
		}
		for _, trace := range list {
			derivative := polynomialDerivative(trace.polynomial)
			if len(derivative) > 1 {
				gcd, e := polynomialGCD(derivative, polynomialDerivative(derivative), b)
				if e != nil {
					return r, e
				}
				squarefree, rest, e := polynomialDivide(derivative, gcd, b)
				if e != nil {
					return r, e
				}
				if len(rest) != 0 {
					return r, ErrConvergence
				}
				remaining := o.MaxRootSubdivisions - r.RootSubdivisions
				if remaining < 1 {
					return r, ErrConvergence
				}
				n, e := isolatePolynomialRoots(derivative, new(big.Rat), big.NewRat(1, 1), b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
					if e := ctx.Err(); e != nil {
						return false, e
					}
					root, e := recoverRationalPolynomialRoot(squarefree, lo, hi, b)
					if e != nil {
						return false, e
					}
					if root != nil {
						add(parameterMap(trace.span, root))
						return true, nil
					}
					if new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 10000000000)) > 0 {
						return false, nil
					}
					r.Unrecovered[axis] = append(r.Unrecovered[axis], ExactParameterInterval{parameterMap(trace.span, lo), parameterMap(trace.span, hi)})
					return true, nil
				})
				r.RootSubdivisions += n
				if e != nil {
					return r, e
				}
			}
			add(new(big.Rat).SetFloat64(trace.span.Start))
			add(new(big.Rat).SetFloat64(trace.span.End))
		}
	}
	for _, u := range candidates[0] {
		for _, v := range candidates[1] {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			remaining := o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			jet, e := p.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			b.operations += jet.ExactOperations
			if e != nil {
				return r, e
			}
			if jet.Point[2].Sign() != 0 {
				continue
			}
			remaining = o.MaxOperations - b.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			local := o.SeparableGraphContactOptions
			local.MaxOperations = remaining
			local.MaxRootSubdivisions = o.MaxRootSubdivisions - r.RootSubdivisions
			if local.MaxRootSubdivisions < 1 {
				return r, ErrConvergence
			}
			r.ProofAttempts++
			proof, e := CertifySeparableSplineGraphPointContact(ctx, a, c, [2]*big.Rat{u, v}, local)
			b.operations += proof.ExactOperations
			r.RootSubdivisions += proof.RootSubdivisions
			r.Contact = proof
			if e == nil && proof.Proven {
				return r, nil
			}
			if e != nil && !errors.Is(e, ErrConvergence) {
				return r, e
			}
		}
	}
	return r, ErrConvergence
}
