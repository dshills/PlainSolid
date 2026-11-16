package geom

import (
	"context"
	"math/big"
)

const maxPeriodicDerivativeOrder = 16

// RationalPeriodicCurveDerivatives retains an exact right-hand jet on the
// original physical knot span. Derivatives[0] is position; higher entries
// use physical parameter units. Rationals and the domain are caller owned.
type RationalPeriodicCurveDerivatives struct {
	Derivatives     [][3]*big.Rat
	Domain          ExactParameterInterval
	Cycle           *big.Int
	Span            int
	ExactOperations int
}

// EvaluateDerivativesRational evaluates the uniform periodic curve through
// the requested order, from 0 to 16. Knots and seams use the right-hand
// piece. A rational derivative can be nonzero above the control degree.
func (c *PeriodicBSpline) EvaluateDerivativesRational(ctx context.Context, t *big.Rat, order int, o SurfacePatchOptions) (result RationalPeriodicCurveDerivatives, err error) {
	if c == nil || c.curve == nil || t == nil {
		return result, ErrInvalid
	}
	b, err := periodicDerivativeBudget(ctx, order, c.degree+1, o)
	if err != nil {
		return result, err
	}
	defer func() { result.ExactOperations = b.operations }()
	if err := b.check(t); err != nil {
		return result, err
	}
	owner, local, err := periodicSpanAtPosition(len(c.poles), c.start, c.period, t, b)
	if err != nil {
		return result, err
	}
	for _, q := range []*big.Rat{owner.Domain.Lower, owner.Domain.Upper, local} {
		if err := b.check(q); err != nil {
			return result, err
		}
	}
	width := new(big.Rat).Sub(owner.Domain.Upper, owner.Domain.Lower)
	controls := make([]exactHomogeneous, c.degree+1)
	for i := range controls {
		p := c.curve.poles[owner.Span+i]
		w := new(big.Rat).SetFloat64(c.curve.weights[owner.Span+i])
		controls[i][3] = w
		for axis := range 3 {
			controls[i][axis] = new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(p, axis)), w)
		}
	}
	blend := func(j, r int) (*big.Rat, *big.Rat) {
		s := big.NewRat(1, int64(c.degree-r+1))
		a := new(big.Rat).Add(local, big.NewRat(int64(c.degree-j), 1))
		a.Mul(a, s)
		return a, s.Quo(s, width)
	}
	return evaluatePeriodicDerivatives(ctx, controls, blend, order, owner, b)
}

// EvaluateDerivativesRational also accepts an edited exact knot basis.
// Repeated knots retain the physical right-hand span and its derivatives.
func (c *ExactPeriodicSplineCurve) EvaluateDerivativesRational(ctx context.Context, t *big.Rat, order int, o SurfacePatchOptions) (result RationalPeriodicCurveDerivatives, err error) {
	if c == nil || len(c.net) == 0 || t == nil {
		return result, ErrInvalid
	}
	b, err := periodicDerivativeBudget(ctx, order, c.degree+1, o)
	if err != nil {
		return result, err
	}
	defer func() { result.ExactOperations = b.operations }()
	owner, local, err := c.owner(t, b)
	if err != nil {
		return result, err
	}
	controls := make([]exactHomogeneous, c.degree+1)
	for i := range controls {
		controls[i] = c.net[(owner.Span+i)%len(c.net)]
	}
	return evaluatePeriodicDerivatives(ctx, controls, rationalSplineBlend(c.degree, owner.Span, local, c.knot), order, owner, b)
}

func periodicDerivativeBudget(ctx context.Context, order, poles int, o SurfacePatchOptions) (*polynomialBudget, error) {
	if ctx == nil || order < 0 || order > maxPeriodicDerivativeOrder {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := patchOptions(o)
	if err != nil {
		return nil, err
	}
	if poles > o.MaxPoles || poles > o.MaxOperations/4 {
		return nil, ErrConvergence
	}
	return &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}, nil
}

