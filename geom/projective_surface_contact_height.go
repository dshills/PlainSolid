package geom

import (
	"context"
	"math/big"
)

// ProjectiveContactHeight binds an exact common station to both original
// raw UV charts and the signed height difference A minus B. Projected stores
// the two physical model coordinates along the graph's chosen axes.
type ProjectiveContactHeight struct {
	OriginalUV                        [2][2]*big.Rat
	Projected                         [2]*big.Rat
	Difference                        *big.Rat
	Sign, ExactOperations             int
	OnOverlapBoundary, OnTrimBoundary bool
}

// ClassifyCommonHeightRational evaluates the sign of the original lifted
// difference at one exact common station. The normalized common clock is
// A's; holes exclude only their open interiors. The graph's immutable
// proof supplies original owner cells and their exact chart correspondence.
func (g ProjectiveSurfaceContactGraph) ClassifyCommonHeightRational(ctx context.Context, uv [2]*big.Rat,
	o SurfacePatchEvaluationOptions) (r ProjectiveContactHeight, err error) {
	if ctx == nil || g.proof == nil || uv[0] == nil || uv[1] == nil {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	one := big.NewRat(1, 1)
	point := [2]*ExactAlgebraicRoot{}
	for axis, u := range uv {
		if err := b.check(u); err != nil {
			return r, err
		}
		if u.Sign() < 0 || u.Cmp(one) > 0 {
			return r, ErrDomain
		}
		x := new(big.Rat).Set(u)
		point[axis] = &ExactAlgebraicRoot{root: &ExactGraphLineRoot{polynomial: rationalPolynomial{new(big.Rat).Neg(x), one}, bounds: [2]*big.Rat{x, x}}}
	}
	for _, hole := range g.proof.holes {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if err := b.check(hole.U.Lower); err != nil {
			return r, err
		}
		if uv[0].Cmp(hole.U.Lower) > 0 && uv[0].Cmp(hole.U.Upper) < 0 &&
			uv[1].Cmp(hole.V.Lower) > 0 && uv[1].Cmp(hole.V.Upper) < 0 {
			return r, ErrDomain
		}
	}
	for _, cell := range g.proof.cells {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if err := b.check(cell.Common.U.Lower); err != nil {
			return r, err
		}
		box := cell.Common
		if uv[0].Cmp(box.U.Lower) < 0 || uv[0].Cmp(box.U.Upper) > 0 ||
			uv[1].Cmp(box.V.Lower) < 0 || uv[1].Cmp(box.V.Upper) > 0 {
			continue
		}
		a := boundedRatArithmetic{b: b}
		u := a.quo(a.sub(uv[0], box.U.Lower), a.sub(box.U.Upper, box.U.Lower))
		v := a.quo(a.sub(uv[1], box.V.Lower), a.sub(box.V.Upper, box.V.Lower))
		if a.err != nil {
			return r, a.err
		}
		w := b.maxOperations - b.operations
		if w < 1 {
			return r, ErrConvergence
		}
		d, err := cell.Difference.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{MaxOperations: w, MaxCoefficientBits: b.maxBits})
		b.operations += d.ExactOperations
		if err != nil {
			return r, err
		}
		q, err := projectiveGraphContactVertex(ctx, point, g.proof.overlap, g.proof.holes, b)
		if err != nil {
			return r, err
		}
		var original [2][2]*big.Rat
		for owner, row := range q.OriginalUV {
			for axis, raw := range row {
				x := raw.Interval()
				if x.Lower.Cmp(x.Upper) != 0 {
					return r, ErrConvergence
				}
				original[owner][axis] = x.Lower
			}
		}
		if err := ctx.Err(); err != nil {
			return r, err
		}
		r.OriginalUV, r.Projected = original, [2]*big.Rat{d.Point[0], d.Point[1]}
		r.Difference, r.Sign = d.Point[2], d.Point[2].Sign()
		r.OnOverlapBoundary, r.OnTrimBoundary = q.OnOverlapBoundary, q.OnTrimBoundary
		return r, nil
	}
	return r, ErrConvergence
}
