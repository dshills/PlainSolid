package geom

import (
	"context"
	"math/big"
)

// GraphSemicubicalCusp proves the complete zero set on Domain is the image of
// normalized UV (r+t²/k,s+t³/k), -Tau<=t<=Tau. It has one cusp at t=0,
// two regular half-germs and two distinct original UMax boundary terminals.
// Side is the sign multiplying (y²-k*x³) in the homogeneous residual.
// Parameters, Terminals and Point are original chart/model values. This is
// not an isolated point contact, transverse tube, or material assembly.
type GraphSemicubicalCusp struct {
	Proven          bool
	Domain          ParameterRectangle
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	Terminals       [2][2]*big.Rat
	TerminalPoints  [2][3]*big.Rat
	Tau, K          *big.Rat
	Side            int
	Graph           SurfaceInjectivity
	ExactOperations int
}

// CertifyGraphSemicubicalCusp checks the entire shifted homogeneous numerator,
// including all forbidden coefficients. r,s are normalized original chart
// coordinates. The exact rational exit proposal tau must satisfy
// tau²=k*(1-r), with tau³/k strictly below both s and 1-s. These gates prove
// full finite-rectangle coverage, not merely a local Taylor classification.
// Conversely every noncusp zero has x>0 and unique t=y/x; then
// x=t²/k and y=t³/k. Thus no additional component can be omitted.
func CertifyGraphSemicubicalCusp(ctx context.Context, a, c *SurfacePatch, r, s, k, tau *big.Rat, o GraphPointContactOptions) (out GraphSemicubicalCusp, err error) {
	if ctx == nil || a == nil || c == nil || r == nil || s == nil || k == nil || tau == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
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
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxCells < 1 || o.MaxCells > 65536 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			out = GraphSemicubicalCusp{}
		}
		out.ExactOperations = b.operations
	}()
	for _, q := range []*big.Rat{r, s, k, tau} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	one := big.NewRat(1, 1)
	if r.Sign() <= 0 || r.Cmp(one) >= 0 || s.Sign() <= 0 || s.Cmp(one) >= 0 || k.Sign() <= 0 || tau.Sign() <= 0 {
		return out, ErrDomain
	}
	t2 := new(big.Rat).Mul(tau, tau)
	exit := new(big.Rat).Mul(k, new(big.Rat).Sub(one, r))
	dy := new(big.Rat).Quo(new(big.Rat).Mul(t2, tau), k)
	for _, q := range []*big.Rat{t2, exit, dy} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	if t2.Cmp(exit) != 0 || dy.Cmp(s) >= 0 || dy.Cmp(new(big.Rat).Sub(one, s)) >= 0 {
		return out, ErrDomain
	}
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return out, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 6 || p.degreeV > 6 {
		return out, ErrConvergence
	}
	// Convert to powers about the proposed node.
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		v := make([]*big.Rat, p.countV)
		for j := range v {
			v[j] = p.net[i*p.countV+j][2]
		}
		q, e := bernsteinToPower(v, b)
		if e != nil {
			return out, e
		}
		rows[i], e = composeProjectionPolynomial(q, rationalPolynomial{s, one}, b)
		if e != nil {
			return out, e
		}
	}
	coefficients := make(map[[2]int]*big.Rat)
	for j := 0; j < p.countV; j++ {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		v := make([]*big.Rat, p.countU)
		for i := range v {
			v[i] = new(big.Rat)
			if j < len(rows[i]) {
				v[i].Set(rows[i][j])
			}
		}
		q, e := bernsteinToPower(v, b)
		if e != nil {
			return out, e
		}
		q, e = composeProjectionPolynomial(q, rationalPolynomial{r, one}, b)
		if e != nil {
			return out, e
		}
		for i, value := range q {
			if value.Sign() != 0 {
				coefficients[[2]int{i, j}] = value
			}
		}
	}

	scale := coefficients[[2]int{0, 2}]
	if scale == nil || scale.Sign() == 0 {
		return out, ErrConvergence
	}
	expected := map[[2]int]*big.Rat{{0, 2}: scale, {3, 0}: new(big.Rat).Neg(new(big.Rat).Mul(scale, k))}
	for ij, q := range coefficients {
		v := expected[ij]
		if v == nil || v.Cmp(q) != 0 {
			return out, ErrConvergence
		}
	}
	for ij, q := range expected {
		if e := b.check(q); e != nil {
			return out, e
		}
		v := coefficients[ij]
		if v == nil || v.Cmp(q) != 0 {
			return out, ErrConvergence
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return out, ErrConvergence
	}
	out.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += out.Graph.ExactOperations
	if e != nil {
		return out, e
	}
	if !out.Graph.ProvenInjective {
		return out, ErrConvergence
	}
	d := p.Domain()
	out.Domain = d
	lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	span := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), lo[0]), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), lo[1])}
	mapUV := func(x, y *big.Rat) [2]*big.Rat {
		return [2]*big.Rat{new(big.Rat).Add(lo[0], new(big.Rat).Mul(span[0], x)), new(big.Rat).Add(lo[1], new(big.Rat).Mul(span[1], y))}
	}
	out.Parameters = mapUV(r, s)
	out.Terminals = [2][2]*big.Rat{mapUV(one, new(big.Rat).Sub(s, dy)), mapUV(one, new(big.Rat).Add(s, dy))}
	for i, uv := range [][2]*big.Rat{out.Parameters, out.Terminals[0], out.Terminals[1]} {
		for _, q := range uv {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		var points [2][3]*big.Rat
		for owner, patch := range []*SurfacePatch{a, c} {
			remaining = o.MaxOperations - b.operations
			if remaining < 1 {
				return out, ErrConvergence
			}
			jet, e := patch.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			b.operations += jet.ExactOperations
			if e != nil {
				return out, e
			}
			points[owner] = jet.Point
		}
		for j := range points[0] {
			if points[0][j].Cmp(points[1][j]) != 0 {
				return out, ErrConvergence
			}
		}
		if i == 0 {
			out.Point = points[0]
		} else {
			out.TerminalPoints[i-1] = points[0]
		}
	}
	out.Tau = new(big.Rat).Set(tau)
	out.K = new(big.Rat).Set(k)
	out.Side = scale.Sign()
	out.Proven = true
	return out, nil
}
