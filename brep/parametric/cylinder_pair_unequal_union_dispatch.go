package parametric

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func finiteUnequalPerpendicularUnion(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairUnionResult, error) {
	var zero FiniteCylinderPairUnionResult
	operands := [2]geom.RawCylinder{a, b}
	if a.Radius < b.Radius {
		operands[0], operands[1] = b, a
	}
	var axes [2]geom.Vec3
	var scale [2]float64
	for i, c := range operands {
		axes[i], scale[i] = coordinateCylinderAxis(c.Axis)
	}
	if scale[0] == 0 || scale[1] == 0 || axes[0].Dot(axes[1]) != 0 {
		return zero, geom.ErrDomain
	}
	center, caps, err := finiteCylinderCrossingFrame(ctx, operands[0], operands[1], tolerance)
	if err != nil {
		return zero, err
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var half [2]float64
	for i, window := range caps {
		if window[1] <= 0 || window[0] != -window[1] {
			return zero, geom.ErrDomain
		}
		length := new(big.Rat).Mul(q(window[1]), q(scale[i]))
		half[i], err = geom.RepresentedExactScalar(length)
		if err != nil {
			return zero, err
		}
		coordinate := q(center.Dot(axes[i]))
		for _, sign := range []int64{-1, 1} {
			cap := new(big.Rat).Add(coordinate, new(big.Rat).Mul(length, big.NewRat(sign, 1)))
			if _, err = geom.RepresentedExactScalar(cap); err != nil {
				return zero, err
			}
		}
	}
	local, err := UnequalPerpendicularCylinderUnion(ctx, geom.Vec3{}, operands[0].Radius, operands[1].Radius, half[0], half[1], tolerance)
	if err != nil {
		return zero, err
	}
	x, z := axes[0], axes[1]
	y := z.Cross(x)
	s, err := local.Solid.Transformed(geom.Affine{{x.X, y.X, z.X, center.X}, {x.Y, y.Y, z.Y, center.Y}, {x.Z, y.Z, z.Z, center.Z}})
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out := FiniteCylinderPairUnionResult{Solid: s, WallFaces: local.WallFaces, LowerCapFaces: local.LowerCapFaces, UpperCapFaces: local.UpperCapFaces}
	if a.Radius < b.Radius {
		for _, groups := range []*[2][]int{&out.WallFaces, &out.LowerCapFaces, &out.UpperCapFaces} {
			groups[0], groups[1] = groups[1], groups[0]
		}
	}
	return out, nil
}
