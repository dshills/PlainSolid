package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// composedGraphPlaneTrim solves the complete homogeneous plane equation
// A(s)+B(s)t on a tensor span. A strict positive control hull for the
// denominator proves one regular dependent coordinate on the original chart.
// Unlike the affine-projection path, the world curve itself may be rational.
func composedGraphPlaneTrim(p *geom.SurfacePatch, plane polynomialPlaneEquation, free int) (PCurve, int, error) {
	if free != 0 && free != 1 {
		return nil, 0, geom.ErrInvalid
	}
	h, _, ok := p.RationalBezierCoefficients(4)
	if !ok || len(h) == 0 {
		return nil, 0, geom.ErrConvergence
	}
	if free == 1 {
		h = transposePolynomialPlanePower(h)
	}
	if len(h[0]) < 2 {
		return nil, 0, geom.ErrConvergence
	}
	a, b := make([]*big.Rat, len(h)), make([]*big.Rat, len(h))
	degree := 2
	for i, row := range h {
		for j, c := range row {
			x := new(big.Rat).Sub(plane.dot([3]*big.Rat{c[0], c[1], c[2]}), new(big.Rat).Mul(c[3], plane.offset))
			switch {
			case j == 0:
				a[i] = x.Neg(x)
				if x.Sign() != 0 {
					degree = max(degree, i)
				}
			case j == 1:
				b[i] = x
				if x.Sign() != 0 {
					degree = max(degree, i+1)
				}
			case x.Sign() != 0:
				return nil, 0, geom.ErrConvergence
			}
		}
	}
	sign := b[0].Sign()
	if sign == 0 || degree > 4 {
		return nil, 0, geom.ErrConvergence
	}
	if sign < 0 {
		for i := range a {
			a[i].Neg(a[i])
			b[i].Neg(b[i])
		}
	}
	// When A is proportional to B the true section is a straight UV
	// isocurve; remove the positive common factor before composition.
	ratio := new(big.Rat).Quo(a[0], b[0])
	constant := true
	for i := range a {
		constant = constant && a[i].Cmp(new(big.Rat).Mul(ratio, b[i])) == 0
	}
	// Prove B positive even in the proportional case; cancelling a zero
	// factor would silently remove an entire coplanar source fiber.
	for i := 0; i <= degree; i++ {
		v := new(big.Rat)
		for j := 0; j <= i && j < len(b); j++ {
			v.Add(v, new(big.Rat).Mul(b[j], big.NewRat(polynomialChoose(i, j), polynomialChoose(degree, j))))
		}
		if v.Sign() <= 0 {
			return nil, 0, geom.ErrConvergence
		}
	}
	if constant {
		a, b, degree = []*big.Rat{ratio}, []*big.Rat{big.NewRat(1, 1)}, 2
	}
	// Keep the established quadratic/quartic UV representation.
	if degree == 3 {
		degree = 4
	}
	d := p.Domain()
	lo, hi := [2]*big.Rat{ratInset(d.UMin), ratInset(d.VMin)}, [2]*big.Rat{ratInset(d.UMax), ratInset(d.VMax)}
	width := [2]*big.Rat{new(big.Rat).Sub(hi[0], lo[0]), new(big.Rat).Sub(hi[1], lo[1])}
	controls := make([][3]*big.Rat, degree+1)
	for i := range controls {
		power := func(row []*big.Rat, shift int) *big.Rat {
			v := new(big.Rat)
			for j := 0; j+shift <= i && j < len(row); j++ {
				n := j + shift
				v.Add(v, new(big.Rat).Mul(row[j], big.NewRat(polynomialChoose(i, n), polynomialChoose(degree, n))))
			}
			return v
		}
		w, s, t := power(b, 0), power(b, 1), power(a, 0)
		controls[i][free] = new(big.Rat).Add(new(big.Rat).Mul(lo[free], w), new(big.Rat).Mul(width[free], s))
		controls[i][1-free] = new(big.Rat).Add(new(big.Rat).Mul(lo[1-free], w), new(big.Rat).Mul(width[1-free], t))
		controls[i][2] = w
	}
	if !strictComposedGraphControls(controls, free, lo[1-free], hi[1-free], 0) {
		return nil, 0, geom.ErrConvergence
	}
	if degree == 2 {
		q, e := NewRationalEndpointQuadraticUV([3][3]*big.Rat{controls[0], controls[1], controls[2]})
		return q, sign, e
	}
	q, e := NewExactBezierUV(controls)
	return q, sign, e
}

