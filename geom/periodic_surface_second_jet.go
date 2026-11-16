package geom

import (
	"context"
	"math/big"
)

// RationalPeriodicSurfaceSecondJet extends the first jet with the three
// physical second partials. Bounds enclose the exact values, which belong
// to the caller. All derivatives use the same original one-sided knot owners.
type RationalPeriodicSurfaceSecondJet struct {
	RationalPeriodicSurfaceJet
	Duu, Duv, Dvv                   [3]*big.Rat
	DuuBounds, DuvBounds, DvvBounds Box
}

// EvaluateSecondJetRational evaluates an exact second jet on the physical UV
// axes. At a periodic seam or interior knot it uses the right-hand piece;
// at the upper end of an ordinary axis it uses the last nonempty piece.
func (s *PeriodicBSplineSurface) EvaluateSecondJetRational(ctx context.Context, u, v *big.Rat,
	o SurfacePatchOptions) (result RationalPeriodicSurfaceSecondJet, err error) {
	if s == nil || s.surface == nil || u == nil || v == nil {
		return result, ErrInvalid
	}
	nu, nv := s.surface.degreeU+1, s.surface.degreeV+1
	b, err := periodicDerivativeBudget(ctx, 2, nu*nv, o)
	if err != nil {
		return result, err
	}
	defer func() { result.ExactOperations = b.operations }()
	uw, un, err := s.tensorAxisAtParameter(ctx, 0, u, b)
	if err != nil {
		return result, err
	}
	vw, vn, err := s.tensorAxisAtParameter(ctx, 1, v, b)
	if err != nil {
		return result, err
	}
	controls, err := s.exactTensorWindowControls(ctx, uw, vw, b)
	if err != nil {
		return result, err
	}
	bezierBlend := func(t *big.Rat, owner ExactSurfaceKnotSpan) splineJetBlend {
		slope := new(big.Rat).Inv(new(big.Rat).Sub(owner.Domain.Upper, owner.Domain.Lower))
		return func(_, _ int) (*big.Rat, *big.Rat) { return t, slope }
	}
	return evaluatePeriodicSecondJet(ctx, nu, nv, func(i, j int) exactHomogeneous { return controls[i*nv+j] },
		uw.owner, vw.owner, bezierBlend(un, uw.owner), bezierBlend(vn, vw.owner), b)
}

// EvaluateSecondJetRational also supports exact nonuniform periodic knots.
// Repeated knots select the same physical pieces as EvaluateRational.
func (s *ExactPeriodicSplineSurface) EvaluateSecondJetRational(ctx context.Context, u, v *big.Rat,
	o SurfacePatchOptions) (result RationalPeriodicSurfaceSecondJet, err error) {
	if s == nil || len(s.net) == 0 || u == nil || v == nil {
		return result, ErrInvalid
	}
	a, c := s.axis[0], s.axis[1]
	nu, nv := a.degree+1, c.degree+1
	b, err := periodicDerivativeBudget(ctx, 2, nu*nv, o)
	if err != nil {
		return result, err
	}
	defer func() { result.ExactOperations = b.operations }()
	uo, ub, err := a.window(u, b)
	if err != nil {
		return result, err
	}
	vo, vb, err := c.window(v, b)
	if err != nil {
		return result, err
	}
	return evaluatePeriodicSecondJet(ctx, nu, nv, func(i, j int) exactHomogeneous {
		return s.net[((uo.Span+i)%a.count)*c.count+(vo.Span+j)%c.count]
	}, uo, vo, ub, vb, b)
}

type splineJetBlend func(j, r int) (*big.Rat, *big.Rat)

func evaluatePeriodicSecondJet(ctx context.Context, nu, nv int, pole func(i, j int) exactHomogeneous,
	uo, vo ExactSurfaceKnotSpan, ub, vb splineJetBlend, b *polynomialBudget) (RationalPeriodicSurfaceSecondJet, error) {
	var result RationalPeriodicSurfaceSecondJet
	d, err := exactSplineSecondTensor(ctx, nu, nv, pole, ub, vb, b)
	if err != nil {
		return result, err
	}
	result.U, result.V = uo, vo
	result.Point, result.Du, result.Dv = d[0][0], d[1][0], d[0][1]
	result.Duu, result.Duv, result.Dvv = d[2][0], d[1][1], d[0][2]
	result.PointBounds, result.DuBounds, result.DvBounds = rationalSurfaceVectorBounds(result.Point), rationalSurfaceVectorBounds(result.Du), rationalSurfaceVectorBounds(result.Dv)
	result.DuuBounds, result.DuvBounds, result.DvvBounds = rationalSurfaceVectorBounds(result.Duu), rationalSurfaceVectorBounds(result.Duv), rationalSurfaceVectorBounds(result.Dvv)
	return result, nil
}

func exactSplineSecondTensor(ctx context.Context, nu, nv int, pole func(i, j int) exactHomogeneous,
	ub, vb splineJetBlend, b *polynomialBudget) (d [3][3][3]*big.Rat, err error) {
	row := make([]exactHomogeneous, nv)
	var unets [3][]exactHomogeneous
	vOrder := min(nv-1, 2)
	for k := 0; k <= vOrder; k++ {
		unets[k] = make([]exactHomogeneous, nu)
	}
	for i := range nu {
		for j := range row {
			row[j] = pole(i, j)
		}
		jet, err := homogeneousSplineDerivatives(ctx, row, vb, 2, b)
		if err != nil {
			return d, err
		}
		for k, p := range jet {
			unets[k][i] = p
		}
	}
	var h [3][3]exactHomogeneous
	for k := 0; k <= vOrder; k++ {
		jet, err := homogeneousSplineDerivatives(ctx, unets[k], ub, 2-k, b)
		if err != nil {
			return d, err
		}
		for j, p := range jet {
			h[j][k] = p
		}
	}
	return dehomogenizeSecondJet(ctx, h, b)
}

func dehomogenizeSecondJet(ctx context.Context, h [3][3]exactHomogeneous, b *polynomialBudget) (d [3][3][3]*big.Rat, err error) {
	w := h[0][0][3]
	if w.Sign() <= 0 {
		return d, ErrInvalid
	}
	two := big.NewRat(2, 1)
	for order := 0; order <= 2; order++ {
		if err := ctx.Err(); err != nil {
			return d, err
		}
		for i := 0; i <= order; i++ {
			j := order - i
			for axis := range 3 {
				value := new(big.Rat)
				if h[i][j][axis] != nil {
					value.Set(h[i][j][axis])
				}
				// Apply the bivariate product rule to Hxyz = w * S.
				for a := 0; a <= i; a++ {
					for c := 0; c <= j; c++ {
						factor := h[a][c][3]
						if a+c == 0 || factor == nil || factor.Sign() == 0 {
							continue
						}
						term := new(big.Rat).Mul(factor, d[i-a][j-c][axis])
						if i == 2 && a == 1 || j == 2 && c == 1 {
							term.Mul(term, two)
						}
						if err := b.check(term); err != nil {
							return d, err
						}
						if err := b.check(value.Sub(value, term)); err != nil {
							return d, err
						}
					}
				}
				d[i][j][axis] = value.Quo(value, w)
				if err := b.check(d[i][j][axis]); err != nil {
					return d, err
				}
			}
		}
	}
	return d, nil
}
