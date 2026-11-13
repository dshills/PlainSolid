package geom

import "math"

func homDerivativeOrder(degree int, poles []homogeneous, knots []float64, t float64, order int) homogeneous {
	if order > degree {
		return homogeneous{}
	}
	for step := 0; step < order; step++ {
		next := make([]homogeneous, len(poles)-1)
		for i := range next {
			den := knots[i+degree+1] - knots[i+1]
			if den != 0 {
				for axis := 0; axis < 4; axis++ {
					next[i][axis] = float64(degree) * (poles[i+1][axis] - poles[i][axis]) / den
				}
			}
		}
		poles = next
		knots = knots[1 : len(knots)-1]
		degree--
	}
	return evalHom(degree, poles, knots, t)
}

// SecondPartials returns analytic uu, uv and vv derivatives, including rational
// weight terms. At interior knots it selects the right-hand polynomial piece
// independently in each direction, and at final endpoints the left-hand piece.
// At knots lacking C2 continuity these are one-sided derivatives; they do not
// assert a classical second derivative exists across the knot.
func (s *BSplineSurface) SecondPartials(u, v float64) (uu, uv, vv Vec3, err error) {
	if !s.validParameter(u, v) {
		return uu, uv, vv, ErrDomain
	}
	poles := toHom(s.poles, s.weights)
	var rows [3][]homogeneous
	for j := range rows {
		rows[j] = make([]homogeneous, s.countU)
		for i := range rows[j] {
			rows[j][i] = homDerivativeOrder(s.degreeV, poles[i*s.countV:(i+1)*s.countV], s.knotsV, v, j)
		}
	}
	h := evalHom(s.degreeU, rows[0], s.knotsU, u)
	hu := homDerivativeOrder(s.degreeU, rows[0], s.knotsU, u, 1)
	hv := evalHom(s.degreeU, rows[1], s.knotsU, u)
	huu := homDerivativeOrder(s.degreeU, rows[0], s.knotsU, u, 2)
	huv := homDerivativeOrder(s.degreeU, rows[1], s.knotsU, u, 1)
	hvv := evalHom(s.degreeU, rows[2], s.knotsU, u)
	point, e := projectHom(h)
	if e != nil {
		return uu, uv, vv, e
	}
	du, e := rationalDerivative(h, hu)
	if e != nil {
		return uu, uv, vv, e
	}
	dv, e := rationalDerivative(h, hv)
	if e != nil {
		return uu, uv, vv, e
	}
	second := func(d homogeneous, a, b Vec3, wa, wb float64) Vec3 {
		return Vec3{(d[0] - point.X*d[3] - a.X*wb - b.X*wa) / h[3], (d[1] - point.Y*d[3] - a.Y*wb - b.Y*wa) / h[3], (d[2] - point.Z*d[3] - a.Z*wb - b.Z*wa) / h[3]}
	}
	uu = second(huu, du, du, hu[3], hu[3])
	uv = second(huv, du, dv, hu[3], hv[3])
	vv = second(hvv, dv, dv, hv[3], hv[3])
	if !uu.IsFinite() || !uv.IsFinite() || !vv.IsFinite() {
		return Vec3{}, Vec3{}, Vec3{}, ErrInvalid
	}
	return uu, uv, vv, nil
}

// SurfaceCurvature contains signed curvature for the normal Su cross Sv.
// PrincipalMin <= PrincipalMax; reversing orientation reverses signed mean and
// principal curvatures. Gaussian curvature is orientation independent.
type SurfaceCurvature struct {
	Normal                                     Vec3
	Gaussian, Mean, PrincipalMin, PrincipalMax float64
}

// Curvature returns curvature of the selected smooth polynomial piece using
// SecondPartials' one-sided knot convention. At nonsmooth knots it is a one-sided
// value, not a claim of curvature continuity. Zero tangents return ErrDegenerate;
// tangent directions with sine <=1e-12 return ErrIllConditioned. Overflow or
// nonfinite differential quantities return ErrInvalid. No certified error bound
// is claimed for these floating-point curvature values.
func (s *BSplineSurface) Curvature(u, v float64) (result SurfaceCurvature, err error) {
	du, dv, e := s.Partials(u, v)
	if e != nil {
		return result, e
	}
	uu, uv, vv, e := s.SecondPartials(u, v)
	if e != nil {
		return result, e
	}
	lu, lv := du.Length(), dv.Length()
	if lu == 0 || lv == 0 {
		return result, ErrDegenerate
	}
	if !finite(lu) || !finite(lv) {
		return result, ErrInvalid
	}
	a, b := du.Scale(1/lu), dv.Scale(1/lv)
	cross := a.Cross(b)
	sin := cross.Length()
	if sin <= 1e-12 {
		return result, ErrIllConditioned
	}
	normal := cross.Scale(1 / sin)
	cos := a.Dot(b)
	// Express the second fundamental form in an orthonormal tangent basis.
	ee, ff, gg := uu.Dot(normal)/lu/lu, uv.Dot(normal)/lu/lv, vv.Dot(normal)/lv/lv
	aa, bb, cc := ee, (ff-cos*ee)/sin, (gg-2*cos*ff+cos*cos*ee)/sin/sin
	mean := aa/2 + cc/2
	radius := math.Hypot(aa/2-cc/2, bb)
	low, high := mean-radius, mean+radius
	// Recover a small eigenvalue from the determinant when cancellation dominates.
	determinant := aa*cc - bb*bb
	if math.Abs(low) < math.Abs(high) && high != 0 {
		low = determinant / high
	} else if low != 0 {
		high = determinant / low
	}
	gaussian := low * high
	if !normal.IsFinite() || !finite(low) || !finite(high) || !finite(mean) || !finite(gaussian) {
		return result, ErrInvalid
	}
	return SurfaceCurvature{normal, gaussian, mean, low, high}, nil
}
