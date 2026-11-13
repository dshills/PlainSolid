package geom

import (
	"context"
	"math/big"
)

// ExactRootRecipe is a versioned serializable algebraic-number description.
// Coefficients are ascending powers, encoded as exact rational strings; exports
// use primitive square-free integer coefficients with positive leading term.
// Lower/Upper are canonical rational strings. This normalization is not a
// minimal-polynomial claim: different recipes can represent the same root.
// Import proves only numeric root identity, not association with a CAD entity.
type ExactRootRecipe struct {
	Version      int      `json:"version"`
	Coefficients []string `json:"coefficients"`
	Lower        string   `json:"lower"`
	Upper        string   `json:"upper"`
}
type ExactRootCodecOptions struct {
	ExactCurvePlaneEventOptions
	MaxDegree, MaxTextBytes int
}

// ExactAlgebraicRoot is immutable after validated import.
type ExactAlgebraicRoot struct{ root *ExactGraphLineRoot }

func (r *ExactAlgebraicRoot) Interval() ExactParameterInterval {
	if r == nil {
		return ExactParameterInterval{}
	}
	return r.root.Interval()
}
func (r *ExactAlgebraicRoot) Compare(ctx context.Context, other *ExactAlgebraicRoot, o ExactCurvePlaneEventOptions) (ExactCurveEventOrder, error) {
	if r == nil || other == nil {
		return ExactCurveEventOrder{}, ErrInvalid
	}
	return CompareExactGraphLineRoots(ctx, r.root, other.root, o)
}

type ExactRootKind int

const (
	ExactRootKindUnknown ExactRootKind = iota
	ExactRootKindRational
	ExactRootKindIrrational
)

// ExactRootClassification never infers irrationality from failed recovery.
// Quadratic nonsquare discriminants prove irrationality; higher degrees may
// remain Unknown unless an exact rational root is recovered.
type ExactRootClassification struct {
	Kind       ExactRootKind
	Rational   *big.Rat
	Operations int
}

func (r *ExactAlgebraicRoot) Classify(ctx context.Context, o ExactCurvePlaneEventOptions) (out ExactRootClassification, err error) {
	if ctx == nil || r == nil || r.root == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return out, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.Operations = b.operations }()
	for _, x := range r.root.polynomial {
		if e := b.check(x); e != nil {
			return out, e
		}
	}
	rational, e := recoverRationalPolynomialRoot(r.root.polynomial, r.root.bounds[0], r.root.bounds[1], b)
	if e != nil {
		return out, e
	}
	if rational != nil {
		out.Kind = ExactRootKindRational
		out.Rational = new(big.Rat).Set(rational)
		return out, nil
	}
	p := r.root.polynomial
	if len(p) == 3 {
		discriminant := new(big.Rat).Sub(new(big.Rat).Mul(p[1], p[1]), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(p[0], p[2])))
		if e := b.check(discriminant); e != nil {
			return out, e
		}
		if discriminant.Sign() > 0 {
			n, d := new(big.Int).Sqrt(discriminant.Num()), new(big.Int).Sqrt(discriminant.Denom())
			if new(big.Int).Mul(n, n).Cmp(discriminant.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(discriminant.Denom()) != 0 {
				out.Kind = ExactRootKindIrrational
			}
		}
	}
	return out, nil
}
func exactRootCodecOptions(o ExactRootCodecOptions) (ExactRootCodecOptions, error) {
	var e error
	o.ExactCurvePlaneEventOptions, e = graphRootOptions(o.ExactCurvePlaneEventOptions)
	if e != nil {
		return o, e
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 128
	}
	if o.MaxTextBytes == 0 {
		o.MaxTextBytes = 1048576
	}
	if o.MaxDegree < 1 || o.MaxDegree > 1024 || o.MaxTextBytes < 1 {
		return o, ErrInvalid
	}
	return o, nil
}

