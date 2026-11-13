package geom

import (
	"context"
	"math/big"
	"sort"
)

// SurfaceAffineCurveOptions bounds exact knot-aligned curve extraction.
// MaxPieces zero selects 4096; embedded zero values use AffineParameterCurve defaults.
type SurfaceAffineCurveOptions struct {
	TubeAffineLineOptions
	MaxPieces int
}

// SurfaceAffineCurvePiece spans original normalized line parameters Start..End.
// Curve uses its own local parameter s in [0,1], where t=Start+(End-Start)*s.
// Cell records the knot rectangle selected without rounded knot parameters.
type SurfaceAffineCurvePiece struct {
	Start, End *big.Rat
	Curve      *ExactSplineCurve
	Cell       ParameterRectangle
}

// SurfaceAffineCurvePieces is a complete ordered partition when Complete.
// Continuous means adjacent exact model endpoints have been compared and agree.
// Derivative continuity across knots is intentionally not asserted.
type SurfaceAffineCurvePieces struct {
	Pieces               []SurfaceAffineCurvePiece
	Complete, Continuous bool
	ExactOperations      int
}

// AffineParameterCurvePieces restricts a multispan positive rational tensor
// spline to an affine rational UV segment. Every crossed knot is mapped into
// exact rational normalized t, including nonbinary cuts and coincident U/V knot
// crossings. Segments lying on a knot use the right-hand cell (last cell at the
// maximum), with original C0 spline semantics. Exact endpoint comparisons verify
// the continuity of the returned partition; no rounded spline reconstruction occurs.
func (p *SurfacePatch) AffineParameterCurvePieces(ctx context.Context, start, end [2]*big.Rat, o SurfaceAffineCurveOptions) (r SurfaceAffineCurvePieces, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxPieces < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	d := p.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	knots := [2][]float64{p.knotsU, p.knotsV}
	var delta [2]*big.Rat
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for axis := 0; axis < 2; axis++ {
		for _, x := range []*big.Rat{start[axis], end[axis]} {
			if x == nil {
				return r, ErrInvalid
			}
			if e := budget.check(x); e != nil {
				return r, e
			}
			if x.Cmp(new(big.Rat).SetFloat64(ranges[axis][0])) < 0 || x.Cmp(new(big.Rat).SetFloat64(ranges[axis][1])) > 0 {
				return r, ErrDomain
			}
		}
		delta[axis] = new(big.Rat).Sub(end[axis], start[axis])
		if delta[axis].Sign() == 0 {
			continue
		}
		previous := ranges[axis][0]
		for _, knot := range knots[axis] {
			if knot <= previous || knot >= ranges[axis][1] {
				continue
			}
			previous = knot
			t := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(knot), start[axis]), delta[axis])
			if e := budget.check(t); e != nil {
				return r, e
			}
			if t.Sign() > 0 && t.Cmp(big.NewRat(1, 1)) < 0 {
				cuts = append(cuts, t)
			}
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
	unique := cuts[:0]
	for _, t := range cuts {
		if len(unique) == 0 || unique[len(unique)-1].Cmp(t) != 0 {
			unique = append(unique, t)
		}
	}
	cuts = unique
	if len(cuts)-1 > o.MaxPieces {
		return r, ErrConvergence
	}
	evaluateUV := func(t *big.Rat) [2]*big.Rat {
		return [2]*big.Rat{new(big.Rat).Add(start[0], new(big.Rat).Mul(t, delta[0])), new(big.Rat).Add(start[1], new(big.Rat).Mul(t, delta[1]))}
	}
	var previousEnd [3]*big.Rat
	for i := 0; i < len(cuts)-1; i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		middle := new(big.Rat).Quo(new(big.Rat).Add(cuts[i], cuts[i+1]), big.NewRat(2, 1))
		point := evaluateUV(middle)
		var cellBounds [2][2]float64
		for axis := 0; axis < 2; axis++ {
			found := false
			for j := 0; j < len(knots[axis])-1; j++ {
				lo, hi := knots[axis][j], knots[axis][j+1]
				if lo >= hi {
					continue
				}
				if point[axis].Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && (point[axis].Cmp(new(big.Rat).SetFloat64(hi)) < 0 || hi == ranges[axis][1] && point[axis].Cmp(new(big.Rat).SetFloat64(hi)) == 0) {
					cellBounds[axis] = [2]float64{lo, hi}
					found = true
					break
				}
			}
			if !found {
				return r, ErrDomain
			}
		}
		cell := ParameterRectangle{UMin: cellBounds[0][0], UMax: cellBounds[0][1], VMin: cellBounds[1][0], VMax: cellBounds[1][1]}
		patch, e := trimPatchBudget(p, cell, budget)
		if e != nil {
			return r, e
		}
		remaining := budget.maxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		options := o.TubeAffineLineOptions
		options.MaxOperations = remaining
		curve, e := patch.AffineParameterCurve(ctx, evaluateUV(cuts[i]), evaluateUV(cuts[i+1]), options)
		if e != nil {
			return r, e
		}
		budget.operations += curve.ExactOperations()
		first, e := exactCurveRationalPoint(curve.patch, new(big.Rat), budget)
		if e != nil {
			return r, e
		}
		last, e := exactCurveRationalPoint(curve.patch, big.NewRat(1, 1), budget)
		if e != nil {
			return r, e
		}
		if i > 0 {
			for axis := 0; axis < 3; axis++ {
				if first[axis].Cmp(previousEnd[axis]) != 0 {
					return r, ErrConvergence
				}
			}
		}
		previousEnd = last
		r.Pieces = append(r.Pieces, SurfaceAffineCurvePiece{Start: new(big.Rat).Set(cuts[i]), End: new(big.Rat).Set(cuts[i+1]), Curve: curve, Cell: cell})
	}
	r.Continuous = true
	r.Complete = true
	return r, nil
}
