//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// BoundAffineConvexWholeVolume bounds the exact represented affine image of
// this already-proved native common. It does not rebuild an oblique Boolean.
// New projected coordinates must be independent of the old residual coordinate;
// the projected 2x2 map and residual scale must both be nonsingular. Thus every
// original graph fiber maps bijectively to a new fiber. Its depth scales by the
// absolute residual scale, even when the new residual includes projected shear.
// Exact convex overlap, not its enclosing rectangle, supplies the area bound.
// Original native face IDs are retained. This does not certify boundary area,
// inertia, point membership, or material occupancy throughout the overlap.
// Exact arithmetic and source clipping share one cumulative operation budget.
func (q *AlgebraicGraphMaterialQuery) BoundAffineConvexWholeVolume(ctx context.Context, m geom.Affine, axes [2]int, o AlgebraicGraphConvexOverlapOptions) (out AlgebraicGraphConvexOverlap, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphConvexOverlap{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || !m.IsFinite() || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o.Exact, err = graphMaterialLimits(o.Exact)
	if err != nil {
		return out, err
	}
	residual := 3 - axes[0] - axes[1]
	if m[axes[0]][q.residual] != 0 || m[axes[1]][q.residual] != 0 || m[residual][q.residual] == 0 {
		return out, geom.ErrConvergence
	}
	var failure error
	used := 0
	calc := func(op byte, a, b *big.Rat) *big.Rat {
		r := new(big.Rat)
		if failure != nil {
			return r
		}
		if used >= o.Exact.MaxOperations || !graphPolynomialBits(a, o.Exact.MaxCoefficientBits) || !graphPolynomialBits(b, o.Exact.MaxCoefficientBits) {
			failure = geom.ErrConvergence
			return r
		}
		used++
		switch op {
		case '+':
			r.Add(a, b)
		case '-':
			r.Sub(a, b)
		case '*':
			r.Mul(a, b)
		}
		if !graphPolynomialBits(r, o.Exact.MaxCoefficientBits) {
			failure = geom.ErrConvergence
		}
		return r
	}
	var a [2][3]*big.Rat
	for i, row := range axes {
		a[i] = [3]*big.Rat{ratInset(m[row][q.axes[0]]), ratInset(m[row][q.axes[1]]), ratInset(m[row][3])}
	}
	det := calc('-', calc('*', a[0][0], a[1][1]), calc('*', a[0][1], a[1][0]))
	scale := new(big.Rat).Abs(ratInset(m[residual][q.residual]))
	if failure != nil || det.Sign() == 0 {
		out.ExactOperations = used
		return out, geom.ErrConvergence
	}
	remaining := o
	remaining.Exact.MaxOperations -= used
	if remaining.Exact.MaxOperations < 1 {
		out.ExactOperations = used
		return out, geom.ErrConvergence
	}
	out, err = q.BoundConvexWholeVolume(ctx, remaining)
	used += out.ExactOperations
	if err != nil {
		out.ExactOperations = used
		return out, err
	}
	transform := func(p [][2]*big.Rat) {
		for i, x := range p {
			for j := 0; j < 2; j++ {
				p[i][j] = calc('+', calc('+', calc('*', a[j][0], x[0]), calc('*', a[j][1], x[1])), a[j][2])
			}
		}
		if det.Sign() < 0 {
			for i, j := 1, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
	for i := range out.Original {
		transform(out.Original[i])
	}
	transform(out.Overlap)
	determinant := new(big.Rat).Abs(det)
	out.ProjectedArea = calc('*', out.ProjectedArea, determinant)
	out.DepthUpper = calc('*', out.DepthUpper, scale)
	out.Volume[1] = calc('*', out.ProjectedArea, out.DepthUpper)
	out.Window = AlgebraicGraphProjectionWindow{}
	for i, p := range out.Overlap {
		for j := 0; j < 2; j++ {
			if i == 0 {
				out.Window.Min[j] = new(big.Rat).Set(p[j])
				out.Window.Max[j] = new(big.Rat).Set(p[j])
			} else {
				if p[j].Cmp(out.Window.Min[j]) < 0 {
					out.Window.Min[j].Set(p[j])
				}
				if p[j].Cmp(out.Window.Max[j]) > 0 {
					out.Window.Max[j].Set(p[j])
				}
			}
		}
	}
	out.ExactOperations = used
	if failure != nil {
		return out, failure
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Placement = m
	out.ProjectionAxes = axes
	out.ResidualAxis = residual
	return out, nil
}
