package geom

import (
	"context"
	"math/big"
)

// SplineGraphFactoredPolynomialContact proves one complete cubic/quartic graph
// multiplied by a nonconstant, strictly sign-definite free-coordinate factor.
// The mode is additive: dependent-degree >1 and factors depending on both UV
// coordinates remain unresolved. Original parameter units and cell ownership
// are retained; no cross-knot tangent continuity is inferred.
type SplineGraphFactoredPolynomialContact struct {
	// FreeAxis is 0 for v=f(u), 1 for u=f(v); Power uses that original coordinate.
	FreeAxis        int
	Degree          int
	Factors         []PolynomialGraphFactor
	Proven          bool
	Domain          ParameterRectangle
	Power           []*big.Rat
	Controls        [][2]*big.Rat
	TerminalMasks   [2]uint8
	PositiveSide    int
	CertifiedCells  []ParameterRectangle
	Graph           SurfaceInjectivity
	ExactOperations int
}

// DiscoverSplineGraphFactoredPolynomialContact recovers f by exact division of
// the first original cell's constant-dependent coefficient by its linear one.
// It then rechecks every original coefficient against g(free)*(dependent-f).
// Factor degrees are <=4, full numerator free degree <=8. Strict Bernstein sign
// proof may use dyadic subdivision up to MaxDepth (default zero); MaxCells also
// bounds cumulative factor sign cells. Zeros, hidden branches, cancellation and
// exhausted proof publish no partial result. The constant-factor mode is unchanged.
func DiscoverSplineGraphFactoredPolynomialContact(ctx context.Context, a, c *SurfacePatch, axis int, o GraphPointContactOptions) (r SplineGraphFactoredPolynomialContact, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphFactoredPolynomialContact{}
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxDepth > 32 {
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
	q, linear, e := graphFreeFactorCoefficients(cells[0], axis, budget)
	if e != nil {
		return r, e
	}
	quotient, remainder, e := polynomialDivide(q, linear, budget)
	if e != nil {
		return r, e
	}
	if len(remainder) != 0 {
		return r, ErrConvergence
	}
	degree := len(quotient) - 1
	if degree < 3 || degree > 4 {
		return r, ErrConvergence
	}
	r.Degree = degree
	r.Power = make([]*big.Rat, degree+1)
	r.Controls = make([][2]*big.Rat, degree+1)
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
	local := make(rationalPolynomial, degree+1)
	for i := range local {
		local[i] = new(big.Rat).Neg(quotient[i])
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
	for i := 0; i <= degree; i++ {
		r.Power[i] = new(big.Rat).Mul(global[i], dv)
		if i == 0 {
			r.Power[i].Add(r.Power[i], v0)
		}
		if e = budget.check(r.Power[i]); e != nil {
			return r, e
		}
	}
	sign, visits := 0, 0
	nonconstant := false
	for _, cell := range cells {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		d := orient(cell.Domain())
		coeff, dependent, e := graphFreeFactorCoefficients(cell, axis, budget)
		if e != nil {
			return r, e
		}
		if len(dependent) == 0 {
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
		base := make(rationalPolynomial, len(f))
		for i := range f {
			base[i] = new(big.Rat).Neg(f[i])
		}
		base[0].Add(base[0], vlo)
		for _, x := range base {
			x.Quo(x, vwidth)
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		expected, e := polynomialProduct(base, dependent, budget)
		if e != nil {
			return r, e
		}
		if len(expected) != len(coeff) {
			return r, ErrConvergence
		}
		for i := range expected {
			if expected[i].Cmp(coeff[i]) != 0 {
				return r, ErrConvergence
			}
		}
		controls, e := graphFactorBernstein(dependent, budget)
		if e != nil {
			return r, e
		}
		before := visits
		factorSign, e := certifyGraphFactorSign(ctx, controls, o.MaxDepth, o.MaxCells, &visits, budget)
		if e != nil {
			return r, e
		}
		if sign != 0 && sign != factorSign {
			return r, ErrConvergence
		}
		sign = factorSign
		if len(dependent) > 1 {
			nonconstant = true
		}
		r.Factors = append(r.Factors, PolynomialGraphFactor{Domain: cell.Domain(), Power: clonePolynomial(dependent), Bernstein: clonePolynomial(controls), Sign: factorSign, VisitedCells: visits - before})
		r.CertifiedCells = append(r.CertifiedCells, cell.Domain())
	}
	if !nonconstant {
		return r, ErrConvergence
	}
	d := orient(p.Domain())
	lo, hi := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)
	width := new(big.Rat).Sub(hi, lo)
	f, e := composeProjectionPolynomial(r.Power[:], rationalPolynomial{lo, width}, budget)
	if e != nil {
		return r, e
	}
	controls := make([]*big.Rat, degree+1)
	for i := range controls {
		controls[i] = new(big.Rat)
		for j := 0; j <= i; j++ {
			term := new(big.Rat).Mul(f[j], new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j))))
			if e = budget.check(term); e != nil {
				return r, e
			}
			controls[i].Add(controls[i], term)
		}
	}
	for i, v := range controls {
		if e = budget.check(v); e != nil {
			return r, e
		}
		if v.Cmp(new(big.Rat).SetFloat64(d.VMin)) <= 0 || v.Cmp(new(big.Rat).SetFloat64(d.VMax)) >= 0 {
			return r, ErrConvergence
		}
		u := new(big.Rat).Add(lo, new(big.Rat).Mul(width, big.NewRat(int64(i), int64(degree))))
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

// PolynomialGraphFactor records the original cell's normalized free-coordinate
// power/Bernstein coefficients of the linear dependent term. Positive dependent
// cell width means its sign is also the full physical height-gap factor sign.
type PolynomialGraphFactor struct {
	Domain             ParameterRectangle
	Power, Bernstein   []*big.Rat
	Sign, VisitedCells int
}

func graphFreeFactorCoefficients(p *SurfacePatch, axis int, b *polynomialBudget) (rationalPolynomial, rationalPolynomial, error) {
	free, factor := make(rationalPolynomial, 9), make(rationalPolynomial, 5)
	for i := range free {
		free[i] = new(big.Rat)
	}
	for i := range factor {
		factor[i] = new(big.Rat)
	}
	columns, e := graphNumeratorPowerColumns(p, b)
	if e != nil {
		return nil, nil, e
	}
	for j, column := range columns {
		for i, x := range column {
			if x.Sign() == 0 {
				continue
			}
			a, c := i, j
			if axis == 1 {
				a, c = c, a
			}
			if c == 0 && a <= 8 {
				free[a].Set(x)
			} else if c == 1 && a <= 4 {
				factor[a].Set(x)
			} else {
				return nil, nil, ErrConvergence
			}
		}
	}
	return trimPolynomial(free), trimPolynomial(factor), nil
}
func graphFactorBernstein(p rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(p) - 1
	if n < 0 {
		return nil, ErrConvergence
	}
	out := make(rationalPolynomial, n+1)
	for i := range out {
		out[i] = new(big.Rat)
		for j := 0; j <= i; j++ {
			term := new(big.Rat).Mul(p[j], new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(n), int64(j))))
			if e := b.check(term); e != nil {
				return nil, e
			}
			out[i].Add(out[i], term)
			if e := b.check(out[i]); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}
func certifyGraphFactorSign(ctx context.Context, controls rationalPolynomial, maxDepth, maxCells int, visits *int, b *polynomialBudget) (int, error) {
	type cell struct {
		net   rationalPolynomial
		depth int
	}
	stack := []cell{{controls, 0}}
	sign := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		if *visits >= maxCells {
			return 0, ErrConvergence
		}
		*visits++
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		s := c.net[0].Sign()
		for _, v := range c.net {
			if v.Sign() != s {
				s = 0
				break
			}
		}
		if s != 0 {
			if sign != 0 && sign != s {
				return 0, ErrConvergence
			}
			sign = s
			continue
		}
		if c.depth >= maxDepth {
			return 0, ErrConvergence
		}
		n := len(c.net)
		left, right := make(rationalPolynomial, n), make(rationalPolynomial, n)
		work := clonePolynomial(c.net)
		left[0] = new(big.Rat).Set(work[0])
		right[n-1] = new(big.Rat).Set(work[n-1])
		for level := 1; level < n; level++ {
			for i := 0; i < n-level; i++ {
				work[i] = new(big.Rat).Quo(new(big.Rat).Add(work[i], work[i+1]), big.NewRat(2, 1))
				if e := b.check(work[i]); e != nil {
					return 0, e
				}
			}
			left[level] = new(big.Rat).Set(work[0])
			right[n-1-level] = new(big.Rat).Set(work[n-1-level])
		}
		stack = append(stack, cell{right, c.depth + 1}, cell{left, c.depth + 1})
	}
	return sign, nil
}
