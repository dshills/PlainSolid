package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func unionSeparatedCoordinateCylinders(ctx context.Context, a, b geom.RawCylinder) (ObliqueEqualCylinderUnionResult, error) {
	var zero ObliqueEqualCylinderUnionResult
	if !parallelCylinderSupportsSeparated(a, b, true) {
		return zero, geom.ErrDomain
	}
	out := ObliqueEqualCylinderUnionResult{CommonVolumeSquared: new(big.Rat)}
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
