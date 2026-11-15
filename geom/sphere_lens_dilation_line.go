package geom

import (
	"context"
	"math/big"
)

// SphereLensDilationLinePoint is an isolated point of the clipped line. Owners
// is zero for an interior singleton, one sphere for an offset cap, or both for
// the rolling band and its joins. Guard-polynomial multiplicity is not exposed.
type SphereLensDilationLinePoint struct {
	Parameter *ExactAlgebraicRoot
	Bounds    ExactParameterInterval
	Owners    BoreBoundaryMask
}

// SphereLensDilationLine represents a convex line clip. There is at most one
// nonzero interval, one singleton, or no result. Interval endpoint masks refer
// to the original cap or rim owners; interior clip limits have mask zero.
type SphereLensDilationLine struct {
	Interval *BoreBoundaryInterval
	Point    *SphereLensDilationLinePoint
}

// ClipDilatedLensRationalLine clips origin+t*direction against the convex
// parallel body of a transverse sphere intersection. It uses rational original
// coefficients even when the source rim or axis length is irrational. The
// complete support and guard partition is signed exactly. Positive displacement
// is the distance to the original material (not to either whole source ball).
// ParameterTolerance only bounds root brackets. The arithmetic/bit limits
// cover polynomial construction, shared isolation and sign work; each root
// codec, ordering and enclosure call is independently bounded by those limits.
func (s SpherePairMaterial) ClipDilatedLensRationalLine(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT, displacement *big.Rat, o ToroidLineOptions) (SphereLensDilationLine, error) {
	var zero SphereLensDilationLine
	if ctx == nil || s.radius2[0] == nil || displacement == nil || displacement.Sign() < 0 || minT == nil || maxT == nil || minT.Cmp(maxT) > 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return zero, ErrDomain
	}
	if displacement.Sign() == 0 {
		clip, err := s.ClipRationalLine(ctx, origin, direction, minT, maxT, o)
		if err != nil {
			return zero, err
		}
		return singleLensLine(clip.Intervals, clip.Points)
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return zero, ErrInvalid
	}
	limits, err := patchOptions(o.Surface)
	if err != nil {
		return zero, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := sphereOverlapArithmetic{ctx: ctx, b: b}
	for _, x := range []*big.Rat{minT, maxT, displacement} {
		a.check(x)
	}
	nonzero := false
	for i, d := range direction {
		if d == nil || origin[i] == nil {
			return zero, ErrInvalid
		}
		nonzero = nonzero || d.Sign() != 0
		a.check(d)
		a.check(origin[i])
		for _, center := range s.center {
			a.check(center[i])
		}
	}
	if !nonzero {
		return zero, ErrInvalid
	}
	for _, radius2 := range s.radius2 {
		a.check(radius2)
	}
	if a.err != nil {
		return zero, a.err
	}
	ops := lensLinePolynomials{&a}
	rim, err := s.deriveLensRim(&a)
	if err != nil {
		return zero, err
	}
	axis, center, d2 := rim.Axis, rim.Center, rim.AxisSquared
	fraction, h2 := rim.CenterFraction, rim.RadiusSquared
	signedDot := func(center [3]*big.Rat) rationalPolynomial {
		start, rate := new(big.Rat), new(big.Rat)
		for i, d := range axis {
			start = a.add(start, a.mul(d, a.sub(origin[i], center[i])))
			rate = a.add(rate, a.mul(d, direction[i]))
		}
		return rationalPolynomial{start, rate}
	}
	var polynomials [12]rationalPolynomial
	var offsets [2]int
	for i, source := range s.center {
		// The full-sphere foot exists at query distance sqrt(S) from the
		// source center. It is retained by the opposite cap's half-space
		// exactly when A−B√S is nonnegative. Both A and A²−B²S are guards.
		S := ops.distance(origin, direction, source)
		R := a.sqrt(s.radius2[i])
		ori := big.NewRat(int64(1-2*i), 1)
		B := a.mul(a.mul(ori, a.sub(fraction, big.NewRat(int64(i), 1))), d2)
		A := ops.scale(signedDot(source), a.mul(ori, R))
		Q := ops.add(ops.product(A, A), ops.scale(S, a.sub(new(big.Rat), a.mul(B, B))))
		low, high := a.sub(R, displacement), a.add(R, displacement)
		inner := rationalPolynomial{big.NewRat(-1, 1)}
		if low.Sign() >= 0 {
			inner = ops.add(rationalPolynomial{a.mul(low, low)}, ops.scale(S, big.NewRat(-1, 1)))
		}
		polynomials[i] = ops.subtractConstant(S, s.radius2[i])
		j := 2 + 4*i
		polynomials[j], polynomials[j+1], polynomials[j+2], polynomials[j+3] = ops.subtractConstant(S, a.mul(high, high)), inner, A, Q
		offsets[i] = B.Sign()
	}
	// Rim distance squared is |p−c|²+h²−2√(h²*rho²). C is that
	// expression's rational part less d². Squaring is safe only for C>=0.
	dot := signedDot(center)
	total := ops.distance(origin, direction, center)
	rho2 := ops.add(total, ops.scale(ops.product(dot, dot), a.div(big.NewRat(-1, 1), d2)))
	C := ops.add(total, rationalPolynomial{a.sub(h2, a.mul(displacement, displacement))})
	Q := ops.add(ops.product(C, C), ops.scale(rho2, a.mul(big.NewRat(-4, 1), h2)))
	polynomials[10], polynomials[11] = C, Q
	if a.err != nil {
		return zero, a.err
	}
	classify := func(sign []int) (BorePointLocation, BoreBoundaryMask) {
		// The partition treats BoreBoundary as retained material, with zero
		// owner mask for strict interior. An original material point is always
		// interior when displacement is nonzero.
		if sign[0] <= 0 && sign[1] <= 0 {
			return BoreBoundary, 0
		}
		const rim = SpherePairFirst | SpherePairSecond
		var owners BoreBoundaryMask
		for i, B := range offsets {
			j := 2 + 4*i
			retained := lensRadicalGuard(sign[j+2], B, sign[j+3])
			if retained < 0 || sign[j] > 0 || sign[j+1] > 0 {
				continue
			}
			if sign[j] < 0 && sign[j+1] < 0 {
				return BoreBoundary, 0
			}
			if retained == 0 {
				owners |= rim
			} else {
				owners |= BoreBoundaryMask(1 << i)
			}
		}
		if sign[10] < 0 || sign[11] < 0 {
			return BoreBoundary, 0
		}
		if sign[11] == 0 {
			owners |= rim
		}
		if owners != 0 {
			return BoreBoundary, owners
		}
		return BoreOutside, 0
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: limits.MaxOperations, MaxCoefficientBits: limits.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	out, err := intersectMaterialBoundaryRational(ctx, polynomials[:], minT, maxT, o, b, ro, classify)
	if err != nil {
		return zero, err
	}
	return singleLensLine(out.Intervals, out.Hits)
}

func singleLensLine(intervals []BoreBoundaryInterval, points []BoreLineHit) (SphereLensDilationLine, error) {
	if len(intervals)+len(points) > 1 || len(intervals) == 1 && intervals[0].Boundary != 0 {
		return SphereLensDilationLine{}, ErrConvergence
	}
	if len(intervals) == 1 {
		return SphereLensDilationLine{Interval: &intervals[0]}, nil
	}
	if len(points) == 1 {
		h := points[0]
		return SphereLensDilationLine{Point: &SphereLensDilationLinePoint{h.Parameter, h.Bounds, h.Boundary}}, nil
	}
	return SphereLensDilationLine{}, nil
}

// lensRadicalGuard is sign(A−B√S), given the signs of A, B and A²−B²S.
func lensRadicalGuard(A, B, Q int) int {
	if B == 0 {
		return A
	}
	if A == 0 && Q == 0 {
		return 0
	}
	if A*B <= 0 {
		return -B
	}
	return B * Q
}

type lensLinePolynomials struct{ *sphereOverlapArithmetic }

func (o lensLinePolynomials) product(a, b rationalPolynomial) rationalPolynomial {
	if o.err != nil {
		return nil
	}
	p, err := polynomialProduct(a, b, o.b)
	o.err = err
	return p
}

func (o lensLinePolynomials) scale(p rationalPolynomial, factor *big.Rat) rationalPolynomial {
	q := make(rationalPolynomial, len(p))
	for i, c := range p {
		q[i] = o.mul(c, factor)
	}
	return trimPolynomial(q)
}

func (o lensLinePolynomials) add(a, b rationalPolynomial) rationalPolynomial {
	p := make(rationalPolynomial, max(len(a), len(b)))
	for i := range p {
		left, right := new(big.Rat), new(big.Rat)
		if i < len(a) {
			left = a[i]
		}
		if i < len(b) {
			right = b[i]
		}
		p[i] = o.sphereOverlapArithmetic.add(left, right)
	}
	return trimPolynomial(p)
}

func (o lensLinePolynomials) subtractConstant(p rationalPolynomial, c *big.Rat) rationalPolynomial {
	return o.add(p, rationalPolynomial{o.sub(new(big.Rat), c)})
}

func (o lensLinePolynomials) distance(origin, direction, center [3]*big.Rat) rationalPolynomial {
	var p rationalPolynomial
	for i := range origin {
		line := rationalPolynomial{o.sub(origin[i], center[i]), direction[i]}
		p = o.add(p, o.product(line, line))
	}
	return p
}
