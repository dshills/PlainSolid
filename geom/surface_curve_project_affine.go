package geom

import (
	"context"
	"math/big"
)

// AffineComposedBezierProjectionPieces retains exact planar chart forms
// U=a.XYZ+d and V=b.XYZ+d. Forms/Plane are caller-owned. Each normalized piece
// preserves the source parameter orientation and its exact OriginalRange.
type AffineComposedBezierProjectionPieces struct {
	Complete      bool
	OriginalRange [2]*big.Rat
	Forms         [2][4]*big.Rat
	Plane         [4]*big.Rat
	Pieces        []ComposedBezierProjection
}

// ProjectAffineBezierPieces2D proves whole-cell raw-plane incidence and an
// invertible affine chart on that plane. It recovers positive Bernstein pieces
// and checks all homogeneous coefficients against the transported source after
// exact original-parameter substitution. Nil form/plane coefficients mean zero.
// Transformation, subdivision stages, and each piece verification independently
// use EventOptions' exact budget, with piece/depth counts bounding total work.
// Any failure returns no partial report. Nonplanar or rank-deficient inputs fail.
func (c *ExactComposedCurveCell) ProjectAffineBezierPieces2D(ctx context.Context, forms [2][4]*big.Rat, plane [4]*big.Rat, ends [2]*big.Rat, o ComposedProjectionOptions) (AffineComposedBezierProjectionPieces, error) {
	var zero AffineComposedBezierProjectionPieces
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	out := AffineComposedBezierProjectionPieces{}
	for j := 0; j < 4; j++ {
		out.Plane[j] = new(big.Rat)
		if plane[j] != nil {
			out.Plane[j].Set(plane[j])
		}
		if e := b.check(out.Plane[j]); e != nil {
			return zero, e
		}
		for i := 0; i < 2; i++ {
			out.Forms[i][j] = new(big.Rat)
			if forms[i][j] != nil {
				out.Forms[i][j].Set(forms[i][j])
			}
			if e := b.check(out.Forms[i][j]); e != nil {
				return zero, e
			}
		}
	}
	determinant := new(big.Rat)
	for j := 0; j < 3; j++ {
		a, z := (j+1)%3, (j+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(out.Forms[0][a], out.Forms[1][z]), new(big.Rat).Mul(out.Forms[0][z], out.Forms[1][a]))
		determinant.Add(determinant, new(big.Rat).Mul(cross, out.Plane[j]))
		if e := b.check(determinant); e != nil {
			return zero, e
		}
	}
	if determinant.Sign() == 0 {
		return zero, ErrDegenerate
	}
	transform := func(form [4]*big.Rat) (rationalPolynomial, error) {
		var result rationalPolynomial
		for axis, q := range form {
			for len(result) < len(c.power[axis]) {
				result = append(result, new(big.Rat))
			}
			for i, v := range c.power[axis] {
				result[i].Add(result[i], new(big.Rat).Mul(q, v))
				if e := b.check(result[i]); e != nil {
					return nil, e
				}
			}
		}
		return trimPolynomial(result), nil
	}
	residual, e := transform(out.Plane)
	if e != nil {
		return zero, e
	}
	if len(residual) != 0 {
		return zero, ErrDomain
	}
	projected := &ExactComposedCurveCell{endpoints: c.endpoints}
	for i := 0; i < 2; i++ {
		projected.power[i], e = transform(out.Forms[i])
		if e != nil {
			return zero, e
		}
	}
	projected.power[3] = make(rationalPolynomial, len(c.power[3]))
	for i, v := range c.power[3] {
		projected.power[3][i] = new(big.Rat).Set(v)
	}
	pieces, e := projected.ProjectBezierPieces2D(ctx, [2]int{0, 1}, ends, o)
	if e != nil {
		return zero, e
	}
	for _, piece := range pieces.Pieces {
		verify := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
		actual, e := sharedProjectionPowers(piece.Curve.patch, verify)
		if e != nil {
			return zero, e
		}
		linear := rationalPolynomial{piece.OriginalRange[0], new(big.Rat).Sub(piece.OriginalRange[1], piece.OriginalRange[0])}
		for axis := 0; axis < 4; axis++ {
			expected, e := composeProjectionPolynomial(projected.power[axis], linear, verify)
			if e != nil {
				return zero, e
			}
			a, z := trimPolynomial(actual[axis]), trimPolynomial(expected)
			if len(a) != len(z) {
				return zero, ErrConvergence
			}
			for j := range a {
				if a[j].Cmp(z[j]) != 0 {
					return zero, ErrConvergence
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	out.OriginalRange = pieces.OriginalRange
	out.Pieces = pieces.Pieces
	return out, nil
}
