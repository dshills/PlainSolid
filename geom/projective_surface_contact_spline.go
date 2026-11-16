package geom

import (
	"context"
	"math/big"
)

// ProjectiveContactSpline stores a single shared analytic model curve and
// both original UV curves. All use the same normalized [0,1] clock from
// the arc's first endpoint to its second. No approximate projection is used.
type ProjectiveContactSpline struct {
	Model           *ExactSplineCurve
	UV              [2]*ExactSplineCurve
	ExactOperations int
}

// BuildArcSpline constructs an exact model curve and both original pcurves
// when a contact arc is straight in the common chart and has provably
// rational endpoints. A regular implicit arc must have an identically zero
// homogeneous residual on the proposed line; endpoint equality alone is
// insufficient. Each owner must admit a single original spline composition.
func (g ProjectiveSurfaceContactGraph) BuildArcSpline(ctx context.Context, index int,
	options SurfacePatchOptions) (r ProjectiveContactSpline, err error) {
	if ctx == nil || g.proof == nil || index < 0 || index >= len(g.proof.arcs) {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	o, err := patchOptions(options)
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	remaining := func() (SurfacePatchOptions, error) {
		limits := o
		limits.MaxOperations -= b.operations
		if limits.MaxOperations < 1 {
			return limits, ErrConvergence
		}
		return limits, nil
	}
	proof := g.proof.arcs[index]
	var endpoints [2][2]*big.Rat
	for i, end := range proof.ends {
		for axis, root := range end {
			q, err := projectiveSplineRationalEndpoint(ctx, root.root, b)
			if err != nil {
				return r, err
			}
			endpoints[i][axis] = q
		}
	}
	a := boundedRatArithmetic{b: b}
	one, zero := big.NewRat(1, 1), new(big.Rat)
	makeLine := func(q [2][2]*big.Rat) (*ExactSplineCurve, error) {
		limits, err := remaining()
		if err != nil {
			return nil, err
		}
		c, err := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{q[0][0], q[0][1], zero, one}, {q[1][0], q[1][1], zero, one}}, limits)
		if err == nil {
			b.operations += c.ExactOperations()
		}
		return c, err
	}
	if proof.saddle == nil {
		// Arc.source uses the residual tile's parameter clock, possibly
		// restricted by local subdivision. Recover its two exact stations.
		var local [2][2]*big.Rat
		for i, q := range endpoints {
			for j, d := range [2]ExactParameterInterval{proof.common.U, proof.common.V} {
				local[i][j] = a.quo(a.sub(q[j], d.Lower), a.sub(d.Upper, d.Lower))
			}
		}
		if a.err != nil {
			return r, a.err
		}
		line, err := makeLine(local)
		if err != nil {
			return r, err
		}
		limits, err := remaining()
		if err != nil {
			return r, err
		}
		delta, err := proof.arc.source.ComposeParameterCurve(ctx, line, limits)
		if err != nil {
			return r, err
		}
		b.operations += delta.ExactOperations()
		for _, h := range delta.patch.net {
			if h[2].Sign() != 0 {
				return r, ErrConvergence
			}
		}
	}
	var uv [2]*ExactSplineCurve
	var qA [2][2]*big.Rat
	var numerator, denominator [2][]*big.Rat
	overlap := g.proof.overlap
	for axis, x := range [2]ExactParameterInterval{overlap.Areas[0].U, overlap.Areas[0].V} {
		rawB := axis
		if overlap.SwapB {
			rawB = 1 - axis
		}
		y := [2]ExactParameterInterval{overlap.Areas[1].U, overlap.Areas[1].V}[rawB]
		numerator[rawB], denominator[rawB] = make([]*big.Rat, 2), make([]*big.Rat, 2)
		ratio := overlap.BStationRatios[axis]
		for i, q := range endpoints {
			st := q[axis]
			qA[i][axis] = a.add(x.Lower, a.mul(a.sub(x.Upper, x.Lower), st))
			n := a.mul(ratio, st)
			d := a.add(one, a.mul(a.sub(ratio, one), st))
			if [2]bool{overlap.ReverseBU, overlap.ReverseBV}[axis] {
				n = a.sub(d, n)
			}
			numerator[rawB][i] = a.add(a.mul(y.Lower, d), a.mul(a.sub(y.Upper, y.Lower), n))
			denominator[rawB][i] = d
			if a.err != nil {
				return r, a.err
			}
			if d.Sign() <= 0 {
				return r, ErrConvergence
			}
		}
	}
	uv[0], err = makeLine(qA)
	if err != nil {
		return r, err
	}
	var bernstein [3][]*big.Rat
	for i := range bernstein {
		x := denominator[0]
		y := denominator[1]
		if i == 0 {
			x = numerator[0]
		} else if i == 1 {
			y = numerator[1]
		}
		bernstein[i], err = exactBernsteinScalarProduct(x, y, b)
		if err != nil {
			return r, err
		}
	}
	limits, err := remaining()
	if err != nil {
		return r, err
	}
	controls := make([][4]*big.Rat, 3)
	for i := range controls {
		controls[i] = [4]*big.Rat{bernstein[0][i], bernstein[1][i], zero, bernstein[2][i]}
	}
	uv[1], err = NewExactHomogeneousBezier(ctx, controls, limits)
	if err != nil {
		return r, err
	}
	b.operations += uv[1].ExactOperations()

	var model [2]*ExactSplineCurve
	for i, p := range g.proof.sources {
		limits, err := remaining()
		if err != nil {
			return r, err
		}
		result, err := p.ComposeParameterCurvePieces(ctx, uv[i], SurfaceCurveCompositionOptions{SurfacePatchOptions: limits})
		b.operations += result.ExactOperations
		if err != nil {
			return r, err
		}
		if !result.Complete || len(result.Pieces) != 1 || result.Pieces[0].Start.Sign() != 0 || result.Pieces[0].End.Cmp(one) != 0 {
			return r, ErrConvergence
		}
		model[i] = result.Pieces[0].Curve
	}
	limits, err = remaining()
	if err != nil {
		return r, err
	}
	comparison, err := CompareExactCurvesContext(ctx, model[0], model[1], ExactCurveComparisonOptions{
		MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
	b.operations += comparison.ExactOperations
	if err != nil {
		return r, err
	}
	if !comparison.Complete || !comparison.Equal {
		return r, ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Model, r.UV = model[0], uv
	return r, nil
}

func projectiveSplineRationalEndpoint(ctx context.Context, root *ExactGraphLineRoot, b *polynomialBudget) (*big.Rat, error) {
	var q *big.Rat
	_, err := isolatePolynomialRoots(root.polynomial, root.bounds[0], root.bounds[1], b, 256,
		func(lo, hi *big.Rat, _ int) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			x, err := recoverRationalPolynomialRoot(root.polynomial, lo, hi, b)
			if err != nil {
				return false, err
			}
			if x != nil {
				if q != nil {
					return false, ErrConvergence
				}
				q = x
				return true, nil
			}
			if len(root.polynomial) <= 3 || new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 1_000_000_000_000)) <= 0 {
				return false, ErrConvergence
			}
			return false, nil
		})
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, ErrConvergence
	}
	return q, nil
}
