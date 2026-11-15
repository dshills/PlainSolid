package geom

import (
	"context"
	"math/big"
)

// SplineGraphTwoPolynomials certifies exactly two disjoint original polynomial
// graph contacts. Power and Controls are ordered by dependent coordinate;
// StripSigns describe below, between, and above. Reports are not mutable native
// authorization. Original knot cells are re-expanded independently.
type SplineGraphTwoPolynomials struct {
	Proven          bool
	Domain          ParameterRectangle
	FreeAxis        int
	Power           [2][]*big.Rat
	Controls        [2][][2]*big.Rat
	StripSigns      [3]int
	TerminalMasks   [2]uint8
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphTwoPolynomials accepts dependent quadratics with nonzero
// constant leading coefficient per original cell, and free degree at most eight.
// Both roots must be cubic/quartic polynomials, strictly ordered and interior on
// the complete domain. Positive Bernstein subdivision proves ordering; no root
// samples substitute for full identities. All failure results are atomic.
func DiscoverSplineGraphTwoPolynomials(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions) (SplineGraphTwoPolynomials, error) {
	return discoverSplineGraphTwoPolynomials(ctx, a, b, axis, o, polynomialGraphContactMode{})
}

type polynomialGraphContactMode struct {
	count, multiplicity int
	certify             func(context.Context, *SurfacePatch, SplineGraphTwoPolynomials, []*big.Rat, ParameterRectangle, *polynomialBudget) error
}

func discoverSplineGraphTwoPolynomials(ctx context.Context, a, b *SurfacePatch, axis int, o GraphPointContactOptions, mode polynomialGraphContactMode) (r SplineGraphTwoPolynomials, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphTwoPolynomials{}
		}
		r.ExactOperations = used
	}()
	if ctx == nil || a == nil || b == nil || len(a.net) == 0 || len(b.net) == 0 || axis < 0 || axis > 1 {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxDepth > 32 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { used = budget.operations }()
	p, e := orderingDifference(ctx, a, b, o.MaxCells, budget)
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
	orient := func(d ParameterRectangle) ParameterRectangle {
		if axis == 1 {
			return ParameterRectangle{UMin: d.VMin, UMax: d.VMax, VMin: d.UMin, VMax: d.UMax}
		}
		return d
	}
	coefficients := func(cell *SurfacePatch) (out [3]rationalPolynomial, e error) {
		for j := range out {
			out[j] = make(rationalPolynomial, 9)
			for i := range out[j] {
				out[j][i] = new(big.Rat)
			}
		}
		columns, e := graphNumeratorPowerColumns(cell, budget)
		if e != nil {
			return out, e
		}
		for j, col := range columns {
			for i, x := range col {
				if x.Sign() == 0 {
					continue
				}
				u, v := i, j
				if axis == 1 {
					u, v = v, u
				}
				if u > 8 || v > 2 {
					return out, ErrConvergence
				}
				out[v][u].Set(x)
			}
		}
		for j := range out {
			out[j] = trimPolynomial(out[j])
		}
		if len(out[2]) != 1 {
			return out, ErrConvergence
		}
		return out, nil
	}
	c, e := coefficients(cells[0])
	if e != nil {
		return r, e
	}
	bb, e := polynomialProduct(c[1], c[1], budget)
	if e != nil {
		return r, e
	}
	ac, e := polynomialProduct(c[0], c[2], budget)
	if e != nil {
		return r, e
	}
	for _, x := range ac {
		x.Mul(x, big.NewRat(4, 1))
		if e = budget.check(x); e != nil {
			return r, e
		}
	}
	disc, e := graphPolynomialSum(bb, ac, -1, budget)
	if e != nil {
		return r, e
	}
	// Cubic/quartic roots have a discriminant square of degree at most eight.
	if len(disc) > 9 {
		return r, ErrConvergence
	}
	if len(disc) == 0 && mode.count > 1 {
		return r, ErrDegenerate
	}
	coincident := len(disc) == 0 && mode.multiplicity == 2
	var root rationalPolynomial
	if !coincident {
		root, e = graphExactPolynomialSqrt(disc, budget)
		if e != nil {
			return r, e
		}
	}
	first := orient(cells[0].Domain())
	u0 := new(big.Rat).SetFloat64(first.UMin)
	du := new(big.Rat).Sub(new(big.Rat).SetFloat64(first.UMax), u0)
	v0 := new(big.Rat).SetFloat64(first.VMin)
	dv := new(big.Rat).Sub(new(big.Rat).SetFloat64(first.VMax), v0)
	shift := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Quo(u0, du)), new(big.Rat).Inv(du)}
	divisor := new(big.Rat).Mul(big.NewRat(2, 1), c[2][0])
	for _, x := range append(shift, divisor) {
		if e = budget.check(x); e != nil {
			return r, e
		}
	}
	for k, sign := range []int{-1, 1} {
		neg := clonePolynomial(c[1])
		for _, x := range neg {
			x.Neg(x)
		}
		q, e := graphPolynomialSum(neg, root, sign, budget)
		if e != nil {
			return r, e
		}
		for _, x := range q {
			x.Quo(x, divisor)
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		if len(q) < 4 || len(q) > 5 {
			return r, ErrConvergence
		}
		global, e := composeProjectionPolynomial(q, shift, budget)
		if e != nil {
			return r, e
		}
		for i, x := range global {
			x.Mul(x, dv)
			if i == 0 {
				x.Add(x, v0)
			}
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		r.Power[k] = global
	}
	d := orient(p.Domain())
	lo := new(big.Rat).SetFloat64(d.UMin)
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), lo)
	visits := len(cells)
	proveSign := func(poly rationalPolynomial) (int, error) {
		bern, e := graphFactorBernstein(poly, budget)
		if e != nil {
			return 0, e
		}
		return certifyGraphFactorSign(ctx, bern, o.MaxDepth, o.MaxCells, &visits, budget)
	}
	delta, e := graphPolynomialSum(r.Power[1], r.Power[0], -1, budget)
	if e != nil {
		return r, e
	}
	delta, e = composeProjectionPolynomial(delta, rationalPolynomial{lo, width}, budget)
	if e != nil {
		return r, e
	}
	var order int
	var roots []*big.Rat
	if coincident {
		order = 1
	} else if mode.count == 0 {
		order, e = proveSign(delta)
	} else {
		roots, order, e = discoverGraphPolynomialPairRoots(ctx, delta, mode.count, mode.multiplicity, o.MaxCells, &visits, budget)
	}
	if e != nil {
		return r, e
	}
	if order < 0 {
		r.Power[0], r.Power[1] = r.Power[1], r.Power[0]
	}
	for k, power := range r.Power {
		local, e := composeProjectionPolynomial(power, rationalPolynomial{lo, width}, budget)
		if e != nil {
			return r, e
		}
		lower := clonePolynomial(local)
		lower[0].Sub(lower[0], new(big.Rat).SetFloat64(d.VMin))
		if e = budget.check(lower[0]); e != nil {
			return r, e
		}
		upper := clonePolynomial(local)
		for _, x := range upper {
			x.Neg(x)
		}
		upper[0].Add(upper[0], new(big.Rat).SetFloat64(d.VMax))
		if e = budget.check(upper[0]); e != nil {
			return r, e
		}
		for _, q := range []rationalPolynomial{lower, upper} {
			s, e := proveSign(q)
			if e != nil {
				return r, e
			}
			if s != 1 {
				return r, ErrConvergence
			}
		}
		bern, e := graphFactorBernstein(local, budget)
		if e != nil {
			return r, e
		}
		r.Controls[k] = make([][2]*big.Rat, len(bern))
		for i, v := range bern {
			u := new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(len(bern)-1))))
			if e = budget.check(u); e != nil {
				return r, e
			}
			r.Controls[k][i] = [2]*big.Rat{u, v}
			if axis == 1 {
				r.Controls[k][i] = [2]*big.Rat{v, u}
			}
		}
	}
	sign := 0
	for _, cell := range cells {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		c, e := coefficients(cell)
		if e != nil {
			return r, e
		}
		s := c[2][0].Sign()
		if sign != 0 && sign != s {
			return r, ErrConvergence
		}
		sign = s
		cd := orient(cell.Domain())
		origin := new(big.Rat).SetFloat64(cd.UMin)
		span := new(big.Rat).Sub(new(big.Rat).SetFloat64(cd.UMax), origin)
		vlo := new(big.Rat).SetFloat64(cd.VMin)
		vw := new(big.Rat).Sub(new(big.Rat).SetFloat64(cd.VMax), vlo)
		var f [2]rationalPolynomial
		for k := range f {
			f[k], e = composeProjectionPolynomial(r.Power[k], rationalPolynomial{origin, span}, budget)
			if e != nil {
				return r, e
			}
			f[k][0].Sub(f[k][0], vlo)
			for _, x := range f[k] {
				x.Quo(x, vw)
				if e = budget.check(x); e != nil {
					return r, e
				}
			}
		}
		sum, e := graphPolynomialSum(f[0], f[1], 1, budget)
		if e != nil {
			return r, e
		}
		prod, e := polynomialProduct(f[0], f[1], budget)
		if e != nil {
			return r, e
		}
		for _, x := range sum {
			x.Mul(x, new(big.Rat).Neg(c[2][0]))
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		for _, x := range prod {
			x.Mul(x, c[2][0])
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		for j, q := range []rationalPolynomial{prod, sum} {
			rem, e := graphPolynomialSum(c[j], q, -1, budget)
			if e != nil {
				return r, e
			}
			if len(rem) != 0 {
				return r, ErrConvergence
			}
		}
		r.CertifiedCells = append(r.CertifiedCells, cell.Domain())
	}
	if coincident {
		// A repeated factor in the first cell alone says nothing about later
		// spans. The same two factors now account for every source coefficient.
		return r, ErrDegenerate
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 || visits >= o.MaxCells {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells - visits, MaxDepth: o.MaxDepth, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return r, e
	}
	r.Proven = true
	r.Domain = p.Domain()
	r.FreeAxis = axis
	r.StripSigns = [3]int{sign, -sign, sign}
	r.TerminalMasks = [2]uint8{1, 2}
	if axis == 1 {
		r.TerminalMasks = [2]uint8{4, 8}
	}
	for _, root := range roots {
		root.Mul(root, width)
		root.Add(root, lo)
		if e = budget.check(root); e != nil {
			return r, e
		}
	}
	if mode.certify != nil {
		if e = mode.certify(ctx, a, r, roots, d, budget); e != nil {
			return r, e
		}
	}
	return r, nil
}
