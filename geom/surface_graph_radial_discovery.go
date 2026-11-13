package geom

import (
	"context"
	"math/big"
)

// SplineGraphRadialPair certifies exactly two concentric circular zero contours
// in the original unit UV square. The positive rational denominator does not
// change this zero set. Radius and squared radii are caller-owned copies.
type SplineGraphRadialPair struct {
	Proven                bool
	Radius, RadiusSquared [2]*big.Rat
	Center                [2]*big.Rat
	Side                  int
	ExactOperations       int
}

// SplineGraphConcentricEllipses certifies two exact axis-aligned elliptic
// contours. Embedded Radius fields are X semiaxes; RadiusY stores Y semiaxes.
type SplineGraphConcentricEllipses struct {
	SplineGraphRadialPair
	// MetricY is positive in (u-Cu)^2+MetricY*(v-Cv)^2.
	MetricY *big.Rat
	RadiusY [2]*big.Rat
}

// DiscoverSplineGraphRadialPair retains the circular-only specialization;
// nonunit metric is rejected even when exact elliptic factors are proved.
func DiscoverSplineGraphRadialPair(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (SplineGraphRadialPair, error) {
	result, e := DiscoverSplineGraphConcentricEllipses(ctx, a, c, o)
	if e != nil {
		return result.SplineGraphRadialPair, e
	}
	if result.MetricY.Cmp(big.NewRat(1, 1)) != 0 {
		return SplineGraphRadialPair{ExactOperations: result.ExactOperations}, ErrConvergence
	}
	return result.SplineGraphRadialPair, nil
}

// DiscoverSplineGraphConcentricEllipses proves a diagonal positive quadratic
// metric times two distinct concentric factors. It requires rational X/Y radii,
// strict unit-domain containment and an exact whole-numerator identity.
func DiscoverSplineGraphConcentricEllipses(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphConcentricEllipses, err error) {
	if ctx == nil || a == nil || c == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	p, e := orderingDifference(ctx, a, c, 1, b)
	if e != nil {
		return r, e
	}
	if p.Domain() != (ParameterRectangle{0, 1, 0, 1}) || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return r, ErrConvergence
	}
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		v := make([]*big.Rat, p.countV)
		for j := range v {
			v[j] = p.net[i*p.countV+j][2]
		}
		rows[i], e = bernsteinToPower(v, b)
		if e != nil {
			return r, e
		}
	}
	coefficients := map[[2]int]*big.Rat{}
	for j := 0; j < p.countV; j++ {
		v := make([]*big.Rat, p.countU)
		for i := range v {
			v[i] = new(big.Rat)
			if j < len(rows[i]) {
				v[i].Set(rows[i][j])
			}
		}
		column, e := bernsteinToPower(v, b)
		if e != nil {
			return r, e
		}
		for i, q := range column {
			coefficients[[2]int{i, j}] = q
		}
	}
	get := func(i, j int) *big.Rat {
		if q := coefficients[[2]int{i, j}]; q != nil {
			return q
		}
		return new(big.Rat)
	}
	scale := get(4, 0)
	if scale.Sign() == 0 {
		return r, ErrConvergence
	}
	metric := new(big.Rat).Quo(get(2, 2), new(big.Rat).Mul(big.NewRat(2, 1), scale))
	if metric.Sign() <= 0 {
		return r, ErrConvergence
	}
	center := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Neg(get(3, 0)), new(big.Rat).Mul(big.NewRat(4, 1), scale)), new(big.Rat).Quo(new(big.Rat).Neg(get(0, 3)), new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(4, 1), scale), new(big.Rat).Mul(metric, metric)))}
	c0 := new(big.Rat).Add(new(big.Rat).Mul(center[0], center[0]), new(big.Rat).Mul(metric, new(big.Rat).Mul(center[1], center[1])))
	sum := new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), c0), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(center[0], center[0]))), new(big.Rat).Quo(get(2, 0), scale))
	product := new(big.Rat).Add(new(big.Rat).Sub(new(big.Rat).Quo(get(0, 0), scale), new(big.Rat).Mul(c0, c0)), new(big.Rat).Mul(sum, c0))
	radial := map[[2]int]*big.Rat{{0, 0}: c0, {1, 0}: new(big.Rat).Mul(big.NewRat(-2, 1), center[0]), {2, 0}: big.NewRat(1, 1), {0, 1}: new(big.Rat).Mul(metric, new(big.Rat).Mul(big.NewRat(-2, 1), center[1])), {0, 2}: metric}
	expected := map[[2]int]*big.Rat{}
	add := func(k [2]int, q *big.Rat) {
		if expected[k] == nil {
			expected[k] = new(big.Rat)
		}
		expected[k].Add(expected[k], q)
	}
	for i, x := range radial {
		for j, y := range radial {
			add([2]int{i[0] + j[0], i[1] + j[1]}, new(big.Rat).Mul(x, y))
		}
		add(i, new(big.Rat).Neg(new(big.Rat).Mul(sum, x)))
	}
	add([2]int{}, product)
	for i := 0; i < max(p.countU, 5); i++ {
		for j := 0; j < max(p.countV, 5); j++ {
			q := expected[[2]int{i, j}]
			if q == nil {
				q = new(big.Rat)
			}
			q = new(big.Rat).Mul(q, scale)
			if e = b.check(q); e != nil {
				return r, e
			}
			if q.Cmp(get(i, j)) != 0 {
				return r, ErrConvergence
			}
		}
	}
	sqrt := func(q *big.Rat) (*big.Rat, bool) {
		if q.Sign() < 0 {
			return nil, false
		}
		n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
		if new(big.Int).Mul(n, n).Cmp(q.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(q.Denom()) != 0 {
			return nil, false
		}
		return new(big.Rat).SetFrac(n, d), true
	}
	discriminant := new(big.Rat).Sub(new(big.Rat).Mul(sum, sum), new(big.Rat).Mul(big.NewRat(4, 1), product))
	for _, q := range []*big.Rat{sum, product, discriminant} {
		if e := b.check(q); e != nil {
			return r, e
		}
	}
	root, ok := sqrt(discriminant)
	if !ok || root.Sign() == 0 {
		return r, ErrConvergence
	}
	for i := 0; i < 2; i++ {
		q := new(big.Rat).Sub(sum, root)
		if i == 1 {
			q.Add(sum, root)
		}
		q.Quo(q, big.NewRat(2, 1))
		radius, ok := sqrt(q)
		if !ok || radius.Sign() <= 0 {
			return r, ErrConvergence
		}
		if e = b.check(radius); e != nil {
			return r, e
		}
		radiusY, ok := sqrt(new(big.Rat).Quo(q, metric))
		if !ok {
			return r, ErrConvergence
		}
		r.RadiusY[i] = radiusY
		for axis, coordinate := range center {
			actualRadius := radius
			if axis == 1 {
				actualRadius = radiusY
			}
			if new(big.Rat).Sub(coordinate, actualRadius).Sign() <= 0 || new(big.Rat).Add(coordinate, actualRadius).Cmp(big.NewRat(1, 1)) >= 0 {
				return r, ErrConvergence
			}
		}
		r.Radius[i], r.RadiusSquared[i] = radius, q
	}
	r.MetricY = metric
	r.Center = center
	r.Side = scale.Sign()
	r.Proven = true
	return r, nil
}
