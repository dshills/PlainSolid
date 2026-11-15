package geom

import (
	"context"
	"math/big"
)

// SplineGraphPolynomialContact proves a complete cubic or quartic original
// graph contact. Power uses the original free coordinate; Controls are original
// UV. PositiveSide is the height-gap sign toward increasing dependent coordinate.
// Transversality is cellwise; no cross-knot tangent continuity is assumed.
type SplineGraphPolynomialContact struct {
	// FreeAxis is 0 for v=f(u), 1 for u=f(v); Power uses that original coordinate.
	FreeAxis        int
	Degree          int
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

// DiscoverSplineGraphPolynomialContact independently verifies every original
// union-knot cell against one cubic/quartic graph and a nonzero constant factor.
// Axis0 means v=f(u), axis1 means u=f(v). Strict Bernstein clearance excludes
// outer dependent boundaries. Extra factors, mixed dependent terms and higher
// degree are unresolved. Original supports and their knots are never replaced.
func DiscoverSplineGraphPolynomialContact(ctx context.Context, a, c *SurfacePatch, axis int, o GraphPointContactOptions) (r SplineGraphPolynomialContact, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphPolynomialContact{}
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
	q, linear, e := graphPolynomialFactorCoefficients(cells[0], axis, budget)
	if e != nil {
		return r, e
	}
	degree := len(q) - 1
	for degree > 0 && q[degree].Sign() == 0 {
		degree--
	}
	if degree < 3 || degree > 4 || linear.Sign() == 0 {
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
		local[i] = new(big.Rat).Neg(new(big.Rat).Quo(q[i], linear))
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
	sign := 0
	for _, cell := range cells {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		d := orient(cell.Domain())
		coeff, dependent, e := graphPolynomialFactorCoefficients(cell, axis, budget)
		if e != nil {
			return r, e
		}
		if dependent.Sign() == 0 {
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
		expected := make([]*big.Rat, 5)
		for i := range expected {
			expected[i] = new(big.Rat)
			if i < len(f) {
				expected[i].Neg(f[i])
			}
		}
		expected[0].Add(expected[0], vlo)
		scale := new(big.Rat).Quo(dependent, vwidth)
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

// The difference numerator must be exactly linear in the dependent coordinate,
// with a nonzero constant coefficient and a free polynomial of degree <=4.
// Every mixed term and every omitted high-degree coefficient is checked.
func graphPolynomialFactorCoefficients(p *SurfacePatch, axis int, b *polynomialBudget) ([]*big.Rat, *big.Rat, error) {
	out := make([]*big.Rat, 5)
	for i := range out {
		out[i] = new(big.Rat)
	}
	linear := new(big.Rat)
	columns, e := graphNumeratorPowerColumns(p, b)
	if e != nil {
		return nil, nil, e
	}
	for j, column := range columns {
		for i, value := range column {
			if value.Sign() == 0 {
				continue
			}
			free, dep := i, j
			if axis == 1 {
				free, dep = dep, free
			}
			if dep == 0 && free <= 4 {
				out[free].Set(value)
			} else if dep == 1 && free == 0 {
				linear.Set(value)
			} else {
				return nil, nil, ErrConvergence
			}
		}
	}
	return out, linear, nil
}
