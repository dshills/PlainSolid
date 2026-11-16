package geom

import (
	"context"
	"errors"
	"math/big"
)

// ProjectiveGraphBoundaryHeightOrder encloses A-minus-B on the exact
// positive-length common projected side of two original graph charts.
// Complete implies strict order everywhere or exact equality. Boundary
// retains each original directed side curve and its projective clock.
type ProjectiveGraphBoundaryHeightOrder struct {
	Boundary        ProjectiveGraphBoundaryContact
	Difference      ExactParameterInterval
	Sign            int
	ExactOperations int
	Complete        bool
}

// CertifyProjectiveGraphBoundaryHeightOrder proves the relative lifts
// using the exact original common projection and side clocks. A side
// must compose as one original Bézier span per owner. A residual range
// that contains zero does not establish a strict order.
func CertifyProjectiveGraphBoundaryHeightOrder(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	o SurfaceAffineCurveOptions) (r ProjectiveGraphBoundaryHeightOrder, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16_384
	}
	boundary, err := CertifyProjectiveGraphBoundaryContact(ctx, a, c, axes, o)
	r.ExactOperations = boundary.ExactOperations
	if err != nil && !errors.Is(err, ErrConvergence) {
		return r, err
	}
	ratio := boundary.BClockRatio
	if ratio == nil || ratio.Sign() <= 0 || len(boundary.Owners[0].Pieces) != 1 || len(boundary.Owners[1].Pieces) != 1 {
		return r, ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	b := &polynomialBudget{operations: r.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	ar := boundedRatArithmetic{b: b}
	one := big.NewRat(1, 1)
	height := 3 - axes[0] - axes[1]
	var lift, weight [2][]*big.Rat
	for owner, side := range boundary.Owners {
		if !side.Complete || !side.Continuous || side.Pieces[0].Start.Sign() != 0 || side.Pieces[0].End.Cmp(one) != 0 {
			return r, ErrConvergence
		}
		limits := SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations - b.operations, MaxCoefficientBits: b.maxBits}
		if limits.MaxOperations < 1 {
			return r, ErrConvergence
		}
		net, used, err := side.Pieces[0].Curve.BezierControlNet(ctx, limits)
		b.operations += used
		if err != nil {
			return r, err
		}
		q := one
		for i, h := range net.Control {
			if owner == 1 && i > 0 {
				q = ar.mul(q, ratio)
			}
			lift[owner] = append(lift[owner], ar.mul(h[height], q))
			weight[owner] = append(weight[owner], ar.mul(h[3], q))
			if ar.err != nil {
				return r, ar.err
			}
		}
	}
	// The homogeneous pullback for t -> ratio*t/(1+(ratio-1)*t)
	// scales Bernstein control i by ratio^i. Common denominators
	// cancel before comparing the physical z coordinate.
	u, err := exactBernsteinScalarProduct(lift[0], weight[1], b)
	if err != nil {
		return r, err
	}
	v, err := exactBernsteinScalarProduct(lift[1], weight[0], b)
	if err != nil {
		return r, err
	}
	denom, err := exactBernsteinScalarProduct(weight[0], weight[1], b)
	if err != nil {
		return r, err
	}
	var difference ExactParameterInterval
	for i, x := range u {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if denom[i].Sign() <= 0 {
			return r, ErrConvergence
		}
		h := ar.quo(ar.sub(x, v[i]), denom[i])
		if ar.err != nil {
			return r, ar.err
		}
		if difference.Lower == nil || h.Cmp(difference.Lower) < 0 {
			difference.Lower = h
		}
		if difference.Upper == nil || h.Cmp(difference.Upper) > 0 {
			difference.Upper = h
		}
	}
	r.Boundary, r.Difference = boundary, difference
	if difference.Lower.Sign() > 0 || difference.Upper.Sign() < 0 ||
		difference.Lower.Sign() == 0 && difference.Upper.Sign() == 0 {
		r.Sign, r.Complete = difference.Lower.Sign(), true
		return r, nil
	}
	return r, ErrConvergence
}
