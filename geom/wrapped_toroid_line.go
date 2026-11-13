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
	if !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return nil, ErrInvalid
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return nil, ErrInvalid
	}
	proof, e := CertifyWrappedToroidEmbedding(ctx, w, o.Surface)
	if e != nil {
		return nil, e
	}
	options, e := patchOptions(o.Surface)
	if e != nil {
		return nil, e
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits, operations: proof.ExactOperations}
	for _, stride := range []int{1, 9} {
		for j := 0; j < 8; j += 2 {
			a, b, c := w.patch.net[j*stride][3], w.patch.net[(j+1)*stride][3], w.patch.net[(j+2)*stride][3]
			l, r := new(big.Rat).Mul(a, c), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(b, b))
			if e := budget.check(l); e != nil {
				return nil, e
			}
			if e := budget.check(r); e != nil {
				return nil, e
			}
			if l.Cmp(r) != 0 {
				return nil, ErrConvergence
			}
		}
	}
	var squares [3]rationalPolynomial
	ov, dv := []float64{origin.X, origin.Y, origin.Z}, []float64{direction.X, direction.Y, direction.Z}
	for i := 0; i < 3; i++ {
		linear := rationalPolynomial{new(big.Rat).Sub(new(big.Rat).SetFloat64(ov[i]), proof.Center[i]), new(big.Rat).SetFloat64(dv[i])}
		squares[i], e = polynomialProduct(linear, linear, budget)
		if e != nil {
			return nil, e
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
		return nil, e
	}
	factor := new(big.Rat).Mul(big.NewRat(4, 1), R2)
	for i := range q {
		for len(polynomial) <= i {
			polynomial = append(polynomial, new(big.Rat))
		}
		polynomial[i].Sub(polynomial[i], new(big.Rat).Mul(factor, q[i]))
		if e := budget.check(polynomial[i]); e != nil {
			return nil, e
		}
	}
	// A=q+(r²/h²)z²+R²−r² is strictly positive because R>r.
	// Thus squaring A=2R sqrt(q) introduces no extraneous real roots.
	tolerance := new(big.Rat).SetFloat64(o.ParameterTolerance)
	roots := []ToroidLineRoot{}
	_, e = isolatePolynomialRoots(trimPolynomial(polynomial), new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT), budget, o.MaxSubdivisions, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if new(big.Rat).Sub(hi, lo).Cmp(tolerance) > 0 {
			return false, nil
		}
		roots = append(roots, ToroidLineRoot{new(big.Rat).Set(lo), new(big.Rat).Set(hi), multiplicity})
		return true, nil
	})
	if e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Min.Cmp(roots[j].Min) < 0 })
	return roots, nil
}