// ImportExactRoot parses bounded exact strings, rejects nonunique/empty root
// intervals, and owns all imported data. Unknown versions are rejected.
func ImportExactRoot(ctx context.Context, recipe ExactRootRecipe, o ExactRootCodecOptions) (*ExactAlgebraicRoot, error) {
	if ctx == nil || recipe.Version != 1 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := exactRootCodecOptions(o)
	if e != nil {
		return nil, e
	}
	if len(recipe.Coefficients) < 2 || len(recipe.Coefficients)-1 > o.MaxDegree {
		return nil, ErrConvergence
	}
	total := len(recipe.Lower) + len(recipe.Upper)
	if total > o.MaxTextBytes {
		return nil, ErrConvergence
	}
	for _, s := range recipe.Coefficients {
		if len(s) > o.MaxTextBytes-total {
			return nil, ErrConvergence
		}
		total += len(s)
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	parse := func(s string) (*big.Rat, error) {
		if len(s) == 0 || len(s) > o.MaxCoefficientBits*2+32 {
			return nil, ErrConvergence
		}
		// Only integer or integer/integer syntax: exponent forms could ask
		// math/big to allocate an unbounded power despite a short input string.
		start, digits, slashes := true, 0, 0
		for _, ch := range s {
			if (ch == '+' || ch == '-') && start {
				start = false
				continue
			}
			if ch == '/' {
				if digits == 0 || slashes != 0 {
					return nil, ErrInvalid
				}
				slashes++
				start = true
				digits = 0
				continue
			}
			if ch < '0' || ch > '9' {
				return nil, ErrInvalid
			}
			start = false
			digits++
		}
		if digits == 0 {
			return nil, ErrInvalid
		}
		x, ok := new(big.Rat).SetString(s)
		if !ok {
			return nil, ErrInvalid
		}
		if e := b.check(x); e != nil {
			return nil, e
		}
		return x, nil
	}
	p := make(rationalPolynomial, len(recipe.Coefficients))
	for i, s := range recipe.Coefficients {
		p[i], e = parse(s)
		if e != nil {
			return nil, e
		}
	}
	lo, e := parse(recipe.Lower)
	if e != nil {
		return nil, e
	}
	hi, e := parse(recipe.Upper)
	if e != nil {
		return nil, e
	}
	if lo.Cmp(hi) > 0 {
		return nil, ErrInvalid
	}
	root, e := canonicalExactRoot(ctx, p, [2]*big.Rat{lo, hi}, o, b)
	if e != nil {
		return nil, e
	}
	return &ExactAlgebraicRoot{root: root}, nil
}

// Export canonicalizes coefficients and verifies one root again under the
// supplied limits. All text and slices are independent of source provenance.
func (r *ExactAlgebraicRoot) Export(ctx context.Context, o ExactRootCodecOptions) (ExactRootRecipe, error) {
	if r == nil || r.root == nil {
		return ExactRootRecipe{}, ErrInvalid
	}
	return exportExactRoot(ctx, r.root, o)
}
func (e *ExactComposedPlaneEvent) RootRecipe(ctx context.Context, o ExactRootCodecOptions) (ExactRootRecipe, error) {
	if e == nil || e.root == nil {
		return ExactRootRecipe{}, ErrInvalid
	}
	return exportExactRoot(ctx, e.root, o)
}
func (c *ExactComposedCurveCell) EndpointRootRecipe(ctx context.Context, endpoint int, o ExactRootCodecOptions) (ExactRootRecipe, error) {
	if c == nil || endpoint < 0 || endpoint > 1 || c.endpoints[endpoint] == nil {
		return ExactRootRecipe{}, ErrInvalid
	}
	return exportExactRoot(ctx, c.endpoints[endpoint], o)
}
func exportExactRoot(ctx context.Context, source *ExactGraphLineRoot, o ExactRootCodecOptions) (r ExactRootRecipe, err error) {
	if ctx == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := exactRootCodecOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	root, e := canonicalExactRoot(ctx, source.polynomial, source.bounds, o, b)
	if e != nil {
		return r, e
	}
	r.Version = 1
	r.Lower = root.bounds[0].RatString()
	r.Upper = root.bounds[1].RatString()
	total := len(r.Lower) + len(r.Upper)
	if total > o.MaxTextBytes {
		return ExactRootRecipe{}, ErrConvergence
	}
	for _, x := range root.polynomial {
		s := x.RatString()
		if len(s) > o.MaxTextBytes-total {
			return ExactRootRecipe{}, ErrConvergence
		}
		total += len(s)
		r.Coefficients = append(r.Coefficients, s)
	}
	return r, nil
}
func canonicalExactRoot(ctx context.Context, polynomial rationalPolynomial, bounds [2]*big.Rat, o ExactRootCodecOptions, b *polynomialBudget) (*ExactGraphLineRoot, error) {
	if len(polynomial)-1 > o.MaxDegree || bounds[0] == nil || bounds[1] == nil {
		return nil, ErrConvergence
	}
	p := make(rationalPolynomial, len(polynomial))
	for i, x := range polynomial {
		if x == nil {
			return nil, ErrInvalid
		}
		if e := b.check(x); e != nil {
			return nil, e
		}
		p[i] = new(big.Rat).Set(x)
	}
	p = trimPolynomial(p)
	if len(p) < 2 || bounds[0].Cmp(bounds[1]) > 0 {
		return nil, ErrInvalid
	}
	for _, x := range bounds {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	count := 0
	steps, e := isolatePolynomialRoots(p, bounds[0], bounds[1], b, o.MaxSubdivisions, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
	if e != nil {
		return nil, e
	}
	if count != 1 {
		return nil, ErrConvergence
	}
	gcd, e := polynomialGCD(p, polynomialDerivative(p), b)
	if e != nil {
		return nil, e
	}
	p, _, e = polynomialDivide(p, gcd, b)
	if e != nil {
		return nil, e
	}
	owned := [2]*big.Rat{new(big.Rat).Set(bounds[0]), new(big.Rat).Set(bounds[1])}
	rational, e := recoverRationalPolynomialRoot(p, owned[0], owned[1], b)
	if e != nil {
		return nil, e
	}
	if rational != nil {
		p = rationalPolynomial{new(big.Rat).Neg(rational), big.NewRat(1, 1)}
		owned = [2]*big.Rat{new(big.Rat).Set(rational), new(big.Rat).Set(rational)}
	}
	lcm := big.NewInt(1)
	for _, x := range p {
		g := new(big.Int).GCD(nil, nil, lcm, x.Denom())
		lcm.Mul(new(big.Int).Quo(lcm, g), x.Denom())
		if lcm.BitLen() > b.maxBits {
			return nil, ErrConvergence
		}
	}
	integers := make([]*big.Int, len(p))
	content := new(big.Int)
	for i, x := range p {
		integers[i] = new(big.Int).Mul(x.Num(), new(big.Int).Quo(new(big.Int).Set(lcm), x.Denom()))
		if integers[i].BitLen() > b.maxBits {
			return nil, ErrConvergence
		}
		content.GCD(nil, nil, content, integers[i])
	}
	if content.Sign() == 0 {
		return nil, ErrInvalid
	}
	if integers[len(integers)-1].Sign() < 0 {
		content.Neg(content)
	}
	for i, x := range integers {
		p[i] = new(big.Rat).SetInt(new(big.Int).Quo(x, content))
		if e := b.check(p[i]); e != nil {
			return nil, e
		}
	}
	return &ExactGraphLineRoot{polynomial: p, bounds: owned, operations: b.operations, subdivisions: steps}, nil
}
