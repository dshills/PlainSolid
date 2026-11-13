package geom

import (
	"context"
	"math/big"
)

// CoupledQuarticGraphContact proves exactly one contact on the full common
// rectangular graph domain. Side*(ZA-ZB)>0 everywhere except Parameters.
// Coefficients describe the homogeneous numerator in normalized chart offsets
// x=normalizedU-witnessU,y=normalizedV-witnessV: A*x^4+B*x²*y²+C*y^4.
// No nonzero Hessian is required. All rational output values are caller-owned.
type CoupledQuarticGraphContact struct {
	Proven          bool
	Side            int
	Coefficients    [3]*big.Rat
	Parameters      [2]*big.Rat
	Point           [3]*big.Rat
	Graph           SurfaceInjectivity
	ExactOperations int
}

// CertifyCoupledQuarticGraphPointContact recognizes the entire single-cell
// numerator, proves strict positivity of the quartic away from zero, and proves
// XY injectivity to exclude other-parameter model contacts. Positive A,C with
// B>=0 or B²<4AC suffice, with a common negative sign allowed. Semidefinite
// equality, saddles, extra terms and unsupported knot layouts stay unresolved.
func CertifyCoupledQuarticGraphPointContact(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o GraphPointContactOptions) (r CoupledQuarticGraphContact, err error) {
	if ctx == nil || a == nil || c == nil || uv[0] == nil || uv[1] == nil {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for _, q := range uv {
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
	domain := p.Domain()
	ranges := [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
	var center [2]*big.Rat
	for axis, q := range uv {
		lo, hi := new(big.Rat).SetFloat64(ranges[axis][0]), new(big.Rat).SetFloat64(ranges[axis][1])
		if q.Cmp(lo) < 0 || q.Cmp(hi) > 0 {
			return r, ErrDomain
		}
		center[axis] = new(big.Rat).Quo(new(big.Rat).Sub(q, lo), new(big.Rat).Sub(hi, lo))
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
		column, e := composeProjectionPolynomial(power, rationalPolynomial{center[0], big.NewRat(1, 1)}, b)
		if e != nil {
			return r, e
		}
		for i, q := range column {
			if q.Sign() == 0 {
				continue
			}
			index := -1
			if i == 4 && j == 0 {
				index = 0
			} else if i == 2 && j == 2 {
				index = 1
			} else if i == 0 && j == 4 {
				index = 2
			}
			if index < 0 {
				return r, ErrConvergence
			}
			r.Coefficients[index].Set(q)
		}
	}
	side := r.Coefficients[0].Sign()
	if side == 0 || r.Coefficients[2].Sign() != side {
		return r, ErrConvergence
	}
	aa, bb, cc := new(big.Rat).Set(r.Coefficients[0]), new(big.Rat).Set(r.Coefficients[1]), new(big.Rat).Set(r.Coefficients[2])
	if side < 0 {
		aa.Neg(aa)
		bb.Neg(bb)
		cc.Neg(cc)
	}
	if bb.Sign() < 0 {
		discriminant := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(aa, cc)), new(big.Rat).Mul(bb, bb))
		if e := b.check(discriminant); e != nil {
			return r, e
		}
		if discriminant.Sign() <= 0 {
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
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += jet.ExactOperations
	if e != nil {
		return r, e
	}
	r.Point = jet.Point
	r.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.Side = side
	r.Proven = true
	return r, nil
}
