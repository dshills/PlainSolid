package geom

import (
	"context"
	"math/big"
	"sort"
)

// ToroidLineOptions bounds exact quartic root isolation. ParameterTolerance is
// required and bounds each returned original-t interval. MaxSubdivisions
// defaults to1024 (maximum65536). Surface limits cover proof and polynomial work.
type ToroidLineOptions struct {
	ParameterTolerance float64
	MaxSubdivisions    int
	Surface            SurfacePatchOptions
}

// ToroidLineRoot encloses one distinct intersection parameter, with exact
// algebraic multiplicity. Equal endpoints identify a rational root exactly.
type ToroidLineRoot struct {
	Min, Max     *big.Rat
	Multiplicity int
}

// IntersectWrappedCircularToroidLine intersects P(t)=origin+t*direction over
// the closed represented finite interval [minT,maxT]. Direction need not be
// unit length. The circular profile identities and canonical embedding are
// proven first. Exact quartic Sturm isolation retains tangencies and endpoint
// roots, returning no partial result on failure. Rational brackets are sorted
// in the original parameter, without normalization or approximate geometry.
func IntersectWrappedCircularToroidLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]ToroidLineRoot, error) {
	if !origin.IsFinite() || !direction.IsFinite() || !finite(minT) || !finite(maxT) {
		return nil, ErrInvalid
	}
	return IntersectWrappedCircularToroidLineRational(ctx, w, rationalLineVec3(origin), rationalLineVec3(direction), new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT), o)
}

// IntersectWrappedCircularToroidLineRational preserves nonbinary model lines
// and finite parameter bounds. Distinct roots have strictly separated closed
// brackets; exact multiple roots retain their multiplicity.
func IntersectWrappedCircularToroidLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) ([]ToroidLineRoot, error) {
	roots, _, e := intersectWrappedCircularToroidLine(ctx, w, origin, direction, minT, maxT, o, false)
	return roots, e
}

// ToroidLineMaterial returns exact boundary brackets and the finite closed
// material spans that they delimit. A repeated outside touch is a singleton
// span. Artificial finite endpoints are not counted as boundary hits.
type ToroidLineMaterial struct {
	Boundary []ToroidLineRoot
	Spans    [][2]ToroidLineRoot
}

