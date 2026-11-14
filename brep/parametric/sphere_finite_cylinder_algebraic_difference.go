package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicSphereDifferenceBody is a separate connected material component.
// VolumePiInterval bounds its ideal represented-affine volume divided by π.
// Face groups identify native result faces, including inward cylindrical walls.
type AlgebraicSphereDifferenceBody struct {
	Solid                                        Solid
	SphericalFaces, CylindricalFaces, FloorFaces []int
	VolumePiInterval                             [2]*big.Rat
}

// SphereFiniteCylinderAlgebraicDifferenceResult describes the main pocket body
// followed by a detached north cap. Crossing and cap rim proofs come from
// exact algebraic common and blind-bore construction; native chart sewing
// remains numerical, with the analytic seam bounds carried by these proofs.
type SphereFiniteCylinderAlgebraicDifferenceResult struct {
	Bodies      []AlgebraicSphereDifferenceBody
	CommonProof SphereFiniteCylinderAlgebraicResult
	PocketProof AlgebraicSphereBore
}

// SphereFiniteCylinderDifferenceAlgebraic implements a bounded two-component
// transverse subtraction. The cutter's lower plane must lie strictly between
// the two crossing circles, and its upper plane above the positive crossing
// and below the north pole. The upper cut therefore removes an entire axial
// band, separating a spherical cap from a body with an inward cylindrical hole.
// Other arrangements, pinches and unresolved ordering reject atomically.
func SphereFiniteCylinderDifferenceAlgebraic(ctx context.Context, frame geom.Frame, R, r, lower, upper, tolerance float64, o geom.ExactCurvePlaneEventOptions) (SphereFiniteCylinderAlgebraicDifferenceResult, error) {
	fail := func(e error) (SphereFiniteCylinderAlgebraicDifferenceResult, error) {
		return SphereFiniteCylinderAlgebraicDifferenceResult{}, e
	}
	if ctx == nil || !finite(R) || !finite(r) || !finite(lower) || !finite(upper) || !finite(tolerance) || R <= 0 || r <= 0 || r >= R || lower >= upper || upper <= 0 || upper >= R || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	mul := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
	sub := func(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
	delta := sub(mul(q(R), q(R)), mul(q(r), q(r)))
	if mul(q(lower), q(lower)).Cmp(delta) >= 0 || mul(q(upper), q(upper)).Cmp(delta) <= 0 {
		return fail(geom.ErrConvergence)
	}
	common, e := SphereFiniteCylinderCommonAlgebraic(ctx, frame, R, r, lower, upper, tolerance, o)
	if e != nil {
		return fail(e)
	}
	pocket, e := SphereBlindBoreAlgebraic(ctx, frame, R, r, lower, tolerance, o)
	if e != nil {
		return fail(e)
	}
	pad := mul(q(tolerance), big.NewRat(8, 1))
	if sub(q(upper), pocket.CrossingInterval.Upper).Cmp(pad) <= 0 {
		return fail(geom.ErrConvergence)
	}
	capRim := common.Rims[len(common.Rims)-1]
	if capRim.RadiusRoot == nil {
		return fail(geom.ErrConvergence)
	}
	sphere, e := geom.NewSphereSurface(frame, R)
	if e != nil {
		return fail(e)
	}
	ff, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: upper}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return fail(e)
	}
	main := AlgebraicSphereDifferenceBody{Solid: pocket.Solid, VolumePiInterval: [2]*big.Rat{new(big.Rat).Set(pocket.VolumePiInterval[0]), new(big.Rat).Set(pocket.VolumePiInterval[1])}}
	for i := 0; i < 4; i++ {
		main.SphericalFaces = append(main.SphericalFaces, 3*i)
		main.CylindricalFaces = append(main.CylindricalFaces, 3*i+1)
		main.FloorFaces = append(main.FloorFaces, 3*i+2)
	}
	capBody := AlgebraicSphereDifferenceBody{}
	var faces []Face
	for i := 0; i < 4; i++ {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		capBody.SphericalFaces = append(capBody.SphericalFaces, len(faces))
		faces = append(faces, Face{Surface: sphere, U: u, V: Interval{capRim.Latitude, math.Pi / 2}})
		capBody.FloorFaces = append(capBody.FloorFaces, len(faces))
		faces = append(faces, Face{Surface: disk{ff}, U: u, V: Interval{0, capRim.NativeRadius}})
	}
	capBody.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	x, y, z := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	a := [3][3]float64{{x.X, x.Y, x.Z}, {y.X, y.Y, y.Z}, {z.X, z.Y, z.Z}}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, mul(q(a[0][i]), sub(mul(q(a[1][j]), q(a[2][k])), mul(q(a[1][k]), q(a[2][j])))))
	}
	if det.Sign() <= 0 {
		return fail(geom.ErrInvalid)
	}
	h := sub(q(R), q(upper))
	volume := mul(det, mul(mul(h, h), sub(q(R), new(big.Rat).Quo(h, big.NewRat(3, 1)))))
	capBody.VolumePiInterval = [2]*big.Rat{new(big.Rat).Set(volume), new(big.Rat).Set(volume)}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return SphereFiniteCylinderAlgebraicDifferenceResult{Bodies: []AlgebraicSphereDifferenceBody{main, capBody}, CommonProof: common, PocketProof: pocket}, nil
}
