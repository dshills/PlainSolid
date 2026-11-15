package geom

import (
	"context"
	"math/big"
)

// SplineGraphBivariatePolynomialContact proves one complete cubic/quartic
// graph times a strictly signed cofactor linear in the dependent coordinate
// and at most quadratic in the free coordinate. Original coordinates and
// one-sided knot ownership are retained. No cross-knot smoothness is implied.
type SplineGraphBivariatePolynomialContact struct {
	// FreeAxis is 0 for v=f(u), 1 for u=f(v); Power uses that original coordinate.
	FreeAxis        int
	Degree          int
	Factors         []BivariateGraphFactor
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

// DiscoverSplineGraphBivariatePolynomialContact recovers a unique cubic/quartic
// polynomial root of the original dependent quadratic by exact discriminant
// square root and polynomial division. Every original cell is independently
// divided with zero remainder. Each cell must have a nonzero quadratic
// coefficient polynomial (isolated zeros are allowed). The cofactor is strictly
// signed on both complete dependent boundaries, which proves its sign on the
// full closed tensor cell. MaxDepth bounds dyadic free-coordinate subdivision;
// MaxCells bounds cumulative sign cells. All failures are atomic. General
// higher dependent degree and ambiguous candidate roots remain unsupported.
func DiscoverSplineGraphBivariatePolynomialContact(ctx context.Context, a, c *SurfacePatch, axis int, o GraphPointContactOptions) (r SplineGraphBivariatePolynomialContact, err error) {
	used := 0
	defer func() {
		if err != nil {
			r = SplineGraphBivariatePolynomialContact{}
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
	firstCoefficients, e := graphBivariateCoefficients(cells[0], axis, budget)
	if e != nil {
		return r, e
	}
	root, e := recoverBivariateGraphRoot(firstCoefficients, budget)
	if e != nil {
		return r, e
	}
	degree := len(root) - 1
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
		local[i] = new(big.Rat).Set(root[i])
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

	for _, cell := range cells {
		if e = ctx.Err(); e != nil {
			return r, e
		}
		d := orient(cell.Domain())
		coeff, e := graphBivariateCoefficients(cell, axis, budget)
		if e != nil {
			return r, e
		}
		origin := new(big.Rat).SetFloat64(d.UMin)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), origin)
		f, e := composeProjectionPolynomial(r.Power[:], rationalPolynomial{origin, width}, budget)
		if e != nil {
			return r, e
		}
		vlo := new(big.Rat).SetFloat64(d.VMin)
		vwidth := new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), vlo)
		localRoot := clonePolynomial(f)
		localRoot[0].Sub(localRoot[0], vlo)
		for _, x := range localRoot {
			x.Quo(x, vwidth)
			if e = budget.check(x); e != nil {
				return r, e
			}
		}
		aRoot, e := polynomialProduct(coeff[2], localRoot, budget)
		if e != nil {
			return r, e
		}
		constant, e := graphPolynomialSum(coeff[1], aRoot, 1, budget)
		if e != nil {
			return r, e
		}
		if len(constant) > 3 || len(coeff[2]) > 3 {
			return r, ErrConvergence
		}
		remainder, e := polynomialProduct(localRoot, constant, budget)
		if e != nil {
			return r, e
		}
		remainder, e = graphPolynomialSum(coeff[0], remainder, 1, budget)
		if e != nil {
			return r, e
		}
		if len(remainder) != 0 {
			return r, ErrConvergence
		}
		upper, e := graphPolynomialSum(constant, coeff[2], 1, budget)
		if e != nil {
			return r, e
		}
		factor := BivariateGraphFactor{Domain: cell.Domain(), EndpointPower: [2][]*big.Rat{clonePolynomial(constant), clonePolynomial(upper)}}
		before := visits
		for side, poly := range []rationalPolynomial{constant, upper} {
			controls, e := graphFactorBernstein(poly, budget)
			if e != nil {
				return r, e
			}
			factor.EndpointBernstein[side] = clonePolynomial(controls)
			sideSign, e := certifyGraphFactorSign(ctx, controls, o.MaxDepth, o.MaxCells, &visits, budget)
			if e != nil {
				return r, e
			}
			if sign != 0 && sign != sideSign {
				return r, ErrConvergence
			}
			sign = sideSign
			factor.Sign = sideSign
		}
		factor.VisitedCells = visits - before
		r.Factors = append(r.Factors, factor)
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

// BivariateGraphFactor records the cofactor on each dependent boundary in
// normalized free-coordinate power/Bernstein form. The cofactor of normalized
// dependent-root differs from the physical one by a positive cell width only.
type BivariateGraphFactor struct {
	Domain                           ParameterRectangle
	EndpointPower, EndpointBernstein [2][]*big.Rat
	Sign, VisitedCells               int
}

func graphBivariateCoefficients(p *SurfacePatch, axis int, b *polynomialBudget) ([3]rationalPolynomial, error) {
	var out [3]rationalPolynomial
	for j := range out {
		out[j] = make(rationalPolynomial, 7)
		for i := range out[j] {
			out[j][i] = new(big.Rat)
		}
	}
	columns, e := graphNumeratorPowerColumns(p, b)
	if e != nil {
		return out, e
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
			if c > 2 || a > 6 {
				return out, ErrConvergence
			}
			out[c][a].Set(x)
		}
	}
	for i := range out {
		out[i] = trimPolynomial(out[i])
	}
	if len(out[2]) == 0 || len(out[2]) > 3 {
		return out, ErrConvergence
	}
	return out, nil
}
func graphPolynomialSum(a, c rationalPolynomial, sign int, b *polynomialBudget) (rationalPolynomial, error) {
	out := make(rationalPolynomial, max(len(a), len(c)))
	for i := range out {
		out[i] = new(big.Rat)
		if i < len(a) {
			out[i].Set(a[i])
		}
		if i < len(c) {
			if sign > 0 {
				out[i].Add(out[i], c[i])
			} else {
				out[i].Sub(out[i], c[i])
			}
		}
		if e := b.check(out[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(out), nil
}
func graphExactPolynomialSqrt(p rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	if len(p) == 0 || (len(p)-1)%2 != 0 || len(p) > 13 {
		return nil, ErrConvergence
	}
	n := (len(p) - 1) / 2
	lead := p[len(p)-1]
	if lead.Sign() <= 0 {
		return nil, ErrConvergence
	}
	num, den := new(big.Int).Sqrt(lead.Num()), new(big.Int).Sqrt(lead.Denom())
	if new(big.Int).Mul(num, num).Cmp(lead.Num()) != 0 || new(big.Int).Mul(den, den).Cmp(lead.Denom()) != 0 {
		return nil, ErrConvergence
	}
	root := make(rationalPolynomial, n+1)
	for i := range root {
		root[i] = new(big.Rat)
	}
	root[n].SetFrac(num, den)
	if e := b.check(root[n]); e != nil {
		return nil, e
	}
	divisor := new(big.Rat).Mul(big.NewRat(2, 1), root[n])
	if e := b.check(divisor); e != nil {
		return nil, e
	}
	for k := n - 1; k >= 0; k-- {
		value := new(big.Rat).Set(p[n+k])
		for i := k + 1; i <= n; i++ {
			j := n + k - i
			if j > k && j <= n {
				value.Sub(value, new(big.Rat).Mul(root[i], root[j]))
				if e := b.check(value); e != nil {
					return nil, e
				}
			}
		}
		root[k].Quo(value, divisor)
		if e := b.check(root[k]); e != nil {
			return nil, e
		}
	}
	square, e := polynomialProduct(root, root, b)
	if e != nil {
		return nil, e
	}
	if len(square) != len(p) {
		return nil, ErrConvergence
	}
	for i := range p {
		if p[i].Cmp(square[i]) != 0 {
			return nil, ErrConvergence
		}
	}
	return root, nil
}
func recoverBivariateGraphRoot(c [3]rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	bb, e := polynomialProduct(c[1], c[1], b)
	if e != nil {
		return nil, e
	}
	ac, e := polynomialProduct(c[0], c[2], b)
	if e != nil {
		return nil, e
	}
	for _, x := range ac {
		x.Mul(x, big.NewRat(4, 1))
		if e = b.check(x); e != nil {
			return nil, e
		}
	}
	disc, e := graphPolynomialSum(bb, ac, -1, b)
	if e != nil {
		return nil, e
	}
	sqrt, e := graphExactPolynomialSqrt(disc, b)
	if e != nil {
		return nil, e
	}
	divisor := clonePolynomial(c[2])
	for _, x := range divisor {
		x.Mul(x, big.NewRat(2, 1))
		if e = b.check(x); e != nil {
			return nil, e
		}
	}
	var root rationalPolynomial
	for _, sign := range []int{-1, 1} {
		numerator := clonePolynomial(c[1])
		for _, x := range numerator {
			x.Neg(x)
		}
		numerator, e = graphPolynomialSum(numerator, sqrt, sign, b)
		if e != nil {
			return nil, e
		}
		q, rem, e := polynomialDivide(numerator, divisor, b)
		if e != nil {
			return nil, e
		}
		if len(rem) != 0 || len(q) < 4 || len(q) > 5 {
			continue
		}
		if root != nil {
			return nil, ErrConvergence
		}
		root = q
	}
	if root == nil {
		return nil, ErrConvergence
	}
	return root, nil
}
