package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicSphereBore retains exact crossing identity separately from native
// rounded charts. SeamSquaredBound bounds the mathematical sphere/cylinder
// charts at their represented trim values in model coordinates; platform
// evaluator rounding remains covered only by native sewing tolerance.
type AlgebraicSphereBore struct {
	Solid            Solid
	Crossing         *geom.ExactAlgebraicRoot
	CrossingInterval geom.ExactParameterInterval
	RoundedHeight    float64
	SeamSquaredBound *big.Rat
	VolumePiInterval [2]*big.Rat
}

// SphereBlindBoreAlgebraic extends the strict constructor to irrational circle
// heights. Exact root isolation is bounded by options; the seam proof uses a
// fixed 49-term rational Taylor evaluation. Both must finish successfully;
// no partial or unproved native result is returned. Floor clearance is proved
// against the complete root interval. Ideal volume intervals include the exact
// represented frame determinant; they are not quadrature error estimates.
func SphereBlindBoreAlgebraic(ctx context.Context, frame geom.Frame, R, r, floor, tolerance float64, o geom.ExactCurvePlaneEventOptions) (AlgebraicSphereBore, error) {
	fail := func(e error) (AlgebraicSphereBore, error) { return AlgebraicSphereBore{}, e }
	if ctx == nil || !finite(R) || !finite(r) || !finite(floor) || !finite(tolerance) || R <= r || r <= 0 || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	add := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	div := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
	sq := func(a *big.Rat) *big.Rat { return mul(a, a) }
	delta := sub(sq(q(R)), sq(q(r)))
	root, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{new(big.Rat).Neg(delta).RatString(), "0", "1"}, Lower: "0", Upper: q(R).RatString()}, geom.ExactRootCodecOptions{ExactCurvePlaneEventOptions: o})
	if e != nil {
		return fail(e)
	}
	enclosure, e := root.Enclose(ctx, div(q(tolerance), big.NewRat(64, 1)), o)
	if e != nil {
		return fail(e)
	}
	if !enclosure.Converged {
		return fail(geom.ErrConvergence)
	}
	lo, hi := enclosure.Interval.Lower, enclosure.Interval.Upper
	pad := mul(q(tolerance), big.NewRat(8, 1))
	for _, gap := range []*big.Rat{q(r), sub(q(R), q(r)), sub(lo, q(floor)), add(lo, q(floor))} {
		if gap.Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
	}
	z, _ := div(add(lo, hi), big.NewRat(2, 1)).Float64()
	latitude := math.Atan2(z, r)
	if !finite(z) || !finite(latitude) || latitude <= 0 || latitude >= math.Pi/2 {
		return fail(geom.ErrConvergence)
	}
	// Taylor polynomials of degree48; for real |x|<2 the order49
	// Lagrange remainder is bounded by |x|^49/49! for both sine and cosine.
	if math.Abs(latitude) >= 2 {
		return fail(geom.ErrConvergence)
	}
	sine, cosine := new(big.Rat), new(big.Rat)
	term := big.NewRat(1, 1)
	for n := 0; n <= 48; n++ {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		switch n % 4 {
		case 0:
			cosine.Add(cosine, term)
		case 1:
			sine.Add(sine, term)
		case 2:
			cosine.Sub(cosine, term)
		case 3:
			sine.Sub(sine, term)
		}
		term = div(mul(term, q(latitude)), big.NewRat(int64(n+1), 1))
	}
	remainder := new(big.Rat).Abs(term)
	errorMax := func(mid, target *big.Rat) *big.Rat {
		return add(new(big.Rat).Abs(sub(mid, target)), mul(q(R), remainder))
	}
	radial := errorMax(mul(q(R), cosine), q(r))
	axial := errorMax(mul(q(R), sine), q(z))
	x, y, axis := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	a, b, c := [3]float64{x.X, x.Y, x.Z}, [3]float64{y.X, y.Y, y.Z}, [3]float64{axis.X, axis.Y, axis.Z}
	det, norm := new(big.Rat), new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		cross := sub(mul(q(b[j]), q(c[k])), mul(q(b[k]), q(c[j])))
		det.Add(det, mul(q(a[i]), cross))
		norm.Add(norm, add(add(sq(q(a[i])), sq(q(b[i]))), sq(q(c[i]))))
	}
	det.Abs(det)
	if det.Sign() == 0 {
		return fail(geom.ErrInvalid)
	}

	seam := mul(norm, add(sq(radial), sq(axial)))
	if seam.Cmp(div(sq(q(tolerance)), big.NewRat(16, 1))) > 0 {
		return fail(geom.ErrConvergence)
	}
	sphere, e := geom.NewSphereSurface(frame, R)
	if e != nil {
		return fail(e)
	}
	cylinder, e := geom.NewCylinderSurface(frame, r)
	if e != nil {
		return fail(e)
	}
	ff, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: floor}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return fail(e)
	}
	var faces []Face
	for i := 0; i < 4; i++ {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		faces = append(faces, Face{Surface: sphere, U: u, V: Interval{-math.Pi / 2, latitude}}, Face{Surface: cylinder, U: u, V: Interval{floor, z}, Reversed: true}, Face{Surface: disk{ff}, U: u, V: Interval{0, r}, Reversed: true})
	}
	solid, e := joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	cap := func(z *big.Rat) *big.Rat { h := sub(q(R), z); return mul(sq(h), sub(q(R), div(h, big.NewRat(3, 1)))) }
	full := mul(big.NewRat(4, 3), mul(sq(q(R)), q(R)))
	removedLo := add(mul(sq(q(r)), sub(lo, q(floor))), cap(hi))
	removedHi := add(mul(sq(q(r)), sub(hi, q(floor))), cap(lo))
	result := AlgebraicSphereBore{Solid: solid, Crossing: root, CrossingInterval: enclosure.Interval, RoundedHeight: z, SeamSquaredBound: seam, VolumePiInterval: [2]*big.Rat{mul(det, sub(full, removedHi)), mul(det, sub(full, removedLo))}}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return result, nil
}
