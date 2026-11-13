package geom

import (
	"context"
	"math/big"
)

// SeparableGraphContactOptions bounds exact univariate root counting, common-XY
// proof and XY injectivity. MaxRootSubdivisions defaults to8192; other zero
// limits use GraphPointContactOptions defaults.
type SeparableGraphContactOptions struct {
	GraphPointContactOptions
	MaxRootSubdivisions int
}

// SeparableSplineGraphContact proves one isolated common graph point and the
// strict sign of A.Z-B.Z everywhere else on the closed rectangle. Orders are
// exact vanishing orders in the two independent parameter directions. No
// definite Hessian is required. Multi-span numerators require exact C1
// compatibility; Orders is the minimum incident order at a knot witness.
// Nonseparable numerators and genuine derivative jumps remain unresolved.
type SeparableSplineGraphContact struct {
	Proven                            bool
	Side                              int
	Orders                            [2]int
	Parameters                        [2]*big.Rat
	Point                             [3]*big.Rat
	Graph                             SurfaceInjectivity
	ExactOperations, RootSubdivisions int
}

// CertifySeparableSplineGraphPointContact proves F(u,v)=f(u)+g(v) for the exact
// homogeneous height numerator, F(witness)=0, and exactly one root of each
// univariate slice with matching strict signs elsewhere. Positive weights then
// preserve that sign; XY injectivity rules out other-parameter spatial contacts.
// Thus quartic/higher even-order zero-curvature tangencies can be certified.
func CertifySeparableSplineGraphPointContact(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o SeparableGraphContactOptions) (r SeparableSplineGraphContact, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || uv[0] == nil || uv[1] == nil {
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
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for _, x := range uv {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	p, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if _, _, ok := patchSplitChoice(p); ok {
		return certifyMultispanSeparableContact(ctx, a, p, uv, o, b)
	}
	d := p.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	normalized := [2]*big.Rat{}
	for axis, x := range uv {
		lo, hi := new(big.Rat).SetFloat64(limits[axis][0]), new(big.Rat).SetFloat64(limits[axis][1])
		if x.Cmp(lo) < 0 || x.Cmp(hi) > 0 {
			return r, ErrDomain
		}
		normalized[axis] = new(big.Rat).Quo(new(big.Rat).Sub(x, lo), new(big.Rat).Sub(hi, lo))
	}
	values := make([]*big.Rat, len(p.net))
	for i, h := range p.net {
		values[i] = h[2]
	}
	value, e := tensorScalarValue(values, p.countU, p.countV, normalized[0], normalized[1], b)
	if e != nil {
		return r, e
	}
	if value.Sign() != 0 {
		return r, ErrConvergence
	}
	// Tensor power coefficients give an exact necessary/sufficient separability
	// identity; cancellation is tested algebraically, never from sample values.
	rows := make([]rationalPolynomial, p.countU)
	for i := range rows {
		rows[i], e = bernsteinToPower(values[i*p.countV:(i+1)*p.countV], b)
		if e != nil {
			return r, e
		}
	}
	for j := 1; j < p.countV; j++ {
		column := make([]*big.Rat, p.countU)
		for i := range column {
			column[i] = new(big.Rat)
			if j < len(rows[i]) {
				column[i].Set(rows[i][j])
			}
		}
		power, e := bernsteinToPower(column, b)
		if e != nil {
			return r, e
		}
		for i := 1; i < len(power); i++ {
			if power[i].Sign() != 0 {
				return r, ErrConvergence
			}
		}
	}
	slices := [2][]*big.Rat{make([]*big.Rat, p.countU), make([]*big.Rat, p.countV)}
	for i := 0; i < p.countU; i++ {
		slices[0][i], e = scalarBernsteinValue(values[i*p.countV:(i+1)*p.countV], normalized[1], b)
		if e != nil {
			return r, e
		}
	}
	for j := 0; j < p.countV; j++ {
		column := make([]*big.Rat, p.countU)
		for i := range column {
			column[i] = values[i*p.countV+j]
		}
		slices[1][j], e = scalarBernsteinValue(column, normalized[0], b)
		if e != nil {
			return r, e
		}
	}
	commonSide := 0
	for axis, coefficients := range slices {
		poly, e := bernsteinToPower(coefficients, b)
		if e != nil {
			return r, e
		}
		if len(poly) == 0 {
			return r, ErrConvergence
		}
		zero, e := polynomialValue(poly, normalized[axis], b)
		if e != nil {
			return r, e
		}
		if zero.Sign() != 0 {
			return r, ErrConvergence
		}
		count, order := 0, 0
		remaining := o.MaxRootSubdivisions - r.RootSubdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(poly, new(big.Rat), big.NewRat(1, 1), b, remaining, func(_, _ *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			count++
			order = m
			return true, nil
		})
		r.RootSubdivisions += steps
		if e != nil {
			return r, e
		}
		if count != 1 {
			return r, ErrConvergence
		}
		side := 0
		root := normalized[axis]
		var samples []*big.Rat
		if root.Sign() > 0 {
			samples = append(samples, new(big.Rat).Mul(root, big.NewRat(1, 2)))
		}
		if root.Cmp(big.NewRat(1, 1)) < 0 {
			samples = append(samples, new(big.Rat).Mul(new(big.Rat).Add(root, big.NewRat(1, 1)), big.NewRat(1, 2)))
		}
		for _, sample := range samples {
			value, e := polynomialValue(poly, sample, b)
			if e != nil {
				return r, e
			}
			if value.Sign() == 0 {
				return r, ErrConvergence
			}
			if side != 0 && side != value.Sign() {
				return r, ErrConvergence
			}
			side = value.Sign()
		}
		if side == 0 || (commonSide != 0 && side != commonSide) {
			return r, ErrConvergence
		}
		commonSide = side
		r.Orders[axis] = order
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
	r.Side = commonSide
	r.Proven = true
	return r, nil
}
