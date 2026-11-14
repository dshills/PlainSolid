package geom

import (
	"context"
	"math/big"
)

// QuarticGramGraphContact proves exactly one contact on Domain, with
// Side*(ZA-ZB)>0 elsewhere. Order is4: the exact height difference and its
// first three Taylor orders vanish at Parameters, with a strictly definite
// fourth order. Tangent contains the common original Du,Dv (not normalized).
// Coefficients are the original homogeneous numerator coefficients of
// x⁴,x³y,x²y²,xy³,y⁴, in normalized original-chart offsets from Parameters.
// Gram is positive definite and represents Side times that numerator on
// (x²,xy,y²). All rational values are owned copies; failure publishes only work.
type QuarticGramGraphContact struct {
	Proven          bool
	Side, Order     int
	Domain          ParameterRectangle
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	Tangent         [2][3]*big.Rat
	Coefficients    [5]*big.Rat
	Gram            [3][3]*big.Rat
	PrincipalMinors [3]*big.Rat
	Graph           SurfaceInjectivity
	ExactOperations int
}

// CertifyQuarticGramGraphPointContact recognizes the full shifted quartic
// numerator, including odd mixed terms. gramCross proposes the (0,2) Gram
// entry for the positive-side form; the other entries are fixed by the exact
// coefficients, including G11=Side*C-2*gramCross. Exact Sylvester minors must
// all be positive. This is a sufficient proof, not a search over Gram matrices.
// A strict interior rational witness, common injective XY projection and exact
// common tangent jets are required. Semidefinite/saddle forms, extra terms,
// unsupported knot layouts and boundary witnesses fail closed. No float witness
// conversion or perturbation is used.
func CertifyQuarticGramGraphPointContact(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, gramCross *big.Rat, o GraphPointContactOptions) (r QuarticGramGraphContact, err error) {
	if ctx == nil || a == nil || c == nil || uv[0] == nil || uv[1] == nil || gramCross == nil {
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
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxCells < 1 || o.MaxCells > 65536 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		if err != nil {
			r = QuarticGramGraphContact{}
		}
		r.ExactOperations = b.operations
	}()
	for _, q := range []*big.Rat{uv[0], uv[1], gramCross} {
		if e := b.check(q); e != nil {
			return r, e
		}
	}
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || p.degreeU > 6 || p.degreeV > 6 {
		return r, ErrConvergence
	}
	d := p.Domain()
	r.Domain = d
	var center [2]*big.Rat
	for axis, ends := range [][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		lo, hi := new(big.Rat).SetFloat64(ends[0]), new(big.Rat).SetFloat64(ends[1])
		if uv[axis].Cmp(lo) <= 0 || uv[axis].Cmp(hi) >= 0 {
			return r, ErrDomain
		}
		q := new(big.Rat).Sub(uv[axis], lo)
		if e = b.check(q); e != nil {
			return r, e
		}
		width := new(big.Rat).Sub(hi, lo)
		if e = b.check(width); e != nil {
			return r, e
		}
		center[axis] = new(big.Rat).Quo(q, width)
		if e = b.check(center[axis]); e != nil {
			return r, e
		}
	}
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		values := make([]*big.Rat, p.countV)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		power, e := bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
		rows[i], e = composeProjectionPolynomial(power, rationalPolynomial{center[1], big.NewRat(1, 1)}, b)
		if e != nil {
			return r, e
		}
	}
	for i := range r.Coefficients {
		r.Coefficients[i] = new(big.Rat)
	}
	for j := 0; j < p.countV; j++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		values := make([]*big.Rat, p.countU)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		power, e := bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
		shifted, e := composeProjectionPolynomial(power, rationalPolynomial{center[0], big.NewRat(1, 1)}, b)
		if e != nil {
			return r, e
		}
		for i, q := range shifted {
			if q.Sign() == 0 {
				continue
			}
			if i+j != 4 || i > 4 || j > 4 {
				return r, ErrConvergence
			}
			r.Coefficients[j].Set(q)
		}
	}
	side := r.Coefficients[0].Sign()
	if side == 0 {
		return r, ErrConvergence
	}
	var positive [5]*big.Rat
	for i, q := range r.Coefficients {
		positive[i] = new(big.Rat).Mul(q, big.NewRat(int64(side), 1))
		if e = b.check(positive[i]); e != nil {
			return r, e
		}
	}
	cross := new(big.Rat).Set(gramCross)
	xy := new(big.Rat).Quo(positive[1], big.NewRat(2, 1))
	yz := new(big.Rat).Quo(positive[3], big.NewRat(2, 1))
	twice := new(big.Rat).Mul(big.NewRat(2, 1), cross)
	middle := new(big.Rat).Sub(positive[2], twice)
	for _, q := range []*big.Rat{xy, yz, twice, middle} {
		if e = b.check(q); e != nil {
			return r, e
		}
	}
	r.Gram = [3][3]*big.Rat{{positive[0], xy, cross}, {new(big.Rat).Set(xy), middle, yz}, {new(big.Rat).Set(cross), new(big.Rat).Set(yz), positive[4]}}
	r.PrincipalMinors, e = quarticGramMinors(r.Gram, b)
	if e != nil {
		return r, e
	}
	for _, q := range r.PrincipalMinors {
		if q.Sign() <= 0 {
			return r, ErrConvergence
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	// Evaluate both original maps independently; the returned jets must agree.
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	ja, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += ja.ExactOperations
	if e != nil {
		return r, e
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	jc, e := c.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += jc.ExactOperations
	if e != nil {
		return r, e
	}
	for axis := range 3 {
		if ja.Point[axis].Cmp(jc.Point[axis]) != 0 || ja.Du[axis].Cmp(jc.Du[axis]) != 0 || ja.Dv[axis].Cmp(jc.Dv[axis]) != 0 {
			return r, ErrConvergence
		}
		r.Point[axis] = new(big.Rat).Set(ja.Point[axis])
		r.Tangent[0][axis] = new(big.Rat).Set(ja.Du[axis])
		r.Tangent[1][axis] = new(big.Rat).Set(ja.Dv[axis])
	}
	r.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.Side = side
	r.Order = 4
	r.Proven = true
	return r, nil
}

func quarticGramMinors(g [3][3]*big.Rat, b *polynomialBudget) (out [3]*big.Rat, err error) {
	minor := func(a, c, d, e *big.Rat) (*big.Rat, error) {
		x := new(big.Rat).Mul(a, e)
		if err := b.check(x); err != nil {
			return nil, err
		}
		y := new(big.Rat).Mul(c, d)
		if err := b.check(y); err != nil {
			return nil, err
		}
		x.Sub(x, y)
		if err := b.check(x); err != nil {
			return nil, err
		}
		return x, nil
	}
	out[0] = new(big.Rat).Set(g[0][0])
	out[1], err = minor(g[0][0], g[0][1], g[1][0], g[1][1])
	if err != nil {
		return out, err
	}
	out[2] = new(big.Rat)
	for j := range 3 {
		columns := []int{}
		for k := range 3 {
			if k != j {
				columns = append(columns, k)
			}
		}
		m, e := minor(g[1][columns[0]], g[1][columns[1]], g[2][columns[0]], g[2][columns[1]])
		if e != nil {
			return out, e
		}
		term := new(big.Rat).Mul(g[0][j], m)
		if e = b.check(term); e != nil {
			return out, e
		}
		if j%2 == 0 {
			out[2].Add(out[2], term)
		} else {
			out[2].Sub(out[2], term)
		}
		if e = b.check(out[2]); e != nil {
			return out, e
		}
	}
	return out, nil
}
