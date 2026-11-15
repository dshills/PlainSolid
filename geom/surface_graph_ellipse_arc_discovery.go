package geom

import (
	"context"
	"math/big"
)

// RationalEllipseArcDiscoveryOptions bounds exact original-support reproof and
// rational direction construction cumulatively. MaxDirectionCoordinate defaults
// to 8 and is at most 64. Exhaustion is unresolved, never absence of contact.
type RationalEllipseArcDiscoveryOptions struct {
	GraphPointContactOptions
	MaxDirectionCoordinate int
}

// DiscoverSplineGraphRationalEllipseArcs constructs a rational point on a proved
// positive quadratic ellipse by intersecting primitive integer directions through
// its exact center. Only exact rational square roots are accepted. The complete
// original supports and all four arcs are independently re-proved afterward.
// This bounded search need not find every ellipse having a rational point.
func DiscoverSplineGraphRationalEllipseArcs(ctx context.Context, a, c *SurfacePatch, o RationalEllipseArcDiscoveryOptions) (r SplineGraphQuadraticEllipseArcs, err error) {
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
	if o.MaxDirectionCoordinate == 0 {
		o.MaxDirectionCoordinate = 8
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDirectionCoordinate < 1 || o.MaxDirectionCoordinate > 64 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	certificate, e := DiscoverSplineGraphQuadraticEllipse(ctx, a, c, o.GraphPointContactOptions)
	used = certificate.ExactOperations
	if e != nil {
		return r, e
	}
	q := certificate.Ellipse.Coefficients
	if !certificate.Proven || q[2].Sign() <= 0 {
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
	level := new(big.Rat).Neg(minimum)
	if level.Sign() <= 0 {
		return r, ErrConvergence
	}
	var point [2]*big.Rat
	found := false
	// Canonical primitive directions avoid duplicate opposite rays. A successful
	// square test constructs a point exactly; unsuccessful search proves nothing.
	for bound := 1; bound <= o.MaxDirectionCoordinate && !found; bound++ {
		for i := -bound; i <= bound && !found; i++ {
			for j := 0; j <= bound && !found; j++ {
				if j == 0 && i != 1 {
					continue
				}
				abs := i
				if abs < 0 {
					abs = -abs
				}
				if abs != bound && j != bound {
					continue
				}
				if new(big.Int).GCD(nil, nil, big.NewInt(int64(abs)), big.NewInt(int64(j))).Cmp(big.NewInt(1)) != 0 {
					continue
				}
				if e = ctx.Err(); e != nil {
					return r, e
				}
				terms := [3]*big.Rat{}
				for k, pair := range [][2]*big.Rat{{q[2], big.NewRat(int64(i*i), 1)}, {q[5], big.NewRat(int64(i*j), 1)}, {q[4], big.NewRat(int64(j*j), 1)}} {
					terms[k], e = op('*', pair[0], pair[1])
					if e != nil {
						return r, e
					}
				}
				norm, e := op('+', terms[0], terms[1])
				if e != nil {
					return r, e
				}
				norm, e = op('+', norm, terms[2])
				if e != nil {
					return r, e
				}
				if norm.Sign() <= 0 {
					return r, ErrConvergence
				}
				squared, e := op('/', level, norm)
				if e != nil {
					return r, e
				}
				n, d := new(big.Int).Sqrt(squared.Num()), new(big.Int).Sqrt(squared.Denom())
				nn, dd := new(big.Int).Mul(n, n), new(big.Int).Mul(d, d)
				for _, v := range []*big.Int{n, d, nn, dd} {
					e = budget.check(new(big.Rat).SetInt(v))
					used = budget.operations
					if e != nil {
						return r, e
					}
				}
				if nn.Cmp(squared.Num()) != 0 || dd.Cmp(squared.Denom()) != 0 {
					continue
				}
				lambda := new(big.Rat).SetFrac(n, d)
				for axis, v := range []int{i, j} {
					offset, e := op('*', lambda, big.NewRat(int64(v), 1))
					if e != nil {
						return r, e
					}
					point[axis], e = op('+', certificate.Center[axis], offset)
					if e != nil {
						return r, e
					}
				}
				found = true
			}
		}
	}
	if !found {
		return r, ErrConvergence
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
