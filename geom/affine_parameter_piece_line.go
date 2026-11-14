package geom

import (
	"context"
	"math/big"
	"sort"
)

// PiecewiseGraphLineHit encloses a root in the global normalized parameter
// of an exact graph curve. Multiplicity is the maximum incident piece order
// at a knot; Pieces identifies every isolated incident piece.
type PiecewiseGraphLineHit struct {
	Parameter ToroidLineRoot
	Pieces    []int
	// SignBefore/SignAfter are strict signs of graph.Z minus line.Z
	// on the adjacent root-free side. Zero means the side is outside
	// the supplied range, or the adjacent piece is coincident.
	SignBefore, SignAfter int
}

// PiecewiseGraphLineIntersection reports all roots and exact coincident
// intervals along a normalized affine UV path. Every curve piece must have
// a continuous identity-XY model image for the intended graph use.
type PiecewiseGraphLineIntersection struct {
	Hits            []PiecewiseGraphLineHit
	Coincident      []ExactParameterInterval
	ExactOperations int
}

// IntersectAffineGraphCurveHeight intersects the Z coordinate of a complete
// ordered collection of exact curve pieces with height(t)=start+(end-start)t.
// Endpoint and tangency roots are retained. No float conversion is applied
// to a curve coefficient, a piece interval, or a height.
func IntersectAffineGraphCurveHeight(ctx context.Context, curves SurfaceAffineCurvePieces, start, end *big.Rat,
	o ToroidLineOptions) (PiecewiseGraphLineIntersection, error) {
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return PiecewiseGraphLineIntersection{}, ErrInvalid
	}
	return intersectAffineGraphCurveHeight(ctx, curves, 2, start, end, new(big.Rat).SetFloat64(o.ParameterTolerance), o, false)
}

// IntersectAffineGraphCurveHeightRational selects any original coordinate as
// the graph height, isolates all contacts to an exact positive normalized
// tolerance, and recovers exactly representable rational roots when possible.
func IntersectAffineGraphCurveHeightRational(ctx context.Context, curves SurfaceAffineCurvePieces, axis int, start, end, tolerance *big.Rat,
	o ToroidLineOptions) (PiecewiseGraphLineIntersection, error) {
	return intersectAffineGraphCurveHeight(ctx, curves, axis, start, end, tolerance, o, true)
}

