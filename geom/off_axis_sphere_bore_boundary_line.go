package geom

import (
	"context"
	"math/big"
)

// BoreBoundaryInterval is a closed original-parameter interval of continuous
// boundary contact. Boundary identifies the support along its relative interior;
// endpoint masks can additionally include rim/floor supports. Immutable roots
// and caller-owned bounds are independent of subsequent query calls.
type BoreBoundaryInterval struct {
	Start, End                           *ExactAlgebraicRoot
	StartBounds, EndBounds               ExactParameterInterval
	Boundary, StartBoundary, EndBoundary BoreBoundaryMask
}
type BoreLineBoundary struct {
	Hits      []BoreLineHit
	Intervals []BoreBoundaryInterval
}

// IntersectLineBoundary extends isolated-line queries with floor/cylinder
// coincidence. It partitions the finite range at every exact support root,
// proves signs on rational interior witnesses, and merges adjacent intervals
// of the same boundary support. Overlap endpoints are not duplicated in Hits.
// The input tolerance bounds isolated root brackets, not material classification.
// Arithmetic/root-isolation work is bounded; individual algebraic comparisons
// and refinements each receive the supplied options (at most seven endpoints).
func (s OffAxisSphereBore) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	fail := func(e error) (BoreLineBoundary, error) { return BoreLineBoundary{}, e }
	if ctx == nil || s.R == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return fail(ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return fail(ErrInvalid)
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return fail(e)
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var polynomials [3]rationalPolynomial
	for i := 0; i < 2; i++ {
		polynomials[i] = rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
	}
	values, derivatives := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{direction.X, direction.Y, direction.Z}
	for axis := 0; axis < 3; axis++ {
		linear := rationalPolynomial{q(values[axis]), q(derivatives[axis])}
		square, e := polynomialProduct(linear, linear, budget)
		if e != nil {
			return fail(e)
		}
		for j, v := range square {
			polynomials[0][j].Add(polynomials[0][j], v)
		}
	}
	polynomials[0][0].Sub(polynomials[0][0], new(big.Rat).Mul(s.R, s.R))
	for axis, center := range []*big.Rat{s.cx, s.cy} {
		linear := rationalPolynomial{new(big.Rat).Sub(q(values[axis]), center), q(derivatives[axis])}
		square, e := polynomialProduct(linear, linear, budget)
		if e != nil {
			return fail(e)
		}
		for j, v := range square {
			polynomials[1][j].Add(polynomials[1][j], v)
		}
	}
	polynomials[1][0].Sub(polynomials[1][0], new(big.Rat).Mul(s.r, s.r))
	polynomials[2] = rationalPolynomial{big.NewRat(1, 1)}
	if s.floor != nil {
		polynomials[2] = rationalPolynomial{new(big.Rat).Sub(q(origin.Z), s.floor), q(direction.Z)}
	}
	for i := range polynomials {
		polynomials[i] = trimPolynomial(polynomials[i])
		for _, x := range polynomials[i] {
			if e := budget.check(x); e != nil {
				return fail(e)
			}
		}
	}
	if len(polynomials[1]) > 0 && len(polynomials[2]) > 0 {
		hits, e := s.IntersectLine(ctx, origin, direction, minT, maxT, o)
		if e != nil {
			return fail(e)
		}
		return BoreLineBoundary{Hits: hits}, nil
	}
	return intersectMaterialBoundary(ctx, polynomials[:], minT, maxT, o, budget, ro, func(signs []int) (BorePointLocation, BoreBoundaryMask) {
		return boreLocation(signs[0], signs[1], signs[2])
	})
}
