package geom

import (
	"context"
	"math/big"
	"sort"
)

// CircularAnnulusLineIntersections records exact original-parameter
// intervals coincident with the closed annulus, plus isolated/rim contacts.
// A tangent to the inner circle lies in the annulus on both sides.
type CircularAnnulusLineIntersections struct {
	Rims  []CircularAnnulusRimHit
	Spans [][2]ToroidLineRoot
	Point *ToroidLineRoot // zero-length parameter interval in the relative interior
}

type CircularAnnulusRimHit struct {
	Parameter ToroidLineRoot
	Inner     bool
}

// IntersectCircularAnnulusLine proves all contacts of a finite represented
// 2D line with a rational concentric circular annulus. Exact quadratic root
// isolation retains inner/outer provenance. Root enclosures are refined
// until different rim events have disjoint brackets, or the work fails.
func IntersectCircularAnnulusLine(ctx context.Context, center [2]*big.Rat, inner, outer *big.Rat,
	origin, direction Vec2, minT, maxT float64, o ToroidLineOptions) (CircularAnnulusLineIntersections, error) {
	var empty CircularAnnulusLineIntersections
	if ctx == nil || !finite(origin.X) || !finite(origin.Y) || !finite(direction.X) || !finite(direction.Y) || direction == (Vec2{}) ||
		!finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 || center[0] == nil || center[1] == nil ||
		inner == nil || outer == nil || inner.Sign() <= 0 || inner.Cmp(outer) >= 0 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	remaining := o.MaxSubdivisions
	if remaining == 0 {
		remaining = 1024
	}
	rat := func(f float64) *big.Rat { return new(big.Rat).SetFloat64(f) }
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	x, y := new(big.Rat).Sub(rat(origin.X), center[0]), new(big.Rat).Sub(rat(origin.Y), center[1])
	u, v := rat(direction.X), rat(direction.Y)
	q := rationalPolynomial{
		new(big.Rat).Add(square(x), square(y)),
		new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(new(big.Rat).Mul(x, u), new(big.Rat).Mul(y, v))),
		new(big.Rat).Add(square(u), square(v)),
	}
	a, z, tol := rat(minT), rat(maxT), rat(o.ParameterTolerance)
	inner2, outer2 := square(inner), square(outer)
	for _, r := range []*big.Rat{a, z, tol, q[0], q[1], q[2], inner2, outer2} {
		if e := b.check(r); e != nil {
			return empty, e
		}
	}
	exact := func(t *big.Rat) ToroidLineRoot { return ToroidLineRoot{new(big.Rat).Set(t), new(big.Rat).Set(t), 1} }
	f, g := clonePolynomial(q), clonePolynomial(q)
	f[0].Sub(f[0], inner2)
	g[0].Sub(g[0], outer2)
	product, e := polynomialProduct(f, g, b)
	if e != nil {
		return empty, e
	}
	var result CircularAnnulusLineIntersections
	for {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		result.Rims = nil
		n, e := isolatePolynomialRoots(product, a, z, b, remaining, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if l.Cmp(h) != 0 && (l.Cmp(a) == 0 || h.Cmp(z) == 0 || new(big.Rat).Sub(h, l).Cmp(tol) > 0) {
				return false, nil
			}
			u, e := polynomialValue(f, l, b)
			if e != nil {
				return false, e
			}
			inner := u.Sign() == 0
			if l.Cmp(h) != 0 {
				v, e := polynomialValue(f, h, b)
				if e != nil {
					return false, e
				}
				w, e := polynomialValue(g, l, b)
				if e != nil {
					return false, e
				}
				x, e := polynomialValue(g, h, b)
				if e != nil {
					return false, e
				}
				if u.Sign() == 0 || v.Sign() == 0 || w.Sign() == 0 || x.Sign() == 0 {
					return false, nil
				}
				// A double root of either quadratic is its rational
				// vertex. Nonexact double brackets require derivative
				// evaluation at that exact vertex instead of sign change.
				inner = u.Sign() != v.Sign()
				if m == 2 {
					vertex := new(big.Rat).Quo(new(big.Rat).Neg(q[1]), new(big.Rat).Mul(big.NewRat(2, 1), q[2]))
					if vertex.Cmp(l) <= 0 || vertex.Cmp(h) >= 0 {
						return false, ErrConvergence
					}
					v, e := polynomialValue(f, vertex, b)
					if e != nil {
						return false, e
					}
					result.Rims = append(result.Rims, CircularAnnulusRimHit{exact(vertex), v.Sign() == 0})
					result.Rims[len(result.Rims)-1].Parameter.Multiplicity = m
					return true, nil
				}
				if !inner && w.Sign() == x.Sign() {
					return false, ErrConvergence
				}
			}
			result.Rims = append(result.Rims, CircularAnnulusRimHit{ToroidLineRoot{new(big.Rat).Set(l), new(big.Rat).Set(h), m}, inner})
			return true, nil
		})
		if e != nil {
			return empty, e
		}
		remaining -= n
		sort.Slice(result.Rims, func(i, j int) bool { return result.Rims[i].Parameter.Min.Cmp(result.Rims[j].Parameter.Min) < 0 })
		disjoint := true
		for i := 1; i < len(result.Rims); i++ {
			a, c := result.Rims[i-1].Parameter, result.Rims[i].Parameter
			if a.Max.Cmp(c.Min) >= 0 {
				disjoint = false
				for _, p := range [2]ToroidLineRoot{a, c} {
					w := new(big.Rat).Sub(p.Max, p.Min)
					if w.Sign() > 0 && w.Cmp(tol) < 0 {
						tol.Set(w)
					}
				}
			}
		}
		if disjoint {
			break
		}
		if remaining == 0 {
			return empty, ErrConvergence
		}
		remaining--
		tol.Quo(tol, big.NewRat(8, 1))
		if e := b.check(tol); e != nil {
			return empty, e
		}
	}
	if a.Cmp(z) == 0 {
		value, e := polynomialValue(q, a, b)
		if e != nil {
			return empty, e
		}
		if value.Cmp(inner2) > 0 && value.Cmp(outer2) < 0 {
			x := exact(a)
			result.Point = &x
		}
		return result, nil
	}
	breaks := []ToroidLineRoot{exact(a)}
	for _, r := range result.Rims {
		t := r.Parameter
		if t.Min.Cmp(a) == 0 && t.Max.Cmp(a) == 0 {
			breaks[0] = t
		} else {
			breaks = append(breaks, t)
		}
	}
	if breaks[len(breaks)-1].Max.Cmp(z) < 0 {
		breaks = append(breaks, exact(z))
	}
	open := false
	for i := 1; i < len(breaks); i++ {
		l, h := breaks[i-1], breaks[i]
		if l.Max.Cmp(h.Min) >= 0 {
			return empty, ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(l.Max, h.Min), big.NewRat(2, 1))
		value, e := polynomialValue(q, mid, b)
		if e != nil {
			return empty, e
		}
		inside := value.Cmp(inner2) > 0 && value.Cmp(outer2) < 0
		if inside {
			if open {
				result.Spans[len(result.Spans)-1][1] = h
			} else {
				result.Spans = append(result.Spans, [2]ToroidLineRoot{l, h})
			}
		}
		open = inside
	}
	return result, nil
}
