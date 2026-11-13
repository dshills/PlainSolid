package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// SteinmetzPositiveCap is the equal-radius cylinder common clipped by
// X≤center.X+Cap. The finite cylinder extends beyond the opposite radial end.
// SeamSquaredBound bounds ideal sin/cos split-point discrepancy from the exact
// Pythagorean seam coordinates; native floating evaluator error is additional.
type SteinmetzPositiveCap struct {
	Solid                         Solid
	Cap, SeamOrdinate, SplitAngle float64
	XAxisFaces, ZAxisFaces        []int
	CapFace                       int
	SeamSquaredBound              *big.Rat
	// Exact enclosure of ideal source volume, including split-angle error.
	IdealVolumeBounds [2]*big.Rat
}

// PerpendicularCylinderPositiveCap supports 0<cap<radius with exactly
// representable sqrt(radius²-cap²). All support surfaces remain actual
// cylinders or a plane. Rounded split angles are independently bounded by
// exact Taylor enclosures and sewn at tolerance, not claimed exact angles.
func PerpendicularCylinderPositiveCap(ctx context.Context, center geom.Vec3, radius, cap, xOtherHalfLength, zHalfLength, tolerance float64) (SteinmetzPositiveCap, error) {
	fail := func(e error) (SteinmetzPositiveCap, error) { return SteinmetzPositiveCap{}, e }
	if !finite(cap) || cap <= 0 || cap >= radius {
		return fail(geom.ErrInvalid)
	}
	if _, e := PerpendicularCylinderCommon(ctx, center, radius, xOtherHalfLength, zHalfLength, tolerance); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	a2 := new(big.Rat).Sub(sq(q(radius)), sq(q(cap)))
	n, d := new(big.Int).Sqrt(a2.Num()), new(big.Int).Sqrt(a2.Denom())
	ar := new(big.Rat).SetFrac(n, d)
	if sq(ar).Cmp(a2) != 0 {
		return fail(geom.ErrUnrepresentable)
	}
	a, e := geom.RepresentedExactScalar(ar)
	if e != nil {
		return fail(e)
	}
	return buildPositiveSteinmetzCap(ctx, center, radius, cap, tolerance, a, ar, ar)
}

func buildPositiveSteinmetzCap(ctx context.Context, center geom.Vec3, radius, cap, tolerance, a float64, alo, ahi *big.Rat) (SteinmetzPositiveCap, error) {
	fail := func(e error) (SteinmetzPositiveCap, error) { return SteinmetzPositiveCap{}, e }
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	angle, errC, errA, bound, e := steinmetzSplitAngle(ctx, radius, cap, tolerance, a, alo, ahi)
	if e != nil {
		return fail(e)
	}
	capX, e := geom.RepresentedExactScalar(new(big.Rat).Add(q(center.X), q(cap)))
	if e != nil {
		return fail(e)
	}
	ranges := []Interval{{-math.Pi / 2, -angle}, {-angle, angle}, {angle, math.Pi / 2}}
	var faces []Face
	var zx, xx []int
	for _, u := range ranges {
		zx = append(zx, len(faces))
		faces = append(faces, Face{Surface: steinmetzChart{center, radius, 0, -1}, U: u, V: Interval{-1, 1}, Reversed: true})
	}
	for _, i := range []int{0, 2} {
		zx = append(zx, len(faces))
		faces = append(faces, Face{Surface: steinmetzChart{center, radius, 0, 1}, U: ranges[i], V: Interval{-1, 1}})
	}
	for _, sign := range []int{1, -1} {
		for i, u := range ranges {
			xx = append(xx, len(faces))
			var surface geom.Surface = steinmetzChart{center, radius, 1, sign}
			v := Interval{-1, 1}
			if i == 1 {
				surface = steinmetzClippedCylinder{center, radius, cap, sign}
				v = Interval{0, 1}
			}
			faces = append(faces, Face{Surface: surface, U: u, V: v, Reversed: sign > 0})
		}
	}
	capID := len(faces)
	faces = append(faces, Face{Surface: steinmetzDisk{geom.Vec3{X: capX, Y: center.Y, Z: center.Z}, radius}, U: ranges[1], V: Interval{-1, 1}})
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	// Both true and represented angles are in (0,pi/2). The chord
	// difference of their radius-r vectors bounds angle error by 2*chord/r,
	// using sin(x)>=2x/pi and pi<4. Coordinate L1 error bounds the chord.
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	volumeAt := func(ordinate *big.Rat) *big.Rat {
		volume := mul(big.NewRat(16, 3), mul(sq(q(radius)), q(radius)))
		volume.Sub(volume, mul(big.NewRat(2, 3), mul(sq(ordinate), ordinate)))
		volume.Sub(volume, mul(big.NewRat(2, 1), mul(sq(q(radius)), ordinate)))
		volume.Add(volume, mul(big.NewRat(2, 1), mul(sq(q(radius)), mul(q(cap), q(angle)))))
		return volume
	}
	volumeError := mul(big.NewRat(4, 1), mul(q(radius), mul(q(cap), new(big.Rat).Add(errC, errA))))
	volumeBounds := [2]*big.Rat{new(big.Rat).Sub(volumeAt(ahi), volumeError), new(big.Rat).Add(volumeAt(alo), volumeError)}
	return SteinmetzPositiveCap{Solid: solid, Cap: cap, SeamOrdinate: a, SplitAngle: angle, XAxisFaces: xx, ZAxisFaces: zx, CapFace: capID, SeamSquaredBound: bound, IdealVolumeBounds: volumeBounds}, nil
}

