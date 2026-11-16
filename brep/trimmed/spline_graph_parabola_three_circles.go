package trimmed

import (
	"context"

	"plainsolid/geom"
)

type parabolaThreeCirclesPartition struct {
	crossing parabolaTwoCrossingCirclesPartition
	proof    *geom.SplineGraphParabolaThreeCircles
	regions  [7]openGraphRegion
}

func proveParabolaThreeCirclesPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (p parabolaThreeCirclesPartition, err error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return p, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return p, err
	}
	p.proof, err = geom.DiscoverSplineGraphParabolaThreeCircles(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return p, err
	}
	p.crossing, err = buildParabolaTwoCrossingCirclesPartition(&p.proof.Crossing)
	if err != nil {
		return p, err
	}
	disco := p.proof.Circles[p.proof.SeparatedCircle]
	outer, inner, err := exactGraphCircleRings(disco.Center, disco.RadiusSquared)
	if err != nil {
		return p, err
	}
	p.crossing.regions[p.proof.SeparatedRegion].holes = [][]openSectionPiece{inner}
	copy(p.regions[:6], p.crossing.regions[:])
	p.regions[6] = openGraphRegion{loop: outer, sign: p.proof.RegionSides[6]}
	return p, nil
}
