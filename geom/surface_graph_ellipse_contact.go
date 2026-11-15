package geom

import (
	"context"
	"math/big"
)

// GraphEllipseHint specifies an axis-aligned ellipse in original chart UV.
// Both squared semi-axis radii must be strictly positive. Hints are proposals;
// exact tensor identities independently validate the entire contact set.
type GraphEllipseHint struct{ Center, RadiusSquared [2]*big.Rat }

// SplineGraphEllipse certifies precisely one closed elliptical contact contour
// strictly inside Domain, with no other contacts. InsideSide and OutsideSide are
// strict signs of A.Z-B.Z. The original XY projection is globally injective, so
// the closed parameter ellipse maps to a simple model contact curve. This does
// not assert Euclidean ellipticity in model space or smoothness across knots.
type SplineGraphEllipse struct {
	Proven                  bool
	Ellipse                 GraphEllipseHint
	Domain                  ParameterRectangle
	InsideSide, OutsideSide int
	Graph                   SurfaceInjectivity
	CertifiedCells          []ParameterRectangle
	ExactOperations         int
}

// CertifySplineGraphEllipse accepts an arbitrary positive-weight common-XY
// spline pair with union-knot cells. It divides each exact numerator by the
// supplied ellipse polynomial and requires every quotient Bernstein coefficient
// to have one common strict sign. Unsupported division/sign bounds remain
// ErrConvergence; a failed hint is never evidence of absence.
func CertifySplineGraphEllipse(ctx context.Context, a, c *SurfacePatch, hint GraphEllipseHint, o GraphPointContactOptions) (r SplineGraphEllipse, err error) {
	return certifySplineGraphEllipseMultiplicity(ctx, a, c, hint, o, 1)
}

// CertifySplineGraphRepeatedEllipse proves an exact double contact along
// one closed original-chart ellipse. The residual keeps the same strict
// sign on both sides. Every union-knot cell must divide twice with a
// remaining strict sign, so mixed orders and other contacts fail closed.
func CertifySplineGraphRepeatedEllipse(ctx context.Context, a, c *SurfacePatch, hint GraphEllipseHint, o GraphPointContactOptions) (SplineGraphEllipse, error) {
	return certifySplineGraphEllipseMultiplicity(ctx, a, c, hint, o, 2)
}

