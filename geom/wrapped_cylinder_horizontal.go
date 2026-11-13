package geom

import (
	"context"
	"math/big"
	"sort"
)

// IntersectWrappedCylinderHorizontalLine intersects a represented horizontal
// finite line with the capped canonical cylinder for arbitrary positive
// rational profile weights. Nonhorizontal/zero directions fail explicitly.
// Each quadratic conic is clipped to its exact control triangle before root
// isolation; this excludes the unused branches of the full implicit conic.
// Shared quadrant vertices are exact rational roots and deduplicated. On a cap
// plane, the convex disk intersection is an explicit boundary overlap with
// algebraic endpoint brackets. Tangency remains a single point, not an interval.
// MaxSubdivisions applies per quadrant (at most four); arithmetic work shares
// one cumulative Surface budget. Each positive-weight quadratic arc is convex
// with the canonical corner tangents, so the capped disk is globally convex.
func IntersectWrappedCylinderHorizontalLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (CylinderLineIntersections, error) {
	if !finite(minT) || !finite(maxT) || minT > maxT || direction.Z != 0 {
		return CylinderLineIntersections{}, ErrInvalid
	}
	return intersectCylinderProfileLine(ctx, w, origin, direction, new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT), o, false)
}

func intersectCylinderProfileLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT *big.Rat, o ToroidLineOptions, ignoreHeight bool) (CylinderLineIntersections, error) {
	empty := CylinderLineIntersections{}
	if !origin.IsFinite() || !direction.IsFinite() || (direction.X == 0 && direction.Y == 0) || minT == nil || maxT == nil || minT.Cmp(maxT) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return empty, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	p, e := CertifyWrappedCylinderEmbedding(ctx, w, o.Surface)
	if e != nil {
		return empty, e
	}
	options, _ := patchOptions(o.Surface)
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: p.ExactOperations}
	z := new(big.Rat).SetFloat64(origin.Z)
	if !ignoreHeight && (z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0) {
		return CylinderLineIntersections{}, nil
	}
	capPart := CylinderBoundaryPart(0)
	if !ignoreHeight && z.Cmp(p.LowerZ) == 0 {
		capPart = CylinderLowerCap
	}
	if !ignoreHeight && z.Cmp(p.UpperZ) == 0 {
		capPart = CylinderUpperCap
	}
	x := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(origin.X), p.CenterX), p.Radius), new(big.Rat).Quo(new(big.Rat).SetFloat64(direction.X), p.Radius)}
	y := rationalPolynomial{new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(origin.Y), p.CenterY), p.Radius), new(big.Rat).Quo(new(big.Rat).SetFloat64(direction.Y), p.Radius)}
	lo, hi := new(big.Rat).Set(minT), new(big.Rat).Set(maxT)
	tol := new(big.Rat).SetFloat64(o.ParameterTolerance)
	negate := func(a rationalPolynomial) rationalPolynomial {
		return rationalPolynomial{new(big.Rat).Neg(a[0]), new(big.Rat).Neg(a[1])}
	}
	quarters := [][2]rationalPolynomial{{x, y}, {y, negate(x)}, {negate(x), negate(y)}, {negate(y), x}}
	result := CylinderLineIntersections{}
	for j, coordinates := range quarters {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		u, v := coordinates[0], coordinates[1]
		a, b := new(big.Rat).Set(lo), new(big.Rat).Set(hi)
		// Clip each exact affine inequality f(t)>=0.
		clip := func(c, d *big.Rat) error {
			if d.Sign() == 0 {
				if c.Sign() < 0 {
					a.SetInt64(1)
					b.SetInt64(0)
				}
				return nil
			}
			t := new(big.Rat).Quo(new(big.Rat).Neg(c), d)
			if e := budget.check(t); e != nil {
				return e
			}
			if d.Sign() > 0 {
				if t.Cmp(a) > 0 {
					a.Set(t)
				}
			} else if t.Cmp(b) < 0 {
				b.Set(t)
			}
			return nil
		}
		oneU := rationalPolynomial{new(big.Rat).Sub(big.NewRat(1, 1), u[0]), new(big.Rat).Neg(u[1])}
		oneV := rationalPolynomial{new(big.Rat).Sub(big.NewRat(1, 1), v[0]), new(big.Rat).Neg(v[1])}
		sum := rationalPolynomial{new(big.Rat).Sub(new(big.Rat).Add(u[0], v[0]), big.NewRat(1, 1)), new(big.Rat).Add(u[1], v[1])}
		for _, linear := range []rationalPolynomial{oneU, oneV, sum} {
			if e := clip(linear[0], linear[1]); e != nil {
				return empty, e
			}
		}
		if a.Cmp(b) > 0 {
			continue
		}
		lhs, e := polynomialProduct(sum, sum, budget)
		if e != nil {
			return empty, e
		}
		rhs, e := polynomialProduct(oneU, oneV, budget)
		if e != nil {
			return empty, e
		}
		w0, w1, w2 := w.patch.net[4*j][3], w.patch.net[4*j+2][3], w.patch.net[4*j+4][3]
		leftScale := new(big.Rat).Mul(w0, w2)
		rightScale := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(w1, w1))
		polynomial := make(rationalPolynomial, 3)
		for k := 0; k < 3; k++ {
			polynomial[k] = new(big.Rat)
			if k < len(lhs) {
				polynomial[k].Mul(lhs[k], leftScale)
			}
			if k < len(rhs) {
				polynomial[k].Sub(polynomial[k], new(big.Rat).Mul(rhs[k], rightScale))
			}
			if e := budget.check(polynomial[k]); e != nil {
				return empty, e
			}
		}
		_, e = isolatePolynomialRoots(trimPolynomial(polynomial), a, b, budget, o.MaxSubdivisions, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(h, l).Cmp(tol) > 0 {
				return false, nil
			}
			result.Hits = append(result.Hits, CylinderLineHit{ToroidLineRoot{new(big.Rat).Set(l), new(big.Rat).Set(h), m}, CylinderLateral | capPart})
			return true, nil
		})
		if e != nil {
			return empty, e
		}
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	unique := result.Hits[:0]
	for _, h := range result.Hits {
		if len(unique) > 0 {
			last := &unique[len(unique)-1]
			a, b := last.Parameter, h.Parameter
			if a.Min.Cmp(a.Max) == 0 && b.Min.Cmp(b.Max) == 0 && a.Min.Cmp(b.Min) == 0 {
				last.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
				continue
			}
		}
		unique = append(unique, h)
	}
	result.Hits = unique
	if capPart == 0 {
		return result, nil
	}
	// Exact profile membership at a represented parameter using the same
	// quadrant conic, without converting the rational line point to binary64.
	inside := func(t *big.Rat) (bool, error) {
		u := new(big.Rat).Add(x[0], new(big.Rat).Mul(x[1], t))
		v := new(big.Rat).Add(y[0], new(big.Rat).Mul(y[1], t))
		j := 0
		if u.Sign() < 0 {
			if v.Sign() >= 0 {
				j = 1
				u, v = new(big.Rat).Set(v), new(big.Rat).Neg(u)
			} else {
				j = 2
				u.Neg(u)
				v.Neg(v)
			}
		} else if v.Sign() < 0 {
			j = 3
			u, v = new(big.Rat).Neg(v), new(big.Rat).Set(u)
		}
		one := big.NewRat(1, 1)
		if u.Cmp(one) > 0 || v.Cmp(one) > 0 {
			return false, nil
		}
		sum := new(big.Rat).Sub(new(big.Rat).Add(u, v), one)
		if sum.Sign() < 0 {
			return true, nil
		}
		wa, wb, wc := w.patch.net[4*j][3], w.patch.net[4*j+2][3], w.patch.net[4*j+4][3]
		left := new(big.Rat).Mul(new(big.Rat).Mul(wa, wc), new(big.Rat).Mul(sum, sum))
		right := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(wb, wb))
		right.Mul(right, new(big.Rat).Sub(one, u))
		right.Mul(right, new(big.Rat).Sub(one, v))
		for _, n := range []*big.Rat{left, right} {
			if e := budget.check(n); e != nil {
				return false, e
			}
		}
		return left.Cmp(right) <= 0, nil
	}
	left, e := inside(lo)
	if e != nil {
		return empty, e
	}
	right, e := inside(hi)
	if e != nil {
		return empty, e
	}
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	if !left && !right && len(result.Hits) < 2 {
		return result, nil
	}
	a, b := exact(lo), exact(hi)
	if !left {
		if len(result.Hits) == 0 {
			return result, nil
		}
		a = result.Hits[0].Parameter
	}
	if !right {
		if len(result.Hits) == 0 {
			return result, nil
		}
		b = result.Hits[len(result.Hits)-1].Parameter
	}
	if a.Min.Cmp(a.Max) == 0 && b.Min.Cmp(b.Max) == 0 && a.Min.Cmp(b.Min) == 0 {
		if len(result.Hits) == 0 {
			result.Hits = append(result.Hits, CylinderLineHit{a, capPart})
		}
		return result, nil
	}
	result.Overlaps = append(result.Overlaps, CylinderLineOverlap{a, b, capPart})
	return result, nil
}
