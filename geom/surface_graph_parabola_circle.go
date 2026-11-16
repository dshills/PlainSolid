package geom

import (
	"context"
	"math/big"
	"slices"
)

// SplineGraphParabolaCircle proves a complete regular open parabola and
// a separate interior circle on the original common graph chart. Controls
// are the exact quadratic Bézier points in original UV. RegionSides gives
// the A-minus-B height signs below and above the parabola outside the circle,
// then inside the circle. Above and below refer to the dependent coordinate.
type SplineGraphParabolaCircle struct {
	FreeAxis         int
	ParabolaControls [3][2]*big.Rat
	CircleCenter     [2]*big.Rat
	RadiusSquared    *big.Rat
	CircleAbove      bool
	RegionSides      [3]int
	Graph            SurfaceInjectivity
	ExactOperations  int
}

// DiscoverSplineGraphParabolaCircle recovers the full homogeneous difference
// K*(y-p(x))*((x-cx)^2+(y-cy)^2-r²), with x on freeAxis. It checks all power
// coefficients of one original unit-domain residual cell and proves strict
// circle containment and separation from the entire open parabola by rational
// bounds. Both curves are simple and the parabola is transverse to the stock
// sides at its two exits. A second residual cell is currently unsupported.
func DiscoverSplineGraphParabolaCircle(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaCircle, error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || freeAxis < 0 || freeAxis > 1 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := alignedPolynomialProofOptions(o)
	if err != nil {
		return nil, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	diff, err := orderingDifference(ctx, a, c, o.MaxCells, b)
	if err != nil {
		return nil, err
	}
	if diff.Domain() != (ParameterRectangle{UMax: 1, VMax: 1}) || diff.countU != diff.degreeU+1 || diff.countV != diff.degreeV+1 ||
		diff.degreeU > 6 || diff.degreeV > 6 {
		return nil, ErrConvergence
	}
	law, err := parabolaCirclePowerColumns(diff, freeAxis, b)
	if err != nil {
		return nil, err
	}
	p, center, radiusSquared, sign, err := factorParabolaCircle(law, b)
	if err != nil {
		return nil, err
	}
	out := &SplineGraphParabolaCircle{FreeAxis: freeAxis, RadiusSquared: radiusSquared, RegionSides: [3]int{-sign, sign, sign}}
	for axis, x := range center {
		out.CircleCenter[(freeAxis+axis)%2] = x
	}
	one := big.NewRat(1, 1)
	ordinates := [3]*big.Rat{
		p[0],
		new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], big.NewRat(2, 1))),
		new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2])),
	}
	for i, q := range ordinates {
		if err := b.check(q); err != nil {
			return nil, err
		}
		if q.Sign() <= 0 || q.Cmp(one) >= 0 {
			return nil, ErrConvergence
		}
		out.ParabolaControls[i][freeAxis] = big.NewRat(int64(i), 2)
		out.ParabolaControls[i][1-freeAxis] = q
	}
	minimum, maximum := ordinates[0], ordinates[0]
	for _, x := range ordinates[1:] {
		if x.Cmp(minimum) < 0 {
			minimum = x
		}
		if x.Cmp(maximum) > 0 {
			maximum = x
		}
	}
	above := new(big.Rat).Sub(center[1], maximum)
	below := new(big.Rat).Sub(minimum, center[1])
	out.CircleAbove = above.Sign() > 0
	gap := below
	if out.CircleAbove {
		out.RegionSides[2] = -sign
		gap = above
	}
	// Each distance to an enclosing side and the separation to the
	// parabola's entire convex hull must exceed the circular radius.
	for _, d := range []*big.Rat{center[0], center[1], new(big.Rat).Sub(one, center[0]), new(big.Rat).Sub(one, center[1]), gap} {
		square := new(big.Rat).Mul(d, d)
		if err := b.check(square); err != nil {
			return nil, err
		}
		if d.Sign() <= 0 || square.Cmp(radiusSquared) <= 0 {
			return nil, ErrConvergence
		}
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	out.Graph, err = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1},
		MaxCells: o.MaxCells, MaxDepth: o.MaxDepth, MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	if err != nil {
		return nil, err
	}
	if !out.Graph.ProvenInjective {
		return nil, ErrConvergence
	}
	out.ExactOperations = b.operations + out.Graph.ExactOperations
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parabolaCirclePowerColumns(diff *SurfacePatch, freeAxis int, b *polynomialBudget) ([4]rationalPolynomial, error) {
	var out [4]rationalPolynomial
	cols, err := graphNumeratorPowerColumns(diff, b)
	if err != nil {
		return out, err
	}
	for j := range out {
		out[j] = make(rationalPolynomial, 5)
		for i := range out[j] {
			out[j][i] = new(big.Rat)
		}
	}
	for j, column := range cols {
		for i, coefficient := range column {
			if coefficient.Sign() == 0 {
				continue
			}
			x, y := i, j
			if freeAxis == 1 {
				x, y = j, i
			}
			if x > 4 || y > 3 {
				return out, ErrConvergence
			}
			out[y][x].Set(coefficient)
		}
	}
	for i := range out {
		out[i] = trimPolynomial(out[i])
	}
	return out, nil
}