func certifySplineGraphEllipseMultiplicity(ctx context.Context, a, c *SurfacePatch, hint GraphEllipseHint, o GraphPointContactOptions,
	multiplicity int) (r SplineGraphEllipse, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for axis := 0; axis < 2; axis++ {
		if hint.Center[axis] == nil || hint.RadiusSquared[axis] == nil || hint.RadiusSquared[axis].Sign() <= 0 {
			return r, ErrInvalid
		}
		for _, x := range []*big.Rat{hint.Center[axis], hint.RadiusSquared[axis]} {
			if e := b.check(x); e != nil {
				return r, e
			}
		}
		r.Ellipse.Center[axis] = new(big.Rat).Set(hint.Center[axis])
		r.Ellipse.RadiusSquared[axis] = new(big.Rat).Set(hint.RadiusSquared[axis])
	}
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	r.Domain = difference.Domain()
	bounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
	for axis := 0; axis < 2; axis++ {
		for end := 0; end < 2; end++ {
			distance := new(big.Rat).Sub(hint.Center[axis], new(big.Rat).SetFloat64(bounds[axis][end]))
			if end == 1 {
				distance.Neg(distance)
			}
			if distance.Sign() <= 0 {
				return r, ErrConvergence
			}
			square := new(big.Rat).Mul(distance, distance)
			if e := b.check(square); e != nil {
				return r, e
			}
			if square.Cmp(hint.RadiusSquared[axis]) <= 0 {
				return r, ErrConvergence
			}
		}
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	sign := 0
	for _, cell := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		d := cell.Domain()
		localBounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		// Polynomial order: constant, U, U², V, V² in normalized cell UV.
		coefficients := [5]*big.Rat{big.NewRat(-1, 1), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
		for axis := 0; axis < 2; axis++ {
			origin := new(big.Rat).Sub(new(big.Rat).SetFloat64(localBounds[axis][0]), hint.Center[axis])
			width := new(big.Rat).Sub(new(big.Rat).SetFloat64(localBounds[axis][1]), new(big.Rat).SetFloat64(localBounds[axis][0]))
			denominator := hint.RadiusSquared[axis]
			coefficients[0].Add(coefficients[0], new(big.Rat).Quo(new(big.Rat).Mul(origin, origin), denominator))
			coefficients[1+2*axis].Quo(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(origin, width)), denominator)
			coefficients[2+2*axis].Quo(new(big.Rat).Mul(width, width), denominator)
		}
		q := cell
		for i := 0; i < multiplicity; i++ {
			q, e = divideGraphAxisQuadraticBernstein(ctx, q, coefficients, b)
			if e != nil {
				return r, e
			}
			if q == nil {
				return r, ErrConvergence
			}
		}
		for _, h := range q.net {
			current := h[2].Sign()
			if current == 0 || (sign != 0 && current != sign) {
				return r, ErrConvergence
			}
			sign = current
		}
		r.CertifiedCells = append(r.CertifiedCells, d)
	}
	if sign == 0 {
		return r, ErrConvergence
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	r.InsideSide = sign
	if multiplicity%2 != 0 {
		r.InsideSide = -sign
	}
	r.OutsideSide = sign
	r.Proven = true
	return r, nil
}

// divideGraphAxisQuadraticBernstein divides by c0+c1*u+c2*u²+c3*v+c4*v².
// Both quadratic leading coefficients must be nonzero. Its returned patch is a
// private scalar Z net; it deliberately has no geometric homogeneous values.
func divideGraphAxisQuadraticBernstein(ctx context.Context, p *SurfacePatch, c [5]*big.Rat, b *polynomialBudget) (*SurfacePatch, error) {
	return divideGraphQuadraticBernstein(ctx, p, [6]*big.Rat{c[0], c[1], c[2], c[3], c[4], new(big.Rat)}, b)
}
func divideGraphQuadraticBernstein(ctx context.Context, p *SurfacePatch, c [6]*big.Rat, b *polynomialBudget) (*SurfacePatch, error) {
	du, dv := p.countU-1, p.countV-1
	if du < 2 || dv < 2 || c[2].Sign() == 0 || c[4].Sign() == 0 {
		return nil, nil
	}
	for _, x := range c {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	maxV := du + dv + 3
	rem := make([][]*big.Rat, du+1)
	q := make([][]*big.Rat, du-1)
	for i := range rem {
		rem[i] = make([]*big.Rat, maxV)
		for j := range rem[i] {
			rem[i][j] = new(big.Rat)
		}
		if i < du-1 {
			q[i] = make([]*big.Rat, maxV)
			for j := range q[i] {
				q[i][j] = new(big.Rat)
			}
		}
	}
	rows := make([]rationalPolynomial, du+1)
	for i := range rows {
		values := make([]*big.Rat, dv+1)
		for j := range values {
			values[j] = p.net[i*p.countV+j][2]
		}
		var e error
		rows[i], e = bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
	}
	for j := 0; j <= dv; j++ {
		values := make([]*big.Rat, du+1)
		for i := range values {
			values[i] = new(big.Rat)
			if j < len(rows[i]) {
				values[i].Set(rows[i][j])
			}
		}
		poly, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		for i, x := range poly {
			rem[i][j].Set(x)
		}
	}
	for i := du; i >= 2; i-- {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j := 0; j < maxV; j++ {
			if rem[i][j].Sign() == 0 {
				continue
			}
			x := new(big.Rat).Quo(rem[i][j], c[2])
			if e := b.check(x); e != nil {
				return nil, e
			}
			q[i-2][j].Set(x)
			rem[i][j].SetInt64(0)
			if j+2 >= maxV {
				return nil, nil
			}
			for _, term := range []struct{ i, j, k int }{{i - 1, j, 1}, {i - 2, j, 0}, {i - 2, j + 1, 3}, {i - 2, j + 2, 4}, {i - 1, j + 1, 5}} {
				rem[term.i][term.j].Sub(rem[term.i][term.j], new(big.Rat).Mul(c[term.k], x))
				if e := b.check(rem[term.i][term.j]); e != nil {
					return nil, e
				}
			}
		}
	}
	for i := 0; i < 2; i++ {
		for _, x := range rem[i] {
			if x.Sign() != 0 {
				return nil, nil
			}
		}
	}
	for i := range q {
		for j := dv - 1; j < maxV; j++ {
			if q[i][j].Sign() != 0 {
				return nil, nil
			}
		}
	}
	result := *p
	result.degreeU -= 2
	result.degreeV -= 2
	result.countU -= 2
	result.countV -= 2
	result.knotsU = append([]float64(nil), p.knotsU[2:len(p.knotsU)-2]...)
	result.knotsV = append([]float64(nil), p.knotsV[2:len(p.knotsV)-2]...)
	result.net = make([]exactHomogeneous, result.countU*result.countV)
	for i := 0; i <= du-2; i++ {
		for j := 0; j <= dv-2; j++ {
			coefficient := new(big.Rat)
			for k := 0; k <= i; k++ {
				fu := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(du-2), int64(k)))
				for l := 0; l <= j; l++ {
					fv := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(l)), new(big.Int).Binomial(int64(dv-2), int64(l)))
					coefficient.Add(coefficient, new(big.Rat).Mul(q[k][l], new(big.Rat).Mul(fu, fv)))
					if e := b.check(coefficient); e != nil {
						return nil, e
					}
				}
			}
			result.net[i*result.countV+j][2] = coefficient
		}
	}
	return &result, nil
}
