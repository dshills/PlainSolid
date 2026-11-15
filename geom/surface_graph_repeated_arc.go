package geom

import (
	"context"
	"math/big"
)

// SplineGraphRepeatedArc is the full model contact of two single-cell graphs
// with identical injective XY charts. It represents one regular rational arc.
// HeightSign is the strict sign of A.Z-B.Z everywhere off the arc. Along
// the arc the residual has multiplicity two in the dependent coordinate.
type SplineGraphRepeatedArc struct {
	Proven                    bool
	DependentAxis, HeightSign int
	Domain                    ParameterRectangle
	UVCurve                   *ExactSplineCurve
	Endpoints                 [2][2]*big.Rat
	ModelEndpoints            [2][3]*big.Rat
	Projection                SurfaceInjectivity
	ExactOperations           int
}

// DiscoverSplineGraphRepeatedArc examines the full homogeneous difference
// in the original common chart. The numerator must have exact degree two in
// dependent, with a zero discriminant and nonvanishing leading coefficient.
// The rational double root must stay strictly between the two dependent
// boundaries. The returned curve uses original chart coordinates and is
// directed by increasing free coordinate.
func DiscoverSplineGraphRepeatedArc(ctx context.Context, a, c *SurfacePatch, dependent int, o GraphPointContactOptions) (r SplineGraphRepeatedArc, err error) {
	if ctx == nil || a == nil || c == nil || dependent < 0 || dependent > 1 {
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
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if difference.countU != difference.degreeU+1 || difference.countV != difference.degreeV+1 || difference.degreeU > 8 || difference.degreeV > 8 {
		return r, ErrConvergence
	}
	p, e := quadraticHeightCoefficients(ctx, difference, dependent, b)
	if e != nil {
		return r, e
	}
	constantTimesLeading, e := polynomialProduct(p[0], p[2], b)
	if e != nil {
		return r, e
	}
	linearSquared, e := polynomialProduct(p[1], p[1], b)
	if e != nil {
		return r, e
	}
	discriminant, e := subtractScaledPolynomial(linearSquared, constantTimesLeading, big.NewRat(4, 1), b)
	if e != nil {
		return r, e
	}
	if len(discriminant) != 0 {
		return r, ErrConvergence
	}
	unit := exactInterval{lo: new(big.Rat), hi: big.NewRat(1, 1)}
	leading, e := polynomialBernsteinInterval(p[2], unit, b)
	if e != nil {
		return r, e
	}
	sign := leading.sign()
	if sign == 0 {
		return r, ErrConvergence
	}
	w, e := subtractScaledPolynomial(nil, p[2], big.NewRat(int64(-2*sign), 1), b)
	if e != nil {
		return r, e
	}
	n, e := subtractScaledPolynomial(nil, p[1], big.NewRat(int64(sign), 1), b)
	if e != nil {
		return r, e
	}
	upper, e := subtractScaledPolynomial(w, n, big.NewRat(1, 1), b)
	if e != nil {
		return r, e
	}
	for _, bound := range []rationalPolynomial{n, upper} {
		interval, e := polynomialBernsteinInterval(bound, unit, b)
		if e != nil {
			return r, e
		}
		if interval.lo.Sign() <= 0 {
			return r, ErrConvergence
		}
	}
	d := difference.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var power [4]rationalPolynomial
	power[3] = w
	for axis, limits := range ranges {
		lo, hi := new(big.Rat).SetFloat64(limits[0]), new(big.Rat).SetFloat64(limits[1])
		w0, e := subtractScaledPolynomial(nil, w, new(big.Rat).Neg(lo), b)
		if e != nil {
			return r, e
		}
		coord := n
		if axis != dependent {
			coord = append(rationalPolynomial{new(big.Rat)}, w...)
		}
		power[axis], e = subtractScaledPolynomial(w0, coord, new(big.Rat).Sub(lo, hi), b)
		if e != nil {
			return r, e
		}
	}
	degree := max(len(power[0]), len(power[1]), len(w)) - 1
	controls := make([]exactHomogeneous, degree+1)
	for i := range controls {
		for axis, coeffs := range power {
			controls[i][axis], e = polynomialBernsteinControl(coeffs, degree, i, b)
			if e != nil {
				return r, e
			}
		}
	}
	curve, e := exactCurveFromHomogeneousBezier(controls, TubeAffineLineOptions{MaxPoles: 4096, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, b)
	if e != nil {
		return r, e
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	projection, e := CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells,
		MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += projection.ExactOperations
	if e != nil {
		return r, e
	}
	if !projection.ProvenInjective {
		return r, ErrConvergence
	}
	for i, h := range []exactHomogeneous{controls[0], controls[len(controls)-1]} {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		uv := [2]*big.Rat{new(big.Rat).Quo(h[0], h[3]), new(big.Rat).Quo(h[1], h[3])}
		point, _, e := exactSurfaceJet(a, uv[0], uv[1], b)
		if e != nil {
			return r, e
		}
		r.Endpoints[i], r.ModelEndpoints[i] = uv, point
	}
	r.Proven, r.Domain, r.DependentAxis, r.HeightSign = true, d, dependent, sign
	r.Projection, r.UVCurve = projection, curve
	return r, nil
}

func quadraticHeightCoefficients(ctx context.Context, surface *SurfacePatch, dependent int, b *polynomialBudget) (out [3]rationalPolynomial, err error) {
	counts := [2]int{surface.countU, surface.countV}
	var columns [3][]*big.Rat
	for i := range columns {
		columns[i] = make([]*big.Rat, counts[1-dependent])
	}
	row := make([]*big.Rat, counts[dependent])
	for i := range columns[0] {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		for j := range row {
			uv := [2]int{}
			uv[dependent], uv[1-dependent] = j, i
			row[j] = surface.net[uv[0]*surface.countV+uv[1]][2]
		}
		coeffs, e := bernsteinToPower(row, b)
		if e != nil {
			return out, e
		}
		if len(coeffs) > 3 {
			return out, ErrConvergence
		}
		for j := range columns {
			columns[j][i] = new(big.Rat)
			if j < len(coeffs) {
				columns[j][i] = coeffs[j]
			}
		}
	}
	for i, col := range columns {
		out[i], err = bernsteinToPower(col, b)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
