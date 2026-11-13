package geom

import (
	"context"
	"math/big"
)

// GraphLineParabolaNode proves the entire zero set consists of one vertical
// line and one regular parabola, meeting at exactly one ordinary double node.
// Parameters and all terminal/tangent coordinates use original chart units.
// Terminals 0,1 are the bottom/top line ends; 2,3 the left/right parabola ends.
// Thus germ pairing is exactly (0,1) and (2,3), with no extra components.
// Side is the sign of the nonzero multiplier C in the documented factorization.
// Rationals are caller-owned. The source XY projection is proved injective.
type GraphLineParabolaNode struct {
	Proven          bool
	Side            int
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	Terminals       [4][2]*big.Rat
	TerminalPoints  [4][3]*big.Rat
	Tangents        [2][2]*big.Rat
	Graph           SurfaceInjectivity
	ExactOperations int
}

// CertifyGraphLineParabolaNode proves the complete homogeneous difference
// numerator equals C*(x-r)*(y-s-m*(x-r)-k*(x-r)^2), for normalized unit chart
// coordinates x,y and supplied normalized r,s,m,k. It requires k!=0 and a
// strictly interior node, and uses strict quadratic Bernstein bounds to prove
// the whole parabola stays between the two horizontal rectangle boundaries.
// Unsupported numerators/layouts or inconclusive bounds return ErrConvergence.
// The ordinary-node claim follows from the two factor gradients (1,0) and
// (-m,1) having determinant one, independently of numerical tolerances.
func CertifyGraphLineParabolaNode(ctx context.Context, a, c *SurfacePatch, r, s, m, k *big.Rat, o GraphPointContactOptions) (out GraphLineParabolaNode, err error) {
	if ctx == nil || a == nil || c == nil || r == nil || s == nil || m == nil || k == nil {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	for _, q := range []*big.Rat{r, s, m, k} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	one := big.NewRat(1, 1)
	if r.Sign() <= 0 || r.Cmp(one) >= 0 || s.Sign() <= 0 || s.Cmp(one) >= 0 || k.Sign() == 0 {
		return out, ErrConvergence
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
	scale := coefficients[[2]int{1, 1}]
	if scale == nil || scale.Sign() == 0 {
		return out, ErrConvergence
	}
	expected := map[[2]int]*big.Rat{{1, 1}: scale, {2, 0}: new(big.Rat).Neg(new(big.Rat).Mul(scale, m)), {3, 0}: new(big.Rat).Neg(new(big.Rat).Mul(scale, k))}
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
		if q.Sign() != 0 && (v == nil || v.Cmp(q) != 0) {
			return out, ErrConvergence
		}
	}
	// q(x)=s+m(x-r)+k(x-r)^2; its three Bernstein controls bound all q.
	q0 := new(big.Rat).Add(new(big.Rat).Sub(s, new(big.Rat).Mul(m, r)), new(big.Rat).Mul(k, new(big.Rat).Mul(r, r)))
	linear := new(big.Rat).Sub(m, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(k, r)))
	q1 := new(big.Rat).Add(q0, new(big.Rat).Quo(linear, big.NewRat(2, 1)))
	q2 := new(big.Rat).Add(q0, new(big.Rat).Add(linear, k))
	for _, q := range []*big.Rat{q0, q1, q2} {
		if e := b.check(q); e != nil {
			return out, e
		}
		if q.Sign() <= 0 || q.Cmp(one) >= 0 {
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
	lo := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	span := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), lo[0]), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), lo[1])}
	mapUV := func(x, y *big.Rat) [2]*big.Rat {
		return [2]*big.Rat{new(big.Rat).Add(lo[0], new(big.Rat).Mul(span[0], x)), new(big.Rat).Add(lo[1], new(big.Rat).Mul(span[1], y))}
	}
	out.Parameters = mapUV(r, s)
	out.Terminals = [4][2]*big.Rat{mapUV(r, new(big.Rat)), mapUV(r, one), mapUV(new(big.Rat), q0), mapUV(one, q2)}
	out.Tangents = [2][2]*big.Rat{{new(big.Rat), new(big.Rat).Set(span[1])}, {new(big.Rat).Set(span[0]), new(big.Rat).Mul(span[1], m)}}
	parameters := append([][2]*big.Rat{out.Parameters}, out.Terminals[:]...)
	for i, uv := range parameters {
		for _, q := range uv {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return out, ErrConvergence
		}
		jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return out, e
		}
		if i == 0 {
			out.Point = jet.Point
		} else {
			out.TerminalPoints[i-1] = jet.Point
		}
	}
	out.Side = scale.Sign()
	out.Proven = true
	return out, nil
}