// steinmetzSplitAngle certifies a represented angular split against its exact
// positive cap distance and enclosed ideal transverse ordinate.
func steinmetzSplitAngle(ctx context.Context, radius, cap, tolerance, a float64, alo, ahi *big.Rat) (float64, *big.Rat, *big.Rat, *big.Rat, error) {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	pad := new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance))
	if q(cap).Cmp(pad) <= 0 || alo.Cmp(pad) <= 0 || new(big.Rat).Sub(q(radius), q(cap)).Cmp(pad) <= 0 {
		return 0, nil, nil, nil, geom.ErrDomain
	}
	angle := math.Atan2(a, cap)
	if !finite(angle) || angle <= 0 || angle >= math.Pi/2 {
		return 0, nil, nil, nil, geom.ErrConvergence
	}
	// Degree48 Taylor, with |angle|^49/49! Lagrange remainder for sin/cos.
	sine, cosine, term := new(big.Rat), new(big.Rat), big.NewRat(1, 1)
	for k := 0; k <= 48; k++ {
		if e := ctx.Err(); e != nil {
			return 0, nil, nil, nil, e
		}
		switch k % 4 {
		case 0:
			cosine.Add(cosine, term)
		case 1:
			sine.Add(sine, term)
		case 2:
			cosine.Sub(cosine, term)
		case 3:
			sine.Sub(sine, term)
		}
		term.Quo(new(big.Rat).Mul(term, q(angle)), big.NewRat(int64(k+1), 1))
	}
	remainder := new(big.Rat).Mul(q(radius), new(big.Rat).Abs(term))
	errC := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(q(radius), cosine), q(cap))), remainder)
	errA := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(q(radius), sine), alo))
	far := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(q(radius), sine), ahi))
	if far.Cmp(errA) > 0 {
		errA = far
	}
	errA.Add(errA, remainder)
	bound := new(big.Rat).Add(sq(errC), sq(errA))
	if bound.Cmp(new(big.Rat).Quo(sq(q(tolerance)), big.NewRat(16, 1))) > 0 {
		return 0, nil, nil, nil, geom.ErrConvergence
	}
	return angle, errC, errA, bound, nil
}

// Actual X-axis cylindrical support with a nonconstant axial interval.
type steinmetzClippedCylinder struct {
	center      geom.Vec3
	radius, cap float64
	sign        int
}

func (s steinmetzClippedCylinder) Eval(u, v float64) (geom.Vec3, error) {
	p, _, _, e := s.evaluate(u, v)
	return p, e
}
func (s steinmetzClippedCylinder) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	_, a, b, e := s.evaluate(u, v)
	return a, b, e
}
func (s steinmetzClippedCylinder) evaluate(u, v float64) (geom.Vec3, geom.Vec3, geom.Vec3, error) {
	if !finite(u) || !finite(v) || u < -math.Pi/2 || u > math.Pi/2 || v < 0 || v > 1 {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrDomain
	}
	sin, cos := math.Sincos(u)
	a := s.radius * cos
	k := float64(s.sign)
	p := s.center.Add(geom.Vec3{X: v*s.cap - (1-v)*a, Y: s.radius * sin, Z: k * a})
	du := geom.Vec3{X: (1 - v) * s.radius * sin, Y: s.radius * cos, Z: -k * s.radius * sin}
	dv := geom.Vec3{X: s.cap + a}
	if !p.IsFinite() || !du.IsFinite() || !dv.IsFinite() {
		return geom.Vec3{}, geom.Vec3{}, geom.Vec3{}, geom.ErrInvalid
	}
	return p, du, dv, nil
}
func (s steinmetzClippedCylinder) secondDerivativeBounds(ctx context.Context, u, v Interval) (float64, float64, float64, error) {
	if e := ctx.Err(); e != nil {
		return 0, 0, 0, e
	}
	b := math.Nextafter(2*s.radius, math.Inf(1))
	if !finite(b) {
		return 0, 0, 0, geom.ErrInvalid
	}
	return b, b, 0, nil
}