// ClipWrappedCircularToroidLineRational classifies a complete set of finite
// rational line cells using the same independently proved original quartic.
func ClipWrappedCircularToroidLineRational(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (ToroidLineMaterial, error) {
	roots, spans, e := intersectWrappedCircularToroidLine(ctx, w, origin, direction, minT, maxT, o, true)
	if e != nil {
		return ToroidLineMaterial{}, e
	}
	return ToroidLineMaterial{Boundary: roots, Spans: spans}, nil
}

func intersectWrappedCircularToroidLine(ctx context.Context, w *WrappedSurfacePatch, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions, clip bool) ([]ToroidLineRoot, [][2]ToroidLineRoot, error) {
	options, e := validateRationalCircularLine(ctx, origin[:], direction[:], minT, maxT, o)
	if e != nil {
		return nil, nil, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return nil, nil, ErrInvalid
	}
	proof, e := CertifyWrappedToroidEmbedding(ctx, w, o.Surface)
	if e != nil {
		return nil, nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: proof.ExactOperations}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, b, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			l, r := new(big.Rat).Mul(a, c), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(b, b))
			if e := budget.check(l); e != nil {
				return nil, nil, e
			}
			if e := budget.check(r); e != nil {
				return nil, nil, e
			}
			if l.Cmp(r) != 0 {
				return nil, nil, ErrConvergence
			}
		}
	}
	var squares [3]rationalPolynomial
	for i := 0; i < 3; i++ {
		linear := rationalPolynomial{new(big.Rat).Sub(origin[i], proof.Center[i]), direction[i]}
		squares[i], e = polynomialProduct(linear, linear, budget)
		if e != nil {
			return nil, nil, e
		}
	}
	coefficient := func(p rationalPolynomial, i int) *big.Rat {
		if i >= len(p) {
			return new(big.Rat)
		}
		return p[i]
	}
	q := make(rationalPolynomial, 3)
	a := make(rationalPolynomial, 3)
	R2 := new(big.Rat).Mul(proof.MajorRadius, proof.MajorRadius)
	r2 := new(big.Rat).Mul(proof.RadialMinorRadius, proof.RadialMinorRadius)
	h2 := new(big.Rat).Mul(proof.AxialMinorRadius, proof.AxialMinorRadius)
	scale := new(big.Rat).Quo(r2, h2)
	for i := 0; i < 3; i++ {
		q[i] = new(big.Rat).Add(coefficient(squares[0], i), coefficient(squares[1], i))
		a[i] = new(big.Rat).Add(q[i], new(big.Rat).Mul(scale, coefficient(squares[2], i)))
	}
	a[0].Add(a[0], new(big.Rat).Sub(R2, r2))
	polynomial, e := polynomialProduct(a, a, budget)
	if e != nil {
		return nil, nil, e
	}
	factor := new(big.Rat).Mul(big.NewRat(4, 1), R2)
	for i := range q {
		for len(polynomial) <= i {
			polynomial = append(polynomial, new(big.Rat))
		}
		polynomial[i].Sub(polynomial[i], new(big.Rat).Mul(factor, q[i]))
		if e := budget.check(polynomial[i]); e != nil {
			return nil, nil, e
		}
	}
	// A=q+(r²/h²)z²+R²−r² is strictly positive because R>r.
	// Thus squaring A=2R sqrt(q) introduces no extraneous real roots.
	tolerance := new(big.Rat).SetFloat64(o.ParameterTolerance)
	roots := []ToroidLineRoot{}
	poly := trimPolynomial(polynomial)
	_, e = isolatePolynomialRoots(poly, minT, maxT, budget, o.MaxSubdivisions, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if new(big.Rat).Sub(hi, lo).Cmp(tolerance) > 0 {
			return false, nil
		}
		if lo.Cmp(hi) != 0 {
			// A distinct interval must exclude any root already emitted
			// at either endpoint, including removed finite endpoint roots.
			for _, t := range []*big.Rat{lo, hi} {
				v, e := polynomialValue(poly, t, budget)
				if e != nil {
					return false, e
				}
				if v.Sign() == 0 {
					return false, nil
				}
			}
			if clip && (lo.Cmp(minT) == 0 || hi.Cmp(maxT) == 0) {
				return false, nil
			}

			if len(roots) > 0 && roots[len(roots)-1].Max.Cmp(lo) == 0 {
				return false, nil
			}
		}
		roots = append(roots, ToroidLineRoot{new(big.Rat).Set(lo), new(big.Rat).Set(hi), multiplicity})
		return true, nil
	})
	if e != nil {
		return nil, nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Min.Cmp(roots[j].Min) < 0 })
	if !clip {
		return roots, nil, nil
	}
	var spans [][2]ToroidLineRoot
	at := func(t *big.Rat) ToroidLineRoot {
		return ToroidLineRoot{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t)}
	}
	cells := make([]ToroidLineRoot, 0, len(roots)+2)
	if len(roots) == 0 || roots[0].Min.Cmp(minT) != 0 {
		cells = append(cells, at(minT))
	}
	cells = append(cells, roots...)
	if cells[len(cells)-1].Max.Cmp(maxT) != 0 {
		cells = append(cells, at(maxT))
	}
	inside := make([]bool, len(cells)+1)
	for i := 1; i < len(cells); i++ {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		a, z := cells[i-1].Max, cells[i].Min
		if a.Cmp(z) >= 0 {
			return nil, nil, ErrConvergence
		}
		v, e := polynomialValue(poly, new(big.Rat).Quo(new(big.Rat).Add(a, z), big.NewRat(2, 1)), budget)
		if e != nil {
			return nil, nil, e
		}
		if v.Sign() == 0 {
			return nil, nil, ErrConvergence
		}
		inside[i] = v.Sign() < 0
	}
	for i, x := range cells {
		if inside[i] {
			if len(spans) == 0 {
				return nil, nil, ErrConvergence
			}
			spans[len(spans)-1][1] = x
			continue
		}
		if x.Multiplicity > 0 || inside[i+1] {
			spans = append(spans, [2]ToroidLineRoot{x, x})
		}
	}
	if len(cells) == 1 && minT.Cmp(maxT) == 0 && cells[0].Multiplicity == 0 {
		v, e := polynomialValue(poly, minT, budget)
		if e != nil {
			return nil, nil, e
		}
		if v.Sign() < 0 {
			spans = append(spans, [2]ToroidLineRoot{cells[0], cells[0]})
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	return roots, spans, nil
}
