package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type trimPower9 [10]*big.Rat

func trimZero9() trimPower9 {
	var p trimPower9
	for i := range p {
		p[i] = new(big.Rat)
	}
	return p
}
func trimMultiply9(a, b trimPower9) trimPower9 {
	p := trimZero9()
	for i := range a {
		for j := 0; j < 10-i; j++ {
			p[i+j].Add(p[i+j], new(big.Rat).Mul(a[i], b[j]))
		}
	}
	return p
}

// BoundQuadraticSplineTrimCorrespondence proves a two-sided squared distance
// bound between the exact trim image and a supplied chord, using their common
// normalized parameter. The trim must be one clamped positive-weight rational
// quadratic (or linear) Bézier span; the patch must be a positive-weight single
// span of degree at most two in each parameter. All source UV control points
// must lie inside the patch domain. Unsupported cases return ErrConvergence.
//
// Range and Reversed select the original curve parameter and traversal; start
// and end follow that traversal. Represented chord endpoints may include any
// rounding or deliberate displacement. No evaluation samples establish this
// proof: the exact homogeneous composition has degree at most eight, its
// residual against the chord degree at most nine. Bernstein convexity and a
// positive denominator lower bound enclose every common-parameter residual.
// Work has fixed degree/size independent of requested tolerance; cancellation
// is checked between coefficient stages. This says nothing about loop topology.
func BoundQuadraticSplineTrimCorrespondence(ctx context.Context, patch *geom.SurfacePatch, trim SplineUV, start, end geom.Vec3) (*big.Rat, error) {
	if ctx == nil || patch == nil || trim.Curve == nil || !start.IsFinite() || !end.IsFinite() || !finite(trim.Range.Min) || !finite(trim.Range.Max) || trim.Range.Min >= trim.Range.Max {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	degree := trim.Curve.Degree()
	if degree < 1 || degree > 2 {
		return nil, geom.ErrConvergence
	}
	poles, knots, weights := trim.Curve.Poles(), trim.Curve.Knots(), trim.Curve.Weights()
	if len(poles) != degree+1 || len(knots) != 2*(degree+1) {
		return nil, geom.ErrConvergence
	}
	lo, hi := knots[0], knots[len(knots)-1]
	for i := 0; i <= degree; i++ {
		if knots[i] != lo || knots[i+degree+1] != hi {
			return nil, geom.ErrConvergence
		}
	}
	if trim.Range.Min < lo || trim.Range.Max > hi {
		return nil, geom.ErrDomain
	}
	domain := patch.Domain()
	var controls [3][3]*big.Rat
	minCurve := (*big.Rat)(nil)
	for i, p := range poles {
		if p.Z != 0 || p.X < domain.UMin || p.X > domain.UMax || p.Y < domain.VMin || p.Y > domain.VMax {
			return nil, geom.ErrDomain
		}
		w := big.NewRat(1, 1)
		if len(weights) != 0 {
			w = ratInset(weights[i])
		}
		if minCurve == nil || w.Cmp(minCurve) < 0 {
			minCurve = new(big.Rat).Set(w)
		}
		controls[i] = [3]*big.Rat{new(big.Rat).Mul(ratInset(p.X), w), new(big.Rat).Mul(ratInset(p.Y), w), w}
	}
	width := new(big.Rat).Sub(ratInset(hi), ratInset(lo))
	a := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(trim.Range.Min), ratInset(lo)), width)
	b := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(trim.Range.Max), ratInset(lo)), width)
	if trim.Reversed {
		a, b = b, a
	}
	linear := trimZero9()
	linear[0].Set(a)
	linear[1].Sub(b, a)
	square := trimMultiply9(linear, linear)
	var h [3]trimPower9
	for axis := 0; axis < 3; axis++ {
		h[axis] = trimZero9()
		h[axis][0].Set(controls[0][axis])
		c1 := new(big.Rat).Sub(controls[1][axis], controls[0][axis])
		c2 := new(big.Rat)
		if degree == 2 {
			c1.Mul(c1, big.NewRat(2, 1))
			c2.Add(controls[0][axis], controls[2][axis])
			c2.Sub(c2, new(big.Rat).Mul(big.NewRat(2, 1), controls[1][axis]))
		}
		for j := range h[axis] {
			h[axis][j].Add(h[axis][j], new(big.Rat).Mul(c1, linear[j]))
			h[axis][j].Add(h[axis][j], new(big.Rat).Mul(c2, square[j]))
		}
	}
	return boundTrimPowerCorrespondence(ctx, patch, h, minCurve, start, end)
}

func boundTrimPowerCorrespondence(ctx context.Context, patch *geom.SurfacePatch, h [3]trimPower9, minCurve *big.Rat, start, end geom.Vec3) (*big.Rat, error) {
	coefficients, minPatch, ok := patch.RationalBiquadraticCoefficients()
	if !ok {
		return nil, geom.ErrConvergence
	}
	domain := patch.Domain()
	// Normalize UV to the patch's unit-square power basis, retaining W.
	for axis, ends := range [][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}} {
		width := new(big.Rat).Sub(ratInset(ends[1]), ratInset(ends[0]))
		for j := range h[axis] {
			h[axis][j].Sub(h[axis][j], new(big.Rat).Mul(ratInset(ends[0]), h[2][j]))
			h[axis][j].Quo(h[axis][j], width)
		}
	}
	var powers [3][5]trimPower9
	for axis := range powers {
		powers[axis][0] = trimZero9()
		powers[axis][0][0].SetInt64(1)
		for i := 1; i <= 4; i++ {
			powers[axis][i] = trimMultiply9(powers[axis][i-1], h[axis])
		}
	}
	var composed [4]trimPower9
	for k := range composed {
		composed[k] = trimZero9()
	}
	for i := 0; i <= 2; i++ {
		for j := 0; j <= 2; j++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			term := trimMultiply9(trimMultiply9(powers[0][i], powers[1][j]), powers[2][4-i-j])
			for k := range composed {
				for d := range term {
					composed[k][d].Add(composed[k][d], new(big.Rat).Mul(coefficients[i][j][k], term[d]))
				}
			}
		}
	}
	var residual [3]trimPower9
	aa, bb := exactBoundPoint(start), exactBoundPoint(end)
	for k := range residual {
		chord := trimZero9()
		chord[0].Set(aa[k])
		chord[1].Sub(bb[k], aa[k])
		product := trimMultiply9(composed[3], chord)
		residual[k] = trimZero9()
		for d := range product {
			residual[k][d].Sub(composed[k][d], product[d])
		}
	}
	choose := func(n, k int) int64 {
		r := int64(1)
		for j := 1; j <= k; j++ {
			r = r * int64(n-j+1) / int64(j)
		}
		return r
	}
	bound := new(big.Rat)
	for i := 0; i <= 9; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		norm := new(big.Rat)
		for k := range residual {
			control := new(big.Rat)
			for j := 0; j <= i; j++ {
				control.Add(control, new(big.Rat).Mul(residual[k][j], big.NewRat(choose(i, j), choose(9, j))))
			}
			norm.Add(norm, new(big.Rat).Mul(control, control))
		}
		if norm.Cmp(bound) > 0 {
			bound = norm
		}
	}
	denominator := new(big.Rat).Mul(minCurve, minCurve)
	denominator.Mul(denominator, denominator)
	denominator.Mul(denominator, minPatch)
	denominator.Mul(denominator, denominator)
	return bound.Quo(bound, denominator), nil
}