func evaluatePeriodicDerivatives(ctx context.Context, controls []exactHomogeneous, blend func(j, r int) (*big.Rat, *big.Rat), order int,
	owner ExactSurfaceKnotSpan, b *polynomialBudget) (RationalPeriodicCurveDerivatives, error) {
	var zero RationalPeriodicCurveDerivatives
	h, err := homogeneousSplineDerivatives(ctx, controls, blend, order, b)
	if err != nil {
		return zero, err
	}
	return dehomogenizePeriodicCurve(ctx, h, order, owner, b)
}

// The homogeneous derivative nets used for mixed tensor derivatives may
// contain zero or negative weight components.
func homogeneousSplineDerivatives(ctx context.Context, controls []exactHomogeneous, blend func(j, r int) (*big.Rat, *big.Rat),
	order int, b *polynomialBudget) ([]exactHomogeneous, error) {
	degree := len(controls) - 1
	homOrder := min(degree, order)
	jet := make([][]exactHomogeneous, len(controls))
	zeroScalar := new(big.Rat) // never used as an arithmetic receiver
	for i, c := range controls {
		jet[i] = make([]exactHomogeneous, homOrder+1)
		jet[i][0] = c
		if err := checkRationalSplinePole(c, b); err != nil {
			return nil, err
		}
		for k := 1; k <= homOrder; k++ {
			jet[i][k] = exactHomogeneous{zeroScalar, zeroScalar, zeroScalar, zeroScalar}
		}
	}
	one := big.NewRat(1, 1)
	for r := 1; r <= degree; r++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := degree; j >= r; j-- {
			a, slope := blend(j, r)
			q := new(big.Rat).Sub(one, a)
			for _, x := range []*big.Rat{a, q, slope} {
				if err := b.check(x); err != nil {
					return nil, err
				}
			}
			left, right := jet[j-1], jet[j]
			// Both loops run backward so the lower derivatives have not been
			// overwritten when differentiating the affine blend.
			for k := min(r, homOrder); k >= 0; k-- {
				for axis := range 4 {
					v := new(big.Rat).Add(new(big.Rat).Mul(q, left[k][axis]), new(big.Rat).Mul(a, right[k][axis]))
					if k > 0 {
						d := new(big.Rat).Sub(right[k-1][axis], left[k-1][axis])
						v.Add(v, d.Mul(d, slope).Mul(d, big.NewRat(int64(k), 1)))
					}
					if err := b.check(v); err != nil {
						return nil, err
					}
					right[k][axis] = v
				}
			}
		}
	}
	return jet[degree], nil
}

func dehomogenizePeriodicCurve(ctx context.Context, h []exactHomogeneous, order int, owner ExactSurfaceKnotSpan,
	b *polynomialBudget) (RationalPeriodicCurveDerivatives, error) {
	var zero RationalPeriodicCurveDerivatives
	homOrder := len(h) - 1
	if h[0][3].Sign() <= 0 {
		return zero, ErrInvalid
	}
	d := make([][3]*big.Rat, order+1)
	var coefficients [maxPeriodicDerivativeOrder + 1][maxPeriodicDerivativeOrder + 1]int64
	for k := range d {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		coefficients[k][0], coefficients[k][k] = 1, 1
		for j := 1; j < k; j++ {
			coefficients[k][j] = coefficients[k-1][j-1] + coefficients[k-1][j]
		}
		for axis := range 3 {
			v := new(big.Rat)
			if k <= homOrder {
				v.Set(h[k][axis])
			}
			// A = w C. Subtract the already-known quotient derivatives
			// from the kth product derivative, then divide by w itself.
			for j := 1; j <= min(k, homOrder); j++ {
				term := new(big.Rat).Mul(h[j][3], d[k-j][axis])
				term.Mul(term, big.NewRat(coefficients[k][j], 1))
				if err := b.check(term); err != nil {
					return zero, err
				}
				v.Sub(v, term)
				if err := b.check(v); err != nil {
					return zero, err
				}
			}
			d[k][axis] = v.Quo(v, h[0][3])
			if err := b.check(d[k][axis]); err != nil {
				return zero, err
			}
		}
	}
	return RationalPeriodicCurveDerivatives{Derivatives: d, Domain: owner.Domain, Cycle: owner.Cycle, Span: owner.Span}, nil
}
