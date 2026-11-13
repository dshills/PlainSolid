package geom

import (
	"context"
	"math/big"
)

// ExactGraphLineRoot retains immutable exact polynomial and root-isolation
// provenance in the global normalized parameter of a selected curve piece.
// Equality of these roots means parameter equality, not model-point equality
// unless callers independently establish the same XY path for both curves.
type ExactGraphLineRoot struct {
	polynomial               rationalPolynomial
	bounds                   [2]*big.Rat
	operations, subdivisions int
}

// Interval returns an independent closed rational enclosure of the unique root.
func (r *ExactGraphLineRoot) Interval() ExactParameterInterval {
	if r == nil || r.bounds[0] == nil {
		return ExactParameterInterval{}
	}
	return ExactParameterInterval{Lower: new(big.Rat).Set(r.bounds[0]), Upper: new(big.Rat).Set(r.bounds[1])}
}
func (r *ExactGraphLineRoot) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}
func (r *ExactGraphLineRoot) Subdivisions() int {
	if r == nil {
		return 0
	}
	return r.subdivisions
}

// NewExactGraphLineRoot proves one distinct root of curve.Z-height(t) on a
// selected exact Bézier piece, where height(t)=start+(end-start)*t uses global
// normalized t. The supplied bracket is intersected with the piece interval;
// its root count must be exactly one. This proves only the selected piece root,
// not completeness over a collection. Repeated and endpoint roots are allowed;
// identically coincident spans and nonunique intervals remain unresolved.
func NewExactGraphLineRoot(ctx context.Context, piece SurfaceAffineCurvePiece, start, end *big.Rat, interval ExactParameterInterval, o ExactCurvePlaneEventOptions) (*ExactGraphLineRoot, error) {
	if ctx == nil || piece.Curve == nil || piece.Curve.patch == nil || piece.Start == nil || piece.End == nil || start == nil || end == nil || interval.Lower == nil || interval.Upper == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if piece.Start.Sign() < 0 || piece.End.Cmp(big.NewRat(1, 1)) > 0 || piece.Start.Cmp(piece.End) >= 0 || interval.Lower.Cmp(interval.Upper) > 0 {
		return nil, ErrInvalid
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	p := piece.Curve.patch
	lo, hi := piece.Curve.Domain()
	if lo != 0 || hi != 1 || p.countU != p.degreeU+1 {
		return nil, ErrConvergence
	}
	for _, x := range []*big.Rat{piece.Start, piece.End, start, end, interval.Lower, interval.Upper} {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	bounds := [2]*big.Rat{new(big.Rat).Set(interval.Lower), new(big.Rat).Set(interval.Upper)}
	if bounds[0].Cmp(piece.Start) < 0 {
		bounds[0].Set(piece.Start)
	}
	if bounds[1].Cmp(piece.End) > 0 {
		bounds[1].Set(piece.End)
	}
	if bounds[0].Cmp(bounds[1]) > 0 {
		return nil, ErrDomain
	}
	global, e := exactGraphHeightPolynomial(piece, start, end, b)
	if e != nil {
		return nil, e
	}
	if len(global) == 0 {
		return nil, ErrConvergence
	}
	count := 0
	steps, e := isolatePolynomialRoots(global, bounds[0], bounds[1], b, o.MaxSubdivisions, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
	if e != nil {
		return nil, e
	}
	if count != 1 {
		return nil, ErrConvergence
	}
	r := &ExactGraphLineRoot{bounds: bounds, operations: b.operations, subdivisions: steps}
	r.polynomial = make(rationalPolynomial, len(global))
	for i, x := range global {
		r.polynomial[i] = new(big.Rat).Set(x)
	}
	return r, nil
}

// CompareExactGraphLineRoots rechecks both unique-root enclosures. A common
// polynomial GCD root in their overlap proves equality; otherwise bounded exact
// refinement proves strict order. Overlapping numeric boxes never imply equal.
func CompareExactGraphLineRoots(ctx context.Context, a, c *ExactGraphLineRoot, o ExactCurvePlaneEventOptions) (r ExactCurveEventOrder, err error) {
	if ctx == nil || a == nil || c == nil || len(a.polynomial) == 0 || len(c.polynomial) == 0 || a.bounds[0] == nil || c.bounds[0] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	var polys [2]rationalPolynomial
	var bounds [2][2]*big.Rat
	for i, root := range []*ExactGraphLineRoot{a, c} {
		polys[i] = make(rationalPolynomial, len(root.polynomial))
		for j, x := range root.polynomial {
			if e := b.check(x); e != nil {
				return r, e
			}
			polys[i][j] = new(big.Rat).Set(x)
		}
		for j, x := range root.bounds {
			if e := b.check(x); e != nil {
				return r, e
			}
			bounds[i][j] = new(big.Rat).Set(x)
		}
	}
	return compareExactPolynomialEvents(ctx, polys, bounds, b, o.MaxSubdivisions)
}
func graphRootOptions(o ExactCurvePlaneEventOptions) (ExactCurvePlaneEventOptions, error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return o, ErrInvalid
	}
	return o, nil
}

func exactGraphHeightPolynomial(piece SurfaceAffineCurvePiece, start, end *big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	p := piece.Curve.patch
	numerator, weights := make([]*big.Rat, p.countU), make([]*big.Rat, p.countU)
	for i := range numerator {
		h := p.net[i*p.countV]
		numerator[i] = h[2]
		weights[i] = h[3]
	}
	f, e := bernsteinToPower(numerator, b)
	if e != nil {
		return nil, e
	}
	w, e := bernsteinToPower(weights, b)
	if e != nil {
		return nil, e
	}
	width := new(big.Rat).Sub(piece.End, piece.Start)
	delta := new(big.Rat).Sub(end, start)
	height := rationalPolynomial{new(big.Rat).Add(start, new(big.Rat).Mul(delta, piece.Start)), new(big.Rat).Mul(delta, width)}
	g, e := polynomialProduct(w, height, b)
	if e != nil {
		return nil, e
	}
	local := make(rationalPolynomial, max(len(f), len(g)))
	for i := range local {
		local[i] = new(big.Rat)
		if i < len(f) {
			local[i].Set(f[i])
		}
		if i < len(g) {
			local[i].Sub(local[i], g[i])
		}
		if e := b.check(local[i]); e != nil {
			return nil, e
		}
	}
	local = trimPolynomial(local)
	if len(local) == 0 {
		return nil, nil
	}
	global, e := composeProjectionPolynomial(local, rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Quo(piece.Start, width)), new(big.Rat).Inv(width)}, b)
	if e != nil {
		return nil, e
	}
	return global, nil
}
