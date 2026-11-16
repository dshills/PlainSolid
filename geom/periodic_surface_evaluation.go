package geom

import (
	"context"
	"math/big"
	"sort"
)

// RationalPeriodicSurfaceJet returns exact coordinates and physical partials
// together with the original one-sided knot owners. All values are owned by
// the result. Each bound encloses its corresponding exact vector.
type RationalPeriodicSurfaceJet struct {
	RationalSurfacePatchJet
	U, V ExactSurfaceKnotSpan
}

// EvaluateRational evaluates the original tensor at exact physical UV. Knot
// and cycle selection use the exact represented periods and knot vectors.
// Interior knots and periodic seams select the right piece. The upper end
// of an ordinary axis selects its last nonempty piece.
func (s *PeriodicBSplineSurface) EvaluateRational(ctx context.Context, u, v *big.Rat, options SurfacePatchOptions) (result RationalPeriodicSurfaceJet, err error) {
	if ctx == nil || s == nil || s.surface == nil || u == nil || v == nil {
		return result, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	o, err := patchOptions(options)
	if err != nil {
		return result, err
	}
	degreeU, degreeV := s.surface.degreeU, s.surface.degreeV
	if (degreeU+1)*(degreeV+1) > o.MaxPoles {
		return result, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	uWindow, un, err := s.tensorAxisAtParameter(ctx, 0, u, b)
	if err != nil {
		return result, err
	}
	vWindow, vn, err := s.tensorAxisAtParameter(ctx, 1, v, b)
	if err != nil {
		return result, err
	}
	controls, err := s.exactTensorWindowControls(ctx, uWindow, vWindow, b)
	if err != nil {
		return result, err
	}
	o.MaxOperations -= b.operations
	if o.MaxOperations < 1 {
		return result, ErrConvergence
	}
	patch, err := NewExactHomogeneousBezierSurface(ctx, degreeU, degreeV, controls, o)
	if err != nil {
		return result, err
	}
	b.operations += patch.ExactOperations()
	point, d, err := exactSurfaceJet(patch, un, vn, b)
	if err != nil {
		return result, err
	}
	widthU := new(big.Rat).Sub(uWindow.owner.Domain.Upper, uWindow.owner.Domain.Lower)
	widthV := new(big.Rat).Sub(vWindow.owner.Domain.Upper, vWindow.owner.Domain.Lower)
	var jet RationalSurfacePatchJet
	for axis := range point {
		jet.Point[axis] = point[axis]
		jet.Du[axis] = new(big.Rat).Quo(d[axis][0], widthU)
		jet.Dv[axis] = new(big.Rat).Quo(d[axis][1], widthV)
		for _, value := range []*big.Rat{jet.Du[axis], jet.Dv[axis]} {
			if err := b.check(value); err != nil {
				return result, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	jet.PointBounds = rationalSurfaceVectorBounds(jet.Point)
	jet.DuBounds = rationalSurfaceVectorBounds(jet.Du)
	jet.DvBounds = rationalSurfaceVectorBounds(jet.Dv)
	result.RationalSurfacePatchJet = jet
	result.U, result.V = uWindow.owner, vWindow.owner
	return result, nil
}

func (s *PeriodicBSplineSurface) tensorAxisAtParameter(ctx context.Context, direction int, position *big.Rat, b *polynomialBudget) (window periodicTensorWindow, normalized *big.Rat, err error) {
	if err := b.check(position); err != nil {
		return window, nil, err
	}
	degree, count, knots := s.tensorSplineAxis(direction)
	axis := s.axis[direction]
	var basisLower, basisUpper *big.Rat
	if axis.Period != 0 {
		window.owner, normalized, err = periodicSpanAtPosition(s.count[direction], axis.Start, axis.Period, position, b)
		if err != nil {
			return window, nil, err
		}
		basisLower = big.NewRat(int64(window.owner.Span), 1)
		basisUpper = new(big.Rat).Add(basisLower, big.NewRat(1, 1))
	} else {
		knot := func(i int) *big.Rat { return new(big.Rat).SetFloat64(knots[i]) }
		end := position.Cmp(knot(count))
		if position.Cmp(knot(degree)) < 0 || end > 0 {
			return window, nil, ErrDomain
		}
		span := sort.Search(count-degree, func(i int) bool {
			c := knot(i + degree + 1).Cmp(position)
			return c > 0 || c == 0 && end == 0
		})
		basisLower, basisUpper = knot(span+degree), knot(span+degree+1)
		window.owner = ExactSurfaceKnotSpan{Domain: ExactParameterInterval{Lower: basisLower, Upper: basisUpper}, Span: span}
		normalized = new(big.Rat).Quo(new(big.Rat).Sub(position, basisLower), new(big.Rat).Sub(basisUpper, basisLower))
	}
	for _, value := range []*big.Rat{window.owner.Domain.Lower, window.owner.Domain.Upper, normalized} {
		if err := b.check(value); err != nil {
			return window, nil, err
		}
	}
	window.weights, err = exactAxisBernsteinWeights(ctx, knots, degree, window.owner.Span+degree, basisLower, basisUpper, b)
	return window, normalized, err
}
