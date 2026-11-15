package parametric

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// SphericalLensDilationHistory retains the original sphere owners and the
// generated rolling blend. The blend uses the original contact circle as
// its centerline. A zero displacement reports BlendFace=-1.
type SphericalLensDilationHistory struct {
	SpherePairOffsetHistory
	BlendFace              int
	RimCenter              geom.Vec3
	RimRadius, BlendRadius float64
}

// ClassifyDilatedSphericalLens uses exact represented coordinates to classify
// the parallel body of this original lens. The analytic query can handle a
// nonrepresentable circular rim without rounding a native support.
func (s SpherePairSet) ClassifyDilatedSphericalLens(ctx context.Context, p geom.Vec3, distance float64, o geom.SphereFullBoundaryDistanceOptions) (geom.SphereLensDilationPoint, error) {
	if !s.valid || !p.IsFinite() || !finite(distance) {
		return geom.SphereLensDilationPoint{}, geom.ErrInvalid
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	return s.query.ClassifyDilatedLensRational(ctx, [3]*big.Rat{q(p.X), q(p.Y), q(p.Z)}, q(distance), o)
}

// ClipDilatedSphericalLensLine retains exact raw-t intervals for the original
// lens parallel body and identifies the original cap or rim at each endpoint.
func (s SpherePairSet) ClipDilatedSphericalLensLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT, distance float64, o geom.ToroidLineOptions) (geom.SphereLensDilationLine, error) {
	if !s.valid || !origin.IsFinite() || !direction.IsFinite() || !finite(minT) || !finite(maxT) || !finite(distance) {
		return geom.SphereLensDilationLine{}, geom.ErrInvalid
	}
	q := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	p := func(v geom.Vec3) [3]*big.Rat { return [3]*big.Rat{q(v.X), q(v.Y), q(v.Z)} }
	return s.query.ClipDilatedLensRationalLine(ctx, p(origin), p(direction), q(minT), q(maxT), q(distance), o)
}

// DilateSphericalLens dilates an originally transverse ball intersection
// using its two outward-offset spherical caps and their rolling torus band.
// The original contact circle's center and radius and both expanded radii
// must be exactly representable. The rolling band is restricted to the regular
// outer half when a complete torus would be singular. Angular trim endpoints
// use the native analytic sewing tolerance.
func (s SpherePairSet) DilateSphericalLens(ctx context.Context, distance, tolerance float64) (Solid, SphericalLensDilationHistory, error) {
	var empty Solid
	noHistory := SphericalLensDilationHistory{SpherePairOffsetHistory: emptySpherePairOffsetHistory(), BlendFace: -1}
	if ctx == nil || !s.valid || !finite(distance) || distance < 0 || !finite(tolerance) || tolerance <= 0 {
		return empty, noHistory, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, noHistory, err
	}
	balls, owners, err := s.sphereOffsetOwners(Intersection, false)
	if err != nil {
		return empty, noHistory, err
	}
	transverse, err := s.query.HasTransverseBoundaryIntersection()
	if err != nil {
		return empty, noHistory, err
	}
	if !transverse {
		return empty, noHistory, geom.ErrDomain
	}
	q := func(value float64) *big.Rat { return new(big.Rat).SetFloat64(value) }
	history := noHistory
	history.SourceFaces = owners
	delta := q(distance)
	for i, ball := range balls {
		history.Radii[i] = new(big.Rat).Add(q(ball.Radius), delta)
	}
	if distance == 0 {
		history.ResultFaces = owners
		return s.solid, history, nil
	}
	frames := [2]geom.Frame{}
	surfaces := [2]geom.SphereSurface{}
	for i, id := range owners {
		frames[i], _ = s.solid.faces[id].Surface.(geom.SphereSurface).Parameters()
		radius, err := geom.RepresentedExactScalar(history.Radii[i])
		if err != nil {
			return empty, noHistory, err
		}
		surfaces[i], err = geom.NewSphereSurface(frames[i], radius)
		if err != nil {
			return empty, noHistory, err
		}
	}
	center, rim, latitude, err := sphericalLensRim(balls, frames[0].ZAxis())
	if err != nil {
		return empty, noHistory, err
	}
	frame, err := geom.NewFrame(center, frames[0].XAxis(), frames[0].ZAxis())
	if err != nil {
		return empty, noHistory, err
	}
	var blend geom.Surface
	if distance < rim {
		blend, err = geom.NewTorusSurface(frame, rim, distance)
	} else {
		blend, err = geom.NewOuterTorusSurface(frame, rim, distance)
	}
	if err != nil {
		return empty, noHistory, err
	}
	south := Face{Surface: surfaces[1], U: circleDomain, V: Interval{-math.Pi / 2, latitude[1]},
		Boundary: [4]Use{{0, false}, {1, false}, {2, false}, {1, true}}}
	middle := Face{Surface: blend, U: circleDomain, V: Interval{latitude[1], latitude[0]},
		Boundary: [4]Use{{2, true}, {3, false}, {4, false}, {3, true}}}
	north := Face{Surface: surfaces[0], U: circleDomain, V: Interval{latitude[0], math.Pi / 2},
		Boundary: [4]Use{{4, true}, {5, false}, {6, false}, {5, true}}}
	vertices := make([]geom.Vec3, 4)
	for i, face := range [...]Face{south, middle, north} {
		vertices[i], err = face.Surface.Eval(0, face.V.Min)
		if err != nil {
			return empty, noHistory, err
		}
	}
	vertices[3], err = north.Surface.Eval(0, north.V.Max)
	if err != nil {
		return empty, noHistory, err
	}
	result, err := buildPatches(vertices, tolerance,
		patch{face: south, corners: [4]int{0, 0, 1, 1}, collapsed: [4]bool{true, false, false, false}},
		patch{face: middle, corners: [4]int{1, 1, 2, 2}},
		patch{face: north, corners: [4]int{2, 2, 3, 3}, collapsed: [4]bool{false, false, true, false}})
	if err != nil {
		return empty, noHistory, err
	}
	if err := ctx.Err(); err != nil {
		return empty, noHistory, err
	}
	history.ResultFaces, history.BlendFace = [2]int{2, 0}, 1
	history.RimCenter, history.RimRadius, history.BlendRadius = center, rim, distance
	return result, history, nil
}

