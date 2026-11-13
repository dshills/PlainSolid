package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func unionSeparatedCoordinateCylinders(ctx context.Context, a, b geom.RawCylinder) (FiniteCylinderPairUnionResult, error) {
	var zero FiniteCylinderPairUnionResult
	if !parallelCylinderSupportsSeparated(a, b, true) {
		return zero, geom.ErrDomain
	}
	out := FiniteCylinderPairUnionResult{CommonVolumeSquared: new(big.Rat)}
	var parts [2]Solid
	n := 0
	for i, c := range [2]geom.RawCylinder{a, b} {
		s, err := nativeCoordinateCylinder(c)
		if err != nil {
			return zero, err
		}
		parts[i] = s
		out.WallFaces[i], out.LowerCapFaces[i], out.UpperCapFaces[i] = []int{n}, []int{n + 1}, []int{n + 2}
		n += len(s.faces)
	}
	var err error
	out.Solid, err = joinIndependent(parts[0], parts[1])
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}

// nativeCoordinateCylinder retains the physical raw cap centers, including
// the Min/Max ordering of a negative direction. Inputs are prevalidated.
func nativeCoordinateCylinder(c geom.RawCylinder) (Solid, error) {
	unit, scale := coordinateCylinderAxis(c.Axis)
	if scale == 0 {
		return Solid{}, geom.ErrDomain
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var start geom.Vec3
	for i, p := range [3]*float64{&start.X, &start.Y, &start.Z} {
		coord := [3]float64{c.Origin.X, c.Origin.Y, c.Origin.Z}[i]
		dir := [3]float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[i]
		cap := func(t float64) (float64, error) {
			return geom.RepresentedExactScalar(new(big.Rat).Add(q(coord), new(big.Rat).Mul(q(t), q(dir))))
		}
		v, err := cap(c.Min)
		if err != nil {
			return Solid{}, err
		}
		if _, err := cap(c.Max); err != nil {
			return Solid{}, err
		}
		*p = v
	}
	length := new(big.Rat).Sub(q(c.Max), q(c.Min))
	height, err := geom.RepresentedExactScalar(length.Mul(length, q(scale)))
	if err != nil {
		return Solid{}, err
	}
	frame, err := geom.FrameFromNormal(start, unit)
	if err != nil {
		return Solid{}, err
	}
	return Cylinder(frame, c.Radius, height)
}

// finiteContainedCylinderUnion retains the outer original boundary once the
// exact perpendicular containment test has excluded every exposed inner face.
// The common volume is that of the contained cylinder; its square includes π²
// and has no rational representation. Return the exact coefficient separately.
func finiteContainedCylinderUnion(ctx context.Context, a, b geom.RawCylinder) (FiniteCylinderPairUnionResult, bool, error) {
	var zero FiniteCylinderPairUnionResult
	inner, ok, err := finiteRawPerpendicularContainment(ctx, a, b)
	if err != nil || !ok {
		return zero, ok, err
	}
	operands := [2]geom.RawCylinder{a, b}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	outer := 1 - inner
	s, err := nativeCoordinateCylinder(operands[outer])
	if err != nil {
		return zero, true, err
	}
	c := operands[inner]
	_, scale := coordinateCylinderAxis(c.Axis)
	r := q(c.Radius)
	coefficient := new(big.Rat).Mul(new(big.Rat).Mul(r, r), new(big.Rat).Mul(q(scale), new(big.Rat).Sub(q(c.Max), q(c.Min))))
	out := FiniteCylinderPairUnionResult{Solid: s, CommonVolumePiCoefficient: coefficient}
	out.WallFaces[outer], out.LowerCapFaces[outer], out.UpperCapFaces[outer] = []int{0}, []int{1}, []int{2}
	return out, true, nil
}

func finiteRawPerpendicularContainment(ctx context.Context, a, b geom.RawCylinder) (int, bool, error) {
	if a.Radius == b.Radius {
		return 0, false, nil
	}
	ua, _ := coordinateCylinderAxis(a.Axis)
	ub, _ := coordinateCylinderAxis(b.Axis)
	if ua == (geom.Vec3{}) || ub == (geom.Vec3{}) || ua.Dot(ub) != 0 {
		return 0, false, nil
	}
	m, err := geom.NewObliqueSkewCylinderMap(ctx, a.Origin, a.Axis, a.Radius, b.Origin, b.Axis, b.Radius, geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		return 0, false, err
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var windows [2][2]*big.Rat
	for i, c := range [2]geom.RawCylinder{a, b} {
		windows[i] = [2]*big.Rat{new(big.Rat).Sub(q(c.Min), m.ClosestRawParameters[i]), new(big.Rat).Sub(q(c.Max), m.ClosestRawParameters[i])}
	}
	owner, ok := perpendicularCylinderContainmentOwner(a, b, windows, m.SeparationSquared)
	return owner, ok, nil
}
