package geom

import (
	"context"
	"math/big"
)

// SplineGraphQuadraticEllipseArcs is a complete positively oriented UV loop.
// Arcs[i] runs on [0,1] from Junctions[i] to Junctions[(i+1)%4], with Z=0.
// Every arc has degree two and strictly positive rational weights. They are not
// necessarily equal-angle quadrants. Controls may lie outside the chart though
// the entire conic is certified strictly inside. Exact descriptors need not be
// losslessly exportable as float64 B-splines.
type SplineGraphQuadraticEllipseArcs struct {
	Complete        bool
	Certificate     SplineGraphQuadraticEllipse
	Arcs            [4]*ExactSplineCurve
	Junctions       [4][2]*big.Rat
	ExactOperations int
}

// BuildSplineGraphQuadraticEllipseArcs revalidates the original pair, then checks
// the supplied rational point exactly on the conic. A rational conjugate vector
// gives a projective parametrization without square roots or a rational circle
// basis. Four overlapping projective parameter charts cover the conic once,
// meeting only at the exact junctions. No model-curve composition is implied.
func BuildSplineGraphQuadraticEllipseArcs(ctx context.Context, a, c *SurfacePatch, h GraphQuadraticEllipseHint, point [2]*big.Rat, o GraphPointContactOptions) (r SplineGraphQuadraticEllipseArcs, err error) {
	return buildSplineGraphQuadraticEllipseArcs(ctx, a, c, h, point, o, 1)
}

// BuildSplineGraphRepeatedQuadraticEllipseArcs proves doubled contact before
// constructing the exact directed rational UV loop through the supplied
// point. The same strict height sign holds inside and outside the loop.
func BuildSplineGraphRepeatedQuadraticEllipseArcs(ctx context.Context, a, c *SurfacePatch, h GraphQuadraticEllipseHint,
	point [2]*big.Rat, o GraphPointContactOptions) (SplineGraphQuadraticEllipseArcs, error) {
	return buildSplineGraphQuadraticEllipseArcs(ctx, a, c, h, point, o, 2)
}

