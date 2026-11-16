package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func proveParabolaCircleObliqueSection(ctx context.Context, sources [3]*geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (out parabolaCircleTangentSection, err error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return out, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return out, err
	}
	p, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return out, err
	}
	open, err := splitParabolaCircleTangentOpenRegions(p.ParabolaControls, p.Parameters, axis)
	if err != nil {
		return out, err
	}
	radial := [2]*big.Rat{new(big.Rat).Sub(p.Parameters[0], p.CircleCenter[0]), new(big.Rat).Sub(p.Parameters[1], p.CircleCenter[1])}
	rotated := [2]*big.Rat{new(big.Rat).Neg(radial[1]), new(big.Rat).Set(radial[0])}
	// Starting the circle's exact rational quadrants at the tangent
	// radius preserves the true point and the original model-side axes.
	// The rotated vector is perpendicular with the same squared length.
	ring, err := ellipseGraphAffineLoop(p.CircleCenter, [2][2]*big.Rat{radial, rotated})
	if err != nil {
		return out, err
	}
	exact, _, err := exactGraphConicRings(ring)
	if err != nil {
		return out, err
	}
	node, err := exactGraphPartitionUV(p.Parameters)
	if err != nil {
		return out, err
	}
	if exact[0].a != node || exact[3].b != node {
		return out, geom.ErrConvergence
	}
	out = parabolaCircleTangentSection{
		axis: axis, diskRegion: p.DiskRegion, parameters: p.Parameters, circleCenter: p.CircleCenter,
		point: p.Point, slope: p.Slope, second: [2]*big.Rat{p.ParabolaSecondDerivative, p.CircleSecondDerivative},
		regions: [3]openGraphRegion{open[0], open[1], {loop: exact}},
	}
	for i, sign := range p.RegionSides {
		out.regions[i].sign = sign
	}
	return out, nil
}
