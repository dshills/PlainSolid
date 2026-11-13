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
	windows, placement, err := finiteSignedPerpendicularWindowsFrame(ctx, operands, tolerance)
	if err != nil {
		return zero, err
	}
	local, err := UnequalPerpendicularCylinderUnionWindows(ctx, geom.Vec3{}, operands[0].Radius, operands[1].Radius, windows[0], windows[1], tolerance)
	if err != nil {
		return zero, err
	}
	s, err := local.Solid.Transformed(placement)
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

func finiteUnequalPerpendicularDifference(ctx context.Context, stock, cutter geom.RawCylinder, tolerance float64) (FiniteCylinderPairDifferenceResult, error) {
	var out FiniteCylinderPairDifferenceResult
	windows, placement, err := finiteSignedPerpendicularWindowsFrame(ctx, [2]geom.RawCylinder{stock, cutter}, tolerance)
	if err != nil {
		return out, err
	}
	local, err := UnequalPerpendicularCylinderDifferenceWindows(ctx, geom.Vec3{}, stock.Radius, cutter.Radius, windows[0], windows[1], tolerance)
	if err != nil {
		return out, err
	}
	s, err := local.Solid.Transformed(placement)
	if err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Solids[0], out.StockWallFaces[0], out.CutterWallFaces[0] = s, local.StockWallFaces, local.CutterWallFaces
	out.StockCapFaces[0] = append(local.LowerCapFaces, local.UpperCapFaces...)
	return out, nil
}

// finiteSignedPerpendicularWindowsFrame places the first cylinder on local
// X and the second on local Z. It preserves both raw axis directions and
// checks that the physical caps survive the conversion to this frame exactly.
func finiteSignedPerpendicularWindowsFrame(ctx context.Context, operands [2]geom.RawCylinder, tolerance float64) (windows [2]Interval, placement geom.Affine, err error) {
	var axes [2]geom.Vec3
	var scale [2]float64
	for i, c := range operands {
		axes[i], scale[i] = coordinateCylinderAxis(c.Axis)
	}
	if scale[0] == 0 || scale[1] == 0 || axes[0].Dot(axes[1]) != 0 {
		return windows, placement, geom.ErrDomain
	}
	center, caps, err := finiteCylinderCrossingFrame(ctx, operands[0], operands[1], tolerance)
	if err != nil {
		return windows, placement, err
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for i, window := range caps {
		if window[0] >= 0 || window[1] <= 0 {
			return windows, placement, geom.ErrDomain
		}
		ends := [2]float64{}
		for j, t := range window {
			distance := new(big.Rat).Mul(q(t), q(scale[i]))
			ends[j], err = geom.RepresentedExactScalar(distance)
			if err != nil {
				return [2]Interval{}, placement, err
			}
			cap := new(big.Rat).Add(q(center.Dot(axes[i])), distance)
			if _, err = geom.RepresentedExactScalar(cap); err != nil {
				return [2]Interval{}, placement, err
			}
		}
		windows[i] = Interval{ends[0], ends[1]}
	}
	x, z := axes[0], axes[1]
	y := z.Cross(x)
	return windows, geom.Affine{{x.X, y.X, z.X, center.X}, {x.Y, y.Y, z.Y, center.Y}, {x.Z, y.Z, z.Z, center.Z}}, nil
}
