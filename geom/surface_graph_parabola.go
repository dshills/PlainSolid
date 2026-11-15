package geom

import (
	"context"
	"math/big"
)

// SplineGraphParabola proves exactly one open transverse contact v=f(u) or u=f(v) over
// the complete original rectangle. Bernstein controls and terminal masks use
// original UV; PositiveSide is the height sign toward increasing dependent coordinate. Every original
// union-knot numerator is a nonzero constant multiple of that same factor.
// Transversality is one-sided on knot cells; tangent continuity across original
// knots is not inferred.
type SplineGraphParabola struct {
	// FreeAxis is 0 for v=f(u), 1 for u=f(v); Power uses that original coordinate.
	FreeAxis        int
	Proven          bool
	Domain          ParameterRectangle
	Power           [3]*big.Rat
	Controls        [3][2]*big.Rat
	TerminalMasks   [2]uint8
	PositiveSide    int
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphParabola infers a genuinely quadratic graph from the first
// original difference cell, then proves all cells independently. Strict vertical
// Bernstein clearance excludes corner/tangent boundary contacts. Higher-degree
// quotients, extra branches and unproved clearance are unresolved, not absent.
func DiscoverSplineGraphParabola(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphParabola, err error) {
	return DiscoverSplineGraphParabolaOnAxis(ctx, a, c, 0, o)
}

// DiscoverSplineGraphParabolaOnAxis proves the same complete graph in either
// original coordinate, without transposing or replacing either support.
// PositiveSide is the sign toward increasing dependent coordinate.
func DiscoverSplineGraphParabolaOnAxis(ctx context.Context, a, c *SurfacePatch, axis int, o GraphPointContactOptions) (r SplineGraphParabola, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphParabola{}
		}
		r.ExactOperations = used
	}()
	if ctx == nil || a == nil || c == nil || axis < 0 || axis > 1 {
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
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { used = budget.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, p, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	q, quadratic, e := graphAxisQuadraticCoefficients(cells[0], budget)
	if e != nil {
		return r, e
	}
	if axis == 1 {
		q = [5]*big.Rat{q[0], q[3], q[4], q[1], q[2]}
	}
	if !quadratic || q[2].Sign() == 0 || q[3].Sign() == 0 || q[4].Sign() != 0 {
		return r, ErrConvergence
	}
	orient := func(d ParameterRectangle) ParameterRectangle {
		if axis == 1 {
			return ParameterRectangle{UMin: d.VMin, UMax: d.VMax, VMin: d.UMin, VMax: d.UMax}
		}
		return d
	}
	first := orient(cells[0].Domain())
	u0 := new(big.Rat).SetFloat64(first.UMin)
	v0 := new(big.Rat).SetFloat64(first.VMin)
	du := new(big.Rat).Sub(new(big.Rat).SetFloat64(first.UMax), u0)
	dv := new(big.Rat).Sub(new(big.Rat).SetFloat64(first.VMax), v0)
	local := make(rationalPolynomial, 3)
	for i := range local {
		local[i] = new(big.Rat).Neg(new(big.Rat).Quo(q[i], q[3]))
		if e = budget.check(local[i]); e != nil {
			return r, e
		}
	}
	shift := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Quo(u0, du)), new(big.Rat).Inv(du)}
	for _, v := range shift {
		if e = budget.check(v); e != nil {
			return r, e
		}
	}
	global, e := composeProjectionPolynomial(local, shift, budget)
	if e != nil {
		return r, e
	}
	for i := 0; i < 3; i++ {
		r.Power[i] = new(big.Rat).Mul(global[i], dv)
		if i == 0 {
			r.Power[i].Add(r.Power[i], v0)
		}
		if e = budget.check(r.Power[i]); e != nil {
			return r, e
		}
	}
	sign := 0
	for _, cell := range cells {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		d := orient(cell.Domain())
		coeff, quadratic, e := graphAxisQuadraticCoefficients(cell, budget)
		if e != nil {
			return r, e
		}
		if axis == 1 {
			coeff = [5]*big.Rat{coeff[0], coeff[3], coeff[4], coeff[1], coeff[2]}
		}
		if !quadratic || coeff[3].Sign() == 0 {
			return r, ErrConvergence
		}
		origin := new(big.Rat).SetFloat64(d.UMin)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), origin)
		f, e := composeProjectionPolynomial(r.Power[:], rationalPolynomial{origin, width}, budget)
		if e != nil {
			return r, e
		}
		vlo := new(big.Rat).SetFloat64(d.VMin)
		vwidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), vlo)
		expected := [5]*big.Rat{new(big.Rat).Sub(vlo, f[0]), new(big.Rat).Neg(f[1]), new(big.Rat).Neg(f[2]), vwidth, new(big.Rat)}
		scale := new(big.Rat).Quo(coeff[3], vwidth)
		if e = budget.check(scale); e != nil {
			return r, e
		}
		for i, w := range expected {
			w.Mul(w, scale)
			if e = budget.check(w); e != nil {
				return r, e
			}
			if w.Cmp(coeff[i]) != 0 {
				return r, ErrConvergence
			}
		}
		if sign != 0 && sign != scale.Sign() {
			return r, ErrConvergence
		}
		sign = scale.Sign()
		r.CertifiedCells = append(r.CertifiedCells, cell.Domain())
	}
	d := orient(p.Domain())
	lo, hi := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	width := new(big.Rat).Sub(hi, lo)
	f, e := composeProjectionPolynomial(r.Power[:], rationalPolynomial{lo, width}, budget)
	if e != nil {
		return r, e
	}
	controls := [3]*big.Rat{new(big.Rat).Set(f[0]), new(big.Rat).Add(f[0], new(big.Rat).Quo(f[1], big.NewRat(2, 1))), new(big.Rat).Add(f[0], new(big.Rat).Add(f[1], f[2]))}
	for i, v := range controls {
		if e = budget.check(v); e != nil {
			return r, e
		}
		if v.Cmp(new(big.Rat).SetFloat64(d.VMin)) <= 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMax)) >= 0 {
			return r, ErrConvergence
		}
		u := new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), 2)))
		if e = budget.check(u); e != nil {
			return r, e
		}
		r.Controls[i] = [2]*big.Rat{u, v}
		if axis == 1 {
			r.Controls[i] = [2]*big.Rat{v, u}
		}
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxDepth: o.MaxDepth, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	r.Domain = p.Domain()
	r.FreeAxis = axis
	r.PositiveSide = sign
	r.TerminalMasks = [2]uint8{1, 2}
	if axis == 1 {
		r.TerminalMasks = [2]uint8{4, 8}
	}
	r.Proven = true
	return r, nil
}
