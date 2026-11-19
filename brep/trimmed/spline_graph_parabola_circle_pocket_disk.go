//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
)

type parabolaCirclePocketDiskPartition struct {
	proof    *geom.SplineGraphParabolaCirclePocketDisk
	crossing parabolaCircleFourCrossingPartition
	regions  [7]openGraphRegion
}

func proveParabolaCirclePocketDiskPartition(ctx context.Context, src [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (p parabolaCirclePocketDiskPartition, err error) {
	if ctx == nil || src[0] == nil || src[1] == nil || src[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return p, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, src[0], src[1], src[2], o.Stock)
	if err != nil {
		return p, err
	}
	p.proof, err = geom.DiscoverSplineGraphParabolaCirclePocketDiskWithVolume(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return p, err
	}
	p.crossing, err = buildParabolaCircleFourCrossingPartition(&p.proof.Crossing)
	if err != nil {
		return p, err
	}
	disk := p.proof.Circles[p.proof.PocketCircle]
	outer, inner, err := exactGraphCircleRings(disk.Center, disk.RadiusSquared)
	if err != nil {
		return p, err
	}
	p.crossing.regions[2].holes = [][]openSectionPiece{inner}
	copy(p.regions[:], p.crossing.regions[:])
	p.regions[6] = openGraphRegion{loop: outer, sign: p.proof.RegionSides[6]}
	return p, nil
}