func factorParabolaCircle(law [4]rationalPolynomial, b *polynomialBudget) (parabola rationalPolynomial, center [2]*big.Rat, radiusSquared *big.Rat, sign int, err error) {
	if len(law[3]) != 1 || len(law[2]) != 3 || len(law[1]) > 3 {
		return nil, center, nil, 0, ErrConvergence
	}
	scale := law[3][0]
	sign = scale.Sign()
	for _, column := range law[:3] {
		for _, x := range column {
			x.Quo(x, scale)
			if err = b.check(x); err != nil {
				return
			}
		}
	}
	// v² gives h2=-2*cy-p. The u² coefficient of v is
	// 1-2*cy*h2[2], and h2[2] is nonzero for a true parabola.
	one, two := big.NewRat(1, 1), big.NewRat(2, 1)
	h12 := new(big.Rat)
	if len(law[1]) == 3 {
		h12.Set(law[1][2])
	}
	twiceCY := new(big.Rat).Quo(new(big.Rat).Sub(one, h12), law[2][2])
	center[1] = new(big.Rat).Quo(twiceCY, two)
	parabola = rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Add(law[2][0], twiceCY)),
		new(big.Rat).Neg(law[2][1]), new(big.Rat).Neg(law[2][2])}
	cross, err := polynomialProduct(parabola, rationalPolynomial{twiceCY}, b)
	if err != nil {
		return
	}
	quadratic, err := graphPolynomialSum(law[1], cross, -1, b)
	if err != nil {
		return
	}
	if len(quadratic) != 3 || quadratic[2].Cmp(one) != 0 {
		return nil, center, nil, 0, ErrConvergence
	}
	center[0] = new(big.Rat).Quo(new(big.Rat).Neg(quadratic[1]), two)
	radiusSquared = new(big.Rat).Add(new(big.Rat).Mul(center[0], center[0]), new(big.Rat).Mul(center[1], center[1]))
	radiusSquared.Sub(radiusSquared, quadratic[0])
	if radiusSquared.Sign() <= 0 {
		return nil, center, nil, 0, ErrConvergence
	}
	for _, x := range []*big.Rat{center[0], center[1], radiusSquared} {
		if err = b.check(x); err != nil {
			return
		}
	}
	constant, err := polynomialProduct(parabola, quadratic, b)
	if err != nil {
		return
	}
	if !slices.EqualFunc(law[0], constant, func(x, y *big.Rat) bool { return new(big.Rat).Add(x, y).Sign() == 0 }) {
		return nil, center, nil, 0, ErrConvergence
	}
	return
}
