package geom

import (
	"context"
	"math/big"
	"slices"
)

// GraphContactCircle stores exact squared radius and original chart center.
type GraphContactCircle struct {
	Center        [2]*big.Rat
	RadiusSquared *big.Rat
}

// SplineGraphParabolaTwoCircles proves the complete zero set of two original
// graphs. The circles are ordered by increasing free center and lie on the
// same side of the regular wall-to-wall parabola. RegionSides gives A-minus-B
// for the lower open side, upper open side, and both interiors in circle order.
type SplineGraphParabolaTwoCircles struct {
	FreeAxis         int
	ParabolaControls [3][2]*big.Rat
	Circles          [2]GraphContactCircle
	CirclesAbove     bool
	RegionSides      [4]int
	Graph            SurfaceInjectivity
	ExactOperations  int
}

// DiscoverSplineGraphParabolaTwoCircles recovers a complete factorization
// K*(y-p(x))*((x-c0)^2+(y-d)^2-r0²)*((x-c1)^2+(y-d)^2-r1²) on one
// common original unit-domain residual cell (tensor degree at most eight).
// The two circular centers must have the same dependent coordinate,
// the radii can differ, and all three contacts must be strictly separated.
// Positivity of the original homogeneous denominator retains the signs.
// No factor, endpoint or contact proposal is supplied by the caller.
func DiscoverSplineGraphParabolaTwoCircles(ctx context.Context, a, c *SurfacePatch, freeAxis int,
	o GraphPointContactOptions) (*SplineGraphParabolaTwoCircles, error) {
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
	if diff.Domain() != (ParameterRectangle{UMax: 1, VMax: 1}) || diff.countU != diff.degreeU+1 ||
		diff.countV != diff.degreeV+1 || diff.degreeU > 8 || diff.degreeV > 8 {
		return nil, ErrConvergence
	}
	law, err := graphMixedPowerColumns(diff, freeAxis, 6, 5, b)
	if err != nil {
		return nil, err
	}
	parabola, circles, sign, err := factorParabolaTwoCircles(law, b)
	if err != nil {
		return nil, err
	}
	out := &SplineGraphParabolaTwoCircles{FreeAxis: freeAxis, Circles: circles, RegionSides: [4]int{-sign, sign, sign, sign}}
	out.CirclesAbove, err = bindSeparatedParabolaCircles(out, parabola, b)
	if err != nil {
		return nil, err
	}
	if out.CirclesAbove {
		out.RegionSides[2], out.RegionSides[3] = -sign, -sign
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

func factorParabolaTwoCircles(law []rationalPolynomial, b *polynomialBudget) (
	parabola rationalPolynomial, circles [2]GraphContactCircle, sign int, err error) {
	if len(law[5]) != 1 || len(law[4]) != 3 || len(law[3]) > 3 {
		return nil, circles, 0, ErrConvergence
	}
	scale := law[5][0]
	sign = scale.Sign()
	for _, column := range law[:5] {
		for _, x := range column {
			x.Quo(x, scale)
			if err = b.check(x); err != nil {
				return
			}
		}
	}
	// In the monic residual, h4=-4*d-p, while h3 has u²
	// coefficient 2-4*d*h4[u²]. The curvature makes recovery unique.
	h32 := new(big.Rat)
	if len(law[3]) == 3 {
		h32.Set(law[3][2])
	}
	fourD := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(2, 1), h32), law[4][2])
	d := new(big.Rat).Quo(fourD, big.NewRat(4, 1))
	if err = b.check(d); err != nil {
		return
	}
	parabola = rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Add(law[4][0], fourD)),
		new(big.Rat).Neg(law[4][1]), new(big.Rat).Neg(law[4][2])}

	// Synthetic division in y by y-p(x), including the full constant
	// remainder. The quotient is stored in ascending powers of y.
	var quotient [5]rationalPolynomial
	quotient[4] = rationalPolynomial{big.NewRat(1, 1)}
	for i := 4; i > 0; i-- {
		p, e := polynomialProduct(parabola, quotient[i], b)
		if e != nil {
			return nil, circles, 0, e
		}
		quotient[i-1], e = graphPolynomialSum(law[i], p, 1, b)
		if e != nil {
			return nil, circles, 0, e
		}
	}
	product, err := polynomialProduct(parabola, quotient[0], b)
	if err != nil {
		return
	}
	remainder, err := graphPolynomialSum(law[0], product, 1, b)
	if err != nil {
		return
	}
	if len(remainder) != 0 {
		return nil, circles, 0, ErrConvergence
	}
	circles, err = factorCircleQuartic(quotient, d, b)
	return
}