func buildSplineGraphQuadraticEllipseArcs(ctx context.Context, a, c *SurfacePatch, h GraphQuadraticEllipseHint,
	point [2]*big.Rat, o GraphPointContactOptions, multiplicity int) (r SplineGraphQuadraticEllipseArcs, err error) {
	if ctx == nil || point[0] == nil || point[1] == nil {
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
	certificate, e := certifySplineGraphQuadraticEllipse(ctx, a, c, h, o, multiplicity)
	r.Certificate = certificate
	r.ExactOperations = certificate.ExactOperations
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - certificate.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = certificate.ExactOperations + budget.operations }()
	for _, x := range point {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	q := certificate.Ellipse.Coefficients
	value := new(big.Rat).Set(q[0])
	for _, term := range []*big.Rat{new(big.Rat).Mul(q[1], point[0]), new(big.Rat).Mul(q[2], new(big.Rat).Mul(point[0], point[0])), new(big.Rat).Mul(q[3], point[1]), new(big.Rat).Mul(q[4], new(big.Rat).Mul(point[1], point[1])), new(big.Rat).Mul(q[5], new(big.Rat).Mul(point[0], point[1]))} {
		value.Add(value, term)
		if e := budget.check(value); e != nil {
			return r, e
		}
	}
	if value.Sign() != 0 {
		return r, ErrDomain
	}
	center := certificate.Center
	p := [2]*big.Rat{new(big.Rat).Sub(point[0], center[0]), new(big.Rat).Sub(point[1], center[1])}
	halfCross := new(big.Rat).Quo(q[5], big.NewRat(2, 1))
	qp := [2]*big.Rat{new(big.Rat).Add(new(big.Rat).Mul(q[2], p[0]), new(big.Rat).Mul(halfCross, p[1])), new(big.Rat).Add(new(big.Rat).Mul(halfCross, p[0]), new(big.Rat).Mul(q[4], p[1]))}
	// det[p,JQp]=p^T Qp>0 fixes counterclockwise parameter orientation.
	w := [2]*big.Rat{new(big.Rat).Neg(qp[1]), new(big.Rat).Set(qp[0])}
	norm := func(v [2]*big.Rat) *big.Rat {
		return new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(q[2], new(big.Rat).Mul(v[0], v[0])), new(big.Rat).Mul(q[4], new(big.Rat).Mul(v[1], v[1]))), new(big.Rat).Mul(q[5], new(big.Rat).Mul(v[0], v[1])))
	}
	denominator := norm(p)
	if denominator.Sign() <= 0 {
		return r, ErrConvergence
	}
	k := new(big.Rat).Quo(norm(w), denominator)
	if k.Sign() <= 0 {
		return r, ErrConvergence
	}
	if e := budget.check(k); e != nil {
		return r, e
	}
	// Choose a rational projective chart scale near 1/sqrt(k) when
	// its exact value is rational. This recovers a balanced four-arc
	// partition and can make otherwise nonrepresentable controls dyadic.
	n, d := new(big.Int).Sqrt(k.Num()), new(big.Int).Sqrt(k.Denom())
	if new(big.Int).Mul(n, n).Cmp(k.Num()) == 0 && new(big.Int).Mul(d, d).Cmp(k.Denom()) == 0 {
		scale := new(big.Rat).SetFrac(d, n)
		k = big.NewRat(1, 1)
		for i := range w {
			w[i].Mul(w[i], scale)
			if e := budget.check(w[i]); e != nil {
				return r, e
			}
		}
	}
	for arc := 0; arc < 4; arc++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		reciprocal := arc == 1 || arc == 2
		reverse := arc == 1 || arc == 3
		direction := int64(1)
		if arc >= 2 {
			direction = -1
		}
		// Homogeneous Bernstein coefficients for direct and reciprocal charts.
		weights := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat).Add(big.NewRat(1, 1), k)}
		pCoefficients := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat).Sub(big.NewRat(1, 1), k)}
		if reciprocal {
			weights[0] = new(big.Rat).Set(k)
			weights[1] = new(big.Rat).Set(k)
			pCoefficients[0] = new(big.Rat).Neg(k)
			pCoefficients[1] = new(big.Rat).Neg(k)
		}
		wCoefficients := [3]*big.Rat{new(big.Rat), big.NewRat(direction, 1), big.NewRat(2*direction, 1)}
		net := make([]exactHomogeneous, 3)
		for i := 0; i < 3; i++ {
			net[i][3] = new(big.Rat).Set(weights[i])
			net[i][2] = new(big.Rat)
			for axis := 0; axis < 2; axis++ {
				net[i][axis] = new(big.Rat).Add(new(big.Rat).Mul(center[axis], weights[i]), new(big.Rat).Add(new(big.Rat).Mul(p[axis], pCoefficients[i]), new(big.Rat).Mul(w[axis], wCoefficients[i])))
			}
			for _, x := range net[i] {
				if e := budget.check(x); e != nil {
					return r, e
				}
			}
		}
		if reverse {
			net[0], net[2] = net[2], net[0]
		}
		r.Arcs[arc], e = exactCurveFromHomogeneousBezier(net, TubeAffineLineOptions{MaxPoles: 6, MaxOperations: budget.maxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, budget)
		if e != nil {
			return r, e
		}
		for axis := 0; axis < 2; axis++ {
			r.Junctions[arc][axis] = new(big.Rat).Quo(net[0][axis], net[0][3])
		}
	}
	// Exact join checks protect chart orientation/reversal implementation.
	for arc, curve := range r.Arcs {
		last := curve.patch.net[(curve.patch.countU-1)*curve.patch.countV]
		for axis := 0; axis < 2; axis++ {
			end := new(big.Rat).Quo(last[axis], last[3])
			if end.Cmp(r.Junctions[(arc+1)%4][axis]) != 0 {
				return r, ErrConvergence
			}
		}
	}
	r.Complete = true
	return r, nil
}