func strictComposedGraphControls(h [][3]*big.Rat, free int, lo, hi *big.Rat, depth int) bool {
	inside := func(p [3]*big.Rat) bool {
		return p[2].Sign() > 0 && p[1-free].Cmp(new(big.Rat).Mul(lo, p[2])) > 0 && p[1-free].Cmp(new(big.Rat).Mul(hi, p[2])) < 0
	}
	if !inside(h[0]) || !inside(h[len(h)-1]) {
		return false
	}
	ok := true
	for _, p := range h[1 : len(h)-1] {
		ok = ok && inside(p)
	}
	if ok {
		return true
	}
	if depth >= 8 {
		return false
	}
	a, b, e := splitCapBezier(h, big.NewRat(1, 2), 16384)
	return e == nil && strictComposedGraphControls(a, free, lo, hi, depth+1) && strictComposedGraphControls(b, free, lo, hi, depth+1)
}

func composeProjectedGraphSectionControls(ctx context.Context, patch *geom.SurfacePatch, c PCurve, axes [2]int, limits geom.SurfacePatchOptions) ([][3]*big.Rat, error) {
	curve, e := polynomialTrimDescriptor(ctx, c, false, limits)
	if e != nil {
		return nil, e
	}
	cells, e := patch.BuildSurfaceCurveCompositionCells(ctx, curve, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: limits})
	if e != nil {
		return nil, e
	}
	if !cells.Complete || len(cells.Cells) != 1 {
		return nil, geom.ErrConvergence
	}
	power, _, e := cells.Cells[0].HomogeneousPower(ctx, geom.SurfacePatchEvaluationOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	// When the control hull is already inside one original knot cell,
	// prefer the reduced positive homogeneous net. This cancels common
	// factors such as the homogeneous denominator of an axial graph.
	if reduced, composedErr := patch.ComposeParameterCurve(ctx, curve, limits); composedErr == nil {
		if span, _, e := reduced.BezierControlNet(ctx, limits); e == nil && len(span.Control) >= 2 && len(span.Control) <= 5 {
			h := make([][3]*big.Rat, len(span.Control))
			for i, c := range span.Control {
				h[i] = [3]*big.Rat{c[axes[0]], c[axes[1]], c[3]}
			}
			return h, nil
		}
	}
	degree := 2
	for _, axis := range []int{axes[0], axes[1], 3} {
		for i := len(power[axis]) - 1; i > degree; i-- {
			if power[axis][i].Sign() != 0 {
				degree = 4
				if i > degree {
					return nil, geom.ErrConvergence
				}
				break
			}
		}
	}
	bits := limits.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	h := make([][3]*big.Rat, degree+1)
	for i := range h {
		for j, axis := range []int{axes[0], axes[1], 3} {
			x := new(big.Rat)
			for k := 0; k <= i && k < len(power[axis]); k++ {
				x.Add(x, new(big.Rat).Mul(power[axis][k], big.NewRat(polynomialChoose(i, k), polynomialChoose(degree, k))))
			}
			if x.Num().BitLen() > bits || x.Denom().BitLen() > bits || j == 2 && x.Sign() <= 0 {
				return nil, geom.ErrConvergence
			}
			h[i][j] = x
		}
	}
	return h, nil
}
