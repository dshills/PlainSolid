package trimmed

import (
	"context"

	"plainsolid/geom"
)

type parabolaMixedCirclesPartition struct {
	crossing parabolaCircleCrossingsPartition
	proof    *geom.SplineGraphParabolaMixedCircles
	regions  [5]openGraphRegion
}

func proveParabolaMixedCirclesPartition(ctx context.Context, sources [3]*geom.BSplineSurface, axis int, tolerance float64,
	o ParabolaCircleGraphOptions) (p parabolaMixedCirclesPartition, err error) {
	if ctx == nil || sources[0] == nil || sources[1] == nil || sources[2] == nil || axis < 0 || axis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return p, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, sources[0], sources[1], sources[2], o.Stock)
	if err != nil {
		return p, err
	}
	p.proof, err = geom.DiscoverSplineGraphParabolaMixedCircles(ctx, patches[0], patches[1], axis, o.Proof)
	if err != nil {
		return p, err
	}
	p.crossing, err = buildParabolaCircleCrossingsPartition(&p.proof.Crossing)
	if err != nil {
		return p, err
	}
	separate := p.proof.Circles[1-p.proof.CrossingCircle]
	circle, hole, err := exactGraphCircleRings(separate.Center, separate.RadiusSquared)
	if err != nil {
		return p, err
	}
	side := p.proof.SeparatedRegion
	p.crossing.regions[side].holes = [][]openSectionPiece{hole}
	copy(p.regions[:4], p.crossing.regions[:])
	p.regions[4] = openGraphRegion{loop: circle, sign: p.proof.RegionSides[4]}
	return p, nil
}

func bindParabolaMixedCircleRing(s Solid, edges []int, p parabolaMixedCirclesPartition) (vertices [4]int, err error) {
	if len(edges) != len(vertices) {
		return vertices, geom.ErrConvergence
	}
	for i, v := range p.proof.SeparatedExtrema {
		uv, err := exactGraphPartitionUV(v.Parameters)
		if err != nil {
			return vertices, err
		}
		if p.regions[4].loop[i].a != uv {
			return vertices, geom.ErrConvergence
		}
		vertex, err := exactParabolaCircleCrossingVertex(s, v.Point)
		if err != nil {
			return vertices, err
		}
		if !matchesParabolaCircleCrossingRay(s, edges[i], vertex, v.Tangents[0]) ||
			!matchesParabolaCircleCrossingRay(s, edges[i], vertex, v.Tangents[1]) {
			return vertices, geom.ErrConvergence
		}
		vertices[i] = vertex
	}
	return vertices, nil
}