func factorCircleQuartic(q [5]rationalPolynomial, d *big.Rat, b *polynomialBudget) (circles [2]GraphContactCircle, err error) {
	// q = (y-d)^4 + S(u)*(y-d)^2 + P(u), where S and P
	// are respectively the sum and product of the two quadratic offsets.
	d2 := new(big.Rat).Mul(d, d)
	d4 := new(big.Rat).Mul(d2, d2)
	sum, err := graphPolynomialSum(q[2], rationalPolynomial{new(big.Rat).Mul(big.NewRat(6, 1), d2)}, -1, b)
	if err != nil {
		return circles, err
	}
	if len(sum) != 3 || sum[2].Cmp(big.NewRat(2, 1)) != 0 {
		return circles, ErrConvergence
	}
	linear, err := polynomialProduct(sum, rationalPolynomial{new(big.Rat).Mul(big.NewRat(-2, 1), d)}, b)
	if err != nil {
		return circles, err
	}
	linear, err = graphPolynomialSum(linear, rationalPolynomial{new(big.Rat).Mul(big.NewRat(-4, 1), new(big.Rat).Mul(d2, d))}, 1, b)
	if err != nil {
		return circles, err
	}
	if !slices.EqualFunc(linear, q[1], func(a, c *big.Rat) bool { return a.Cmp(c) == 0 }) {
		return circles, ErrConvergence
	}
	offset, err := polynomialProduct(sum, rationalPolynomial{d2}, b)
	if err != nil {
		return circles, err
	}
	offset, err = graphPolynomialSum(offset, rationalPolynomial{d4}, 1, b)
	if err != nil {
		return circles, err
	}
	product, err := graphPolynomialSum(q[0], offset, -1, b)
	if err != nil {
		return circles, err
	}
	discriminant, err := polynomialProduct(sum, sum, b)
	if err != nil {
		return circles, err
	}
	product4, err := polynomialProduct(product, rationalPolynomial{big.NewRat(4, 1)}, b)
	if err != nil {
		return circles, err
	}
	discriminant, err = graphPolynomialSum(discriminant, product4, -1, b)
	if err != nil {
		return circles, err
	}
	delta, err := graphExactPolynomialSqrt(discriminant, b)
	if err != nil {
		return circles, err
	}
	if len(delta) != 2 {
		return circles, ErrConvergence
	}
	// Positive leading coefficient of the square root makes the plus
	// branch the smaller free center. The quadratic coefficient is one.
	for i, side := range [2]int{1, -1} {
		p, err := graphPolynomialSum(sum, delta, side, b)
		if err != nil {
			return circles, err
		}
		x := new(big.Rat).Quo(new(big.Rat).Neg(p[1]), big.NewRat(4, 1))
		r2 := new(big.Rat).Sub(new(big.Rat).Mul(x, x), new(big.Rat).Quo(p[0], big.NewRat(2, 1)))
		for _, value := range []*big.Rat{x, r2} {
			if err := b.check(value); err != nil {
				return circles, err
			}
		}
		if r2.Sign() <= 0 {
			return circles, ErrConvergence
		}
		circles[i] = GraphContactCircle{Center: [2]*big.Rat{x, new(big.Rat).Set(d)}, RadiusSquared: r2}
	}
	return circles, nil
}

func bindSeparatedParabolaCircles(out *SplineGraphParabolaTwoCircles, p rationalPolynomial, b *polynomialBudget) (bool, error) {
	one := big.NewRat(1, 1)
	controls := [3]*big.Rat{
		p[0],
		new(big.Rat).Add(p[0], new(big.Rat).Quo(p[1], big.NewRat(2, 1))),
		new(big.Rat).Add(p[0], new(big.Rat).Add(p[1], p[2])),
	}
	minimum, maximum := controls[0], controls[0]
	for i, q := range controls {
		if err := b.check(q); err != nil {
			return false, err
		}
		if q.Sign() <= 0 || q.Cmp(one) >= 0 {
			return false, ErrConvergence
		}
		out.ParabolaControls[i][out.FreeAxis] = big.NewRat(int64(i), 2)
		out.ParabolaControls[i][1-out.FreeAxis] = q
		if q.Cmp(minimum) < 0 {
			minimum = q
		}
		if q.Cmp(maximum) > 0 {
			maximum = q
		}
	}
	first, second := out.Circles[0], out.Circles[1]
	separation := new(big.Rat).Sub(second.Center[0], first.Center[0])
	margin := new(big.Rat).Mul(separation, separation)
	margin.Sub(margin, first.RadiusSquared).Sub(margin, second.RadiusSquared)
	cross := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(first.RadiusSquared, second.RadiusSquared))
	square := new(big.Rat).Mul(margin, margin)
	if err := b.check(square); err != nil {
		return false, err
	}
	if err := b.check(cross); err != nil {
		return false, err
	}
	if separation.Sign() <= 0 || margin.Sign() <= 0 || square.Cmp(cross) <= 0 {
		return false, ErrConvergence
	}
	y := first.Center[1]
	gap := new(big.Rat).Sub(y, maximum)
	above := gap.Sign() > 0
	if !above {
		gap.Sub(minimum, y)
	}
	for i, circle := range out.Circles {
		x := circle.Center[0]
		for _, distance := range []*big.Rat{x, y, new(big.Rat).Sub(one, x), new(big.Rat).Sub(one, y), gap} {
			square := new(big.Rat).Mul(distance, distance)
			if err := b.check(square); err != nil {
				return false, err
			}
			if distance.Sign() <= 0 || square.Cmp(circle.RadiusSquared) <= 0 {
				return false, ErrConvergence
			}
		}
		out.Circles[i].Center[out.FreeAxis] = x
		out.Circles[i].Center[1-out.FreeAxis] = circle.Center[1]
	}
	return above, nil
}
