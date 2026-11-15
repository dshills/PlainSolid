package geom

import (
	"context"
	"math/big"
)

// RationalCircleArcDiscoveryOptions bounds original conic reproof and exact
// rational base-point construction cumulatively. MaxSearchSteps defaults to
// 4096, maximum 65536. Failure is unresolved, never proof of absent contact.
type RationalCircleArcDiscoveryOptions struct {
	GraphPointContactOptions
	MaxSearchSteps int
}

// DiscoverSplineGraphRationalCircleArcs recognizes a complete original-chart
// quadratic circle and constructs its exact rational arcs without requiring a
// rational radius. For radius squared N/D it searches integers a²+b²=N*D,
// yielding the exact point center+(a/D,b/D). This is a bounded integer search,
// not rounded UV sampling. Both original supports and the resulting rational
// point are independently re-proved by the complete conic arc constructor.
// Unequal ellipse axes, nonquadratic first cells and exhausted search reject.
func DiscoverSplineGraphRationalCircleArcs(ctx context.Context, a, c *SurfacePatch, o RationalCircleArcDiscoveryOptions) (r SplineGraphQuadraticEllipseArcs, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphQuadraticEllipseArcs{}
		}
		r.ExactOperations = used
	}()
	if ctx == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxSearchSteps == 0 {
		o.MaxSearchSteps = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSearchSteps < 1 || o.MaxSearchSteps > 65536 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	certificate, e := DiscoverSplineGraphQuadraticEllipse(ctx, a, c, o.GraphPointContactOptions)
	used = certificate.ExactOperations
	if e != nil {
		return r, e
	}
	q := certificate.Ellipse.Coefficients
	if !certificate.Proven || q[5].Sign() != 0 || q[2].Sign() <= 0 || q[2].Cmp(q[4]) != 0 {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: used}
	// Check each derived rational before further arithmetic or integer search.
	op := func(kind byte, x, y *big.Rat) (*big.Rat, error) {
		z := new(big.Rat)
		switch kind {
		case '+':
			z.Add(x, y)
		case '*':
			z.Mul(x, y)
		case '/':
			z.Quo(x, y)
		}
		e := budget.check(z)
		used = budget.operations
		return z, e
	}
	x, e := op('*', q[1], certificate.Center[0])
	if e != nil {
		return r, e
	}
	y, e := op('*', q[3], certificate.Center[1])
	if e != nil {
		return r, e
	}
	sum, e := op('+', x, y)
	if e != nil {
		return r, e
	}
	half, e := op('/', sum, big.NewRat(2, 1))
	if e != nil {
		return r, e
	}
	minimum, e := op('+', q[0], half)
	if e != nil {
		return r, e
	}
	radiusSquared, e := op('/', new(big.Rat).Neg(minimum), q[2])
	if e != nil {
		return r, e
	}
	if radiusSquared.Sign() <= 0 {
		return r, ErrConvergence
	}
	n, d := radiusSquared.Num(), radiusSquared.Denom()
	product := new(big.Int).Mul(n, d)
	if e = budget.check(new(big.Rat).SetInt(product)); e != nil {
		used = budget.operations
		return r, e
	}
	lo, hi := new(big.Int), new(big.Int).Sqrt(product)
	if e = budget.check(new(big.Rat).SetInt(hi)); e != nil {
		used = budget.operations
		return r, e
	}
	one := big.NewInt(1)
	found := false
	for step := 0; step < o.MaxSearchSteps && lo.Cmp(hi) <= 0; step++ {
		if e = ctx.Err(); e != nil {
			used = budget.operations
			return r, e
		}
		aa, bb := new(big.Int).Mul(lo, lo), new(big.Int).Mul(hi, hi)
		total := new(big.Int).Add(aa, bb)
		for _, v := range []*big.Int{aa, bb, total} {
			if e = budget.check(new(big.Rat).SetInt(v)); e != nil {
				used = budget.operations
				return r, e
			}
		}
		comparison := total.Cmp(product)
		if comparison == 0 {
			found = true
			break
		}
		if comparison < 0 {
			lo.Add(lo, one)
		} else {
			hi.Sub(hi, one)
		}
	}
	used = budget.operations
	if !found {
		return r, ErrConvergence
	}
	point := [2]*big.Rat{}
	for axis, v := range []*big.Int{lo, hi} {
		point[axis], e = op('+', certificate.Center[axis], new(big.Rat).SetFrac(v, d))
		if e != nil {
			return r, e
		}
	}
	remaining := o.MaxOperations - used
	if remaining < 1 {
		return r, ErrConvergence
	}
	proof := o.GraphPointContactOptions
	proof.MaxOperations = remaining
	arcs, e := BuildSplineGraphQuadraticEllipseArcs(ctx, a, c, certificate.Ellipse, point, proof)
	used += arcs.ExactOperations
	if e != nil {
		return r, e
	}
	if !arcs.Complete {
		return r, ErrConvergence
	}
	return arcs, nil
}
