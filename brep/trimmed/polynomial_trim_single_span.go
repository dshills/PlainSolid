package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// polynomialTrimBezierDescriptor preserves a single parameterized rational
// Bezier even when its original author inserted redundant knots. At most eight
// active spans and 64 source poles/knots are accepted on this extra path. A
// candidate extrapolated from the first cell is never evidence by itself: exact
// cross-multiplied identity over every original span is mandatory. Each existing
// descriptor operation retains its SurfacePatchOptions work limit; the complete
// comparison has one cumulative limit across at most eight spans. Extrapolation
// is separately bounded by the same operation/bit limits (at most 120 products
// and sums for degree four). No sampled fitting or original knot mutation is performed.
func polynomialTrimBezierDescriptor(ctx context.Context, q SplineUV, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if q.Curve == nil || q.Curve.Degree() < 1 || q.Curve.Degree() > 4 {
		return nil, geom.ErrConvergence
	}
	first := q.Range.Max
	spans := 1
	last := q.Range.Min
	for _, k := range q.Curve.Knots() {
		if k > q.Range.Min && k < q.Range.Max && k != last {
			if spans == 1 {
				first = k
			}
			spans++
			last = k
		}
	}
	if spans > 1 && (spans > 8 || len(q.Curve.Knots()) > 64 || len(q.Curve.Poles()) > 64) {
		return nil, geom.ErrConvergence
	}
	c, reverse, e := coincidentUseCurve(ctx, Use{Curve: q}, o)
	if e != nil {
		return nil, e
	}
	if spans == 1 {
		return c.Reparameterize(0, 1, reverse, o)
	}
	piece, e := c.Restrict(q.Range.Min, first, o)
	if e != nil {
		return nil, e
	}
	net, _, e := piece.BezierControlNet(ctx, o)
	if e != nil {
		return nil, e
	}
	n := len(net.Control) - 1
	if n < 1 || n > 4 {
		return nil, geom.ErrConvergence
	}
	scale := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(q.Range.Max), ratInset(q.Range.Min)), new(big.Rat).Sub(ratInset(first), ratInset(q.Range.Min)))
	complement := new(big.Rat).Sub(big.NewRat(1, 1), scale)
	maxBits, maxOps := o.MaxCoefficientBits, o.MaxOperations
	if maxBits == 0 {
		maxBits = 16384
	}
	if maxOps == 0 {
		maxOps = 1000000
	}
	if maxBits < 1 || maxOps < 1 {
		return nil, geom.ErrInvalid
	}
	check := func(v *big.Rat) bool { return v.Num().BitLen() <= maxBits && v.Denom().BitLen() <= maxBits }
	if !check(scale) || !check(complement) {
		return nil, geom.ErrConvergence
	}
	work := net.Control
	h := make([][4]*big.Rat, n+1)
	h[0] = work[0]
	operations := 0
	for level := 1; level <= n; level++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for i := 0; i <= n-level; i++ {
			for k := 0; k < 4; k++ {
				if operations > maxOps-3 {
					return nil, geom.ErrConvergence
				}
				operations += 3
				a := new(big.Rat).Mul(complement, work[i][k])
				b := new(big.Rat).Mul(scale, work[i+1][k])
				v := new(big.Rat).Add(a, b)
				if !check(a) || !check(b) || !check(v) {
					return nil, geom.ErrConvergence
				}
				work[i][k] = v
			}
		}
		h[level] = work[0]
	}
	candidate, e := geom.NewExactHomogeneousBezier(ctx, h, o)
	if e != nil {
		return nil, e
	}
	proof, e := geom.CompareExactCurvesContext(ctx, c, candidate, geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxSpans: 8, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	if !proof.Complete || !proof.Equal {
		return nil, geom.ErrConvergence
	}
	return candidate.Reparameterize(0, 1, reverse, o)
}