func intersectAffineGraphCurveHeight(ctx context.Context, curves SurfaceAffineCurvePieces, axis int, start, end, tol *big.Rat,
	o ToroidLineOptions, recover bool) (PiecewiseGraphLineIntersection, error) {
	var empty PiecewiseGraphLineIntersection
	if ctx == nil || axis < 0 || axis > 2 || start == nil || end == nil || tol == nil || tol.Sign() <= 0 || len(curves.Pieces) == 0 || len(curves.Pieces) > 4096 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return empty, e
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	for _, v := range []*big.Rat{start, end, tol} {
		if e := b.check(v); e != nil {
			return empty, e
		}
	}
	delta := new(big.Rat).Sub(end, start)
	one, zero := big.NewRat(1, 1), new(big.Rat)
	atHeight := func(t *big.Rat) *big.Rat { return new(big.Rat).Add(start, new(big.Rat).Mul(delta, t)) }
	var r PiecewiseGraphLineIntersection
	last := zero
	var previous *big.Rat
	remaining := o.MaxSubdivisions
	for i, x := range curves.Pieces {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if x.Curve == nil || x.Curve.patch == nil || x.Start == nil || x.End == nil || x.Start.Cmp(last) != 0 ||
			x.End.Cmp(x.Start) <= 0 || x.End.Cmp(one) > 0 {
			return empty, ErrInvalid
		}
		p := x.Curve.patch
		u, v := x.Curve.Domain()
		if u != 0 || v != 1 || p.degreeU+1 != p.countU || p.countU > options.MaxPoles/2 {
			return empty, ErrInvalid
		}
		first, final := p.net[0], p.net[(p.countU-1)*p.countV]
		z0, z1 := new(big.Rat).Quo(first[axis], first[3]), new(big.Rat).Quo(final[axis], final[3])
		if previous != nil && previous.Cmp(z0) != 0 {
			return empty, ErrConvergence
		}
		last, previous = x.End, z1
		width := new(big.Rat).Sub(x.End, x.Start)
		a, c := atHeight(x.Start), new(big.Rat).Mul(delta, width)
		for _, n := range []*big.Rat{z0, z1, x.Start, x.End, width, a, c} {
			if e := b.check(n); e != nil {
				return empty, e
			}
		}
		n, w := make([]*big.Rat, p.countU), make([]*big.Rat, p.countU)
		for j := range n {
			h := p.net[j*p.countV]
			n[j], w[j] = h[axis], h[3]
		}
		f, e := bernsteinToPower(n, b)
		if e != nil {
			return empty, e
		}
		g, e := bernsteinToPower(w, b)
		if e != nil {
			return empty, e
		}
		g, e = polynomialProduct(g, rationalPolynomial{a, c}, b)
		if e != nil {
			return empty, e
		}
		k := max(len(f), len(g))
		equation := make(rationalPolynomial, k)
		for j := range equation {
			equation[j] = new(big.Rat)
			if j < len(f) {
				equation[j].Set(f[j])
			}
			if j < len(g) {
				equation[j].Sub(equation[j], g[j])
			}
			if e := b.check(equation[j]); e != nil {
				return empty, e
			}
		}
		equation = trimPolynomial(equation)
		if len(equation) == 0 {
			if j := len(r.Coincident); j > 0 && r.Coincident[j-1].Upper.Cmp(x.Start) == 0 {
				r.Coincident[j-1].Upper.Set(x.End)
			} else {
				r.Coincident = append(r.Coincident, ExactParameterInterval{Lower: new(big.Rat).Set(x.Start), Upper: new(big.Rat).Set(x.End)})
			}
			continue
		}
		global := func(local *big.Rat) *big.Rat { return new(big.Rat).Add(x.Start, new(big.Rat).Mul(width, local)) }
		steps, e := isolatePolynomialRoots(equation, zero, one, b, remaining, func(l, h *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if recover {
				x, e := recoverRationalPolynomialRoot(equation, l, h, b)
				if e != nil {
					return false, e
				}
				if x != nil {
					l, h = x, x
				}
			}
			a, z := global(l), global(h)
			if l.Cmp(h) != 0 && (l.Sign() == 0 || h.Cmp(one) == 0 || new(big.Rat).Sub(z, a).Cmp(tol) > 0) {
				return false, nil
			}
			before, after := 0, 0
			if l.Cmp(h) != 0 {
				left, e := polynomialValue(equation, l, b)
				if e != nil {
					return false, e
				}
				right, e := polynomialValue(equation, h, b)
				if e != nil {
					return false, e
				}
				if left.Sign() == 0 || right.Sign() == 0 {
					return false, nil
				}
				before, after = left.Sign(), right.Sign()
			} else {
				derivative := equation
				for j := 0; j < m; j++ {
					derivative = polynomialDerivative(derivative)
				}
				v, e := polynomialValue(derivative, l, b)
				if e != nil {
					return false, e
				}
				if v.Sign() == 0 {
					return false, ErrConvergence
				}
				if l.Sign() > 0 {
					before = v.Sign()
					if m%2 != 0 {
						before = -before
					}
				}
				if l.Cmp(one) < 0 {
					after = v.Sign()
				}
			}
			r.Hits = append(r.Hits, PiecewiseGraphLineHit{
				Parameter: ToroidLineRoot{a, z, m}, Pieces: []int{i}, SignBefore: before, SignAfter: after,
			})
			return true, nil
		})
		if e != nil {
			return empty, e
		}
		remaining -= steps
	}
	if last.Cmp(one) != 0 {
		return empty, ErrInvalid
	}
	sort.SliceStable(r.Hits, func(i, j int) bool { return r.Hits[i].Parameter.Min.Cmp(r.Hits[j].Parameter.Min) < 0 })
	out := r.Hits[:0]
	for _, h := range r.Hits {
		if len(out) > 0 {
			last := &out[len(out)-1]
			a, c := last.Parameter, h.Parameter
			if a.Max.Cmp(c.Min) > 0 || a.Max.Cmp(c.Min) == 0 && (a.Min.Cmp(a.Max) != 0 || c.Min.Cmp(c.Max) != 0) {
				return empty, ErrConvergence
			}
			if a.Max.Cmp(c.Min) == 0 {
				last.Parameter.Multiplicity = max(a.Multiplicity, c.Multiplicity)
				last.Pieces = append(last.Pieces, h.Pieces...)
				if last.SignBefore == 0 {
					last.SignBefore = h.SignBefore
				}
				if last.SignAfter == 0 {
					last.SignAfter = h.SignAfter
				}
				continue
			}
		}
		out = append(out, h)
	}
	r.Hits = out
	r.ExactOperations = b.operations
	return r, nil
}
