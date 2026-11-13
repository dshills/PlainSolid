package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// SphereBlindBoreResult retains a spherical exterior, cylindrical pocket wall
// and circular floor. CrossingHeight is the exactly represented positive root
// of R²-r². VolumePiCoefficient is the exact ideal affine material volume/π;
// native angular/cap charts are independently rounded within sewing tolerance.
type SphereBlindBoreResult struct {
	Solid                                  Solid
	CrossingHeight                         float64
	VolumePiCoefficient                    *big.Rat
	SphereFaces, CylinderFaces, FloorFaces [4]int
}

// SphereBlindBore removes an axial cylinder starting at floor and extending
// beyond the north pole of a sphere centered on frame.Origin. The floor must
// lie strictly between both transverse sphere/cylinder intersection circles.
// R>r>0; tangency, near-contact and nonrepresentable intersection roots reject.
// This bounded subset does not approximate an algebraic crossing height.
func SphereBlindBore(ctx context.Context, frame geom.Frame, R, r, floor, tolerance float64) (SphereBlindBoreResult, error) {
	fail := func(e error) (SphereBlindBoreResult, error) { return SphereBlindBoreResult{}, e }
	if ctx == nil || !finite(R) || !finite(r) || !finite(floor) || !finite(tolerance) || R <= 0 || r <= 0 || r >= R || tolerance <= 0 {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	delta := new(big.Rat).Sub(square(q(R)), square(q(r)))
	n, d := new(big.Int).Sqrt(delta.Num()), new(big.Int).Sqrt(delta.Denom())
	if new(big.Int).Mul(new(big.Int).Set(n), n).Cmp(delta.Num()) != 0 || new(big.Int).Mul(new(big.Int).Set(d), d).Cmp(delta.Denom()) != 0 {
		return fail(geom.ErrConvergence)
	}
	exactZ := new(big.Rat).SetFrac(n, d)
	z, e := geom.RepresentedExactScalar(exactZ)
	if e != nil {
		return fail(e)
	}
	pad := new(big.Rat).Mul(q(tolerance), big.NewRat(8, 1))
	for _, gap := range []*big.Rat{q(r), new(big.Rat).Sub(q(R), q(r)), new(big.Rat).Sub(exactZ, q(floor)), new(big.Rat).Add(exactZ, q(floor))} {
		if gap.Cmp(pad) <= 0 {
			return fail(geom.ErrConvergence)
		}
	}
	sphere, e := geom.NewSphereSurface(frame, R)
	if e != nil {
		return fail(e)
	}
	cylinder, e := geom.NewCylinderSurface(frame, r)
	if e != nil {
		return fail(e)
	}
	floorFrame, e := geom.NewFrame(frame.ToWorld(geom.Vec3{Z: floor}), frame.XAxis(), frame.ZAxis())
	if e != nil {
		return fail(e)
	}
	latitude := math.Atan2(z, r)
	result := SphereBlindBoreResult{CrossingHeight: z}
	var faces []Face
	for i := 0; i < 4; i++ {
		u := Interval{float64(i) * math.Pi / 2, float64(i+1) * math.Pi / 2}
		result.SphereFaces[i] = len(faces)
		faces = append(faces, Face{Surface: sphere, U: u, V: Interval{-math.Pi / 2, latitude}})
		result.CylinderFaces[i] = len(faces)
		faces = append(faces, Face{Surface: cylinder, U: u, V: Interval{floor, z}, Reversed: true})
		result.FloorFaces[i] = len(faces)
		faces = append(faces, Face{Surface: disk{floorFrame}, U: u, V: Interval{0, r}, Reversed: true})
	}
	result.Solid, e = joinPrimitivePatches(faces, tolerance)
	if e != nil {
		return fail(e)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	// Removed volume = πr²(z-floor)+πh²(R-h/3), h=R-z.
	h := new(big.Rat).Sub(q(R), exactZ)
	removed := new(big.Rat).Add(new(big.Rat).Mul(square(q(r)), new(big.Rat).Sub(exactZ, q(floor))), new(big.Rat).Mul(square(h), new(big.Rat).Sub(q(R), new(big.Rat).Quo(h, big.NewRat(3, 1)))))
	volume := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 3), new(big.Rat).Mul(square(q(R)), q(R))), removed)
	x, y, axis := frame.XAxis(), frame.YAxis(), frame.ZAxis()
	a, b, c := [3]float64{x.X, x.Y, x.Z}, [3]float64{y.X, y.Y, y.Z}, [3]float64{axis.X, axis.Y, axis.Z}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		det.Add(det, new(big.Rat).Mul(q(a[i]), new(big.Rat).Sub(new(big.Rat).Mul(q(b[j]), q(c[k])), new(big.Rat).Mul(q(b[k]), q(c[j])))))
	}
	det.Abs(det)
	result.VolumePiCoefficient = new(big.Rat).Mul(volume, det)
	return result, nil
}
