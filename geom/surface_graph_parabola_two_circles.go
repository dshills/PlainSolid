package geom

import (
	"context"
	"math/big"
)

// GraphContactCircle stores exact squared radius and original chart center.
type GraphContactCircle struct {
	Center        [2]*big.Rat
	RadiusSquared *big.Rat
}

// SplineGraphParabolaTwoCircles proves the complete zero set of two original
// graphs. Circle centers are ordered by free coordinate and then by dependent
// coordinate. Both circles lie on the same side of the regular wall-to-wall parabola. RegionSides gives A-minus-B
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
// K*(y-p(x))*((x-c0)^2+(y-d0)^2-r0²)*((x-c1)^2+(y-d1)^2-r1²) on one
// common original unit-domain residual cell (tensor degree at most eight).
// Both circles must be strictly interior, mutually separated, and on the
// same side of the parabola. Their centers and radii may vary independently.
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
	// The monic residual has h4=-2*(d0+d1)-p. Its h3[u²]
	// coefficient is 2-2*(d0+d1)*h4[u²], even for different ordinates.
	// The nonzero curvature makes recovery unique.
	h32 := new(big.Rat)
	if len(law[3]) == 3 {
		h32.Set(law[3][2])
	}
	twiceCenterSum := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(2, 1), h32), law[4][2])
	if err = b.check(twiceCenterSum); err != nil {
		return
	}
	parabola = rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Add(law[4][0], twiceCenterSum)),
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
	circles, err = factorCircleQuartic(quotient, b)
	return
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
	var separationSquared big.Rat
	for axis := range 2 {
		delta := new(big.Rat).Sub(second.Center[axis], first.Center[axis])
		separationSquared.Add(&separationSquared, delta.Mul(delta, delta))
	}
	margin := new(big.Rat).Set(&separationSquared)
	margin.Sub(margin, first.RadiusSquared).Sub(margin, second.RadiusSquared)
	cross := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(first.RadiusSquared, second.RadiusSquared))
	square := new(big.Rat).Mul(margin, margin)
	if err := b.check(square); err != nil {
		return false, err
	}
	if err := b.check(cross); err != nil {
		return false, err
	}
	if margin.Sign() <= 0 || square.Cmp(cross) <= 0 {
		return false, ErrConvergence
	}
	above := first.Center[1].Cmp(maximum) > 0
	for i, circle := range out.Circles {
		x, y := circle.Center[0], circle.Center[1]
		gap := new(big.Rat).Sub(minimum, y)
		if above {
			gap.Sub(y, maximum)
		}
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