func sphericalLensRim(balls [2]Ball, axis geom.Vec3) (geom.Vec3, float64, [2]float64, error) {
	var zero geom.Vec3
	var latitude [2]float64
	q := func(value float64) *big.Rat { return new(big.Rat).SetFloat64(value) }
	a, b := balls[0].Center, balls[1].Center
	xyzA := [3]float64{a.X, a.Y, a.Z}
	xyzB := [3]float64{b.X, b.Y, b.Z}
	n := [3]float64{axis.X, axis.Y, axis.Z}
	separation := new(big.Rat)
	for i := range n {
		step := new(big.Rat).Sub(q(xyzB[i]), q(xyzA[i]))
		separation.Add(separation, step.Mul(step, q(n[i])))
	}
	if separation.Sign() <= 0 {
		return zero, 0, latitude, geom.ErrDomain
	}
	square := func(value *big.Rat) *big.Rat { return new(big.Rat).Mul(value, value) }
	a2, b2, d2 := square(q(balls[0].Radius)), square(q(balls[1].Radius)), square(separation)
	axial := new(big.Rat).Sub(a2, b2)
	axial.Add(axial, d2)
	axial.Quo(axial, new(big.Rat).Mul(big.NewRat(2, 1), separation))
	radiusSquared := new(big.Rat).Sub(a2, square(axial))
	if radiusSquared.Sign() <= 0 {
		return zero, 0, latitude, geom.ErrDomain
	}
	num, den := new(big.Int).Sqrt(radiusSquared.Num()), new(big.Int).Sqrt(radiusSquared.Denom())
	if new(big.Int).Mul(num, num).Cmp(radiusSquared.Num()) != 0 || new(big.Int).Mul(den, den).Cmp(radiusSquared.Denom()) != 0 {
		return zero, 0, latitude, geom.ErrUnrepresentable
	}
	rim, err := geom.RepresentedExactScalar(new(big.Rat).SetFrac(num, den))
	if err != nil {
		return zero, 0, latitude, err
	}
	var center [3]float64
	for i := range n {
		position := new(big.Rat).Add(q(xyzA[i]), new(big.Rat).Mul(q(n[i]), axial))
		center[i], err = geom.RepresentedExactScalar(position)
		if err != nil {
			return zero, 0, latitude, err
		}
	}
	for i, x := range [2]*big.Rat{axial, new(big.Rat).Sub(axial, separation)} {
		ratio, _ := new(big.Rat).Quo(x, q(rim)).Float64()
		latitude[i] = math.Atan(ratio)
	}
	return geom.Vec3{X: center[0], Y: center[1], Z: center[2]}, rim, latitude, nil
}
