package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/geom"
)

// PeriodicConcavePolygonTrimmedFaceWithHoles retains two convex
// clockwise affine holes in the original order of the type-144 face.
type PeriodicConcavePolygonTrimmedFaceWithHoles struct {
	PeriodicConcavePolygonTrimmedFace
	Holes [2]PeriodicConvexPolygonTrimLoop
}

// ReadPeriodicConcavePolygonTrimmedFaceWithHoles proves two disjoint
// convex holes strictly inside the original concave loop. Both holes
// require a nonisoparametric edge and share the certified knot cell.
func ReadPeriodicConcavePolygonTrimmedFaceWithHoles(ctx context.Context, r io.Reader, l Limits) (PeriodicConcavePolygonTrimmedFaceWithHoles, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 0, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicConcavePolygonTrimmedFaceWithHoles{}, err
	}
	return provePeriodicConcavePolygonTrimmedFaceWithHoles(ctx, source)
}

func provePeriodicConcavePolygonTrimmedFaceWithHoles(ctx context.Context, source periodicTrimSource) (PeriodicConcavePolygonTrimmedFaceWithHoles, error) {
	var zero PeriodicConcavePolygonTrimmedFaceWithHoles
	if len(source.holes) != 2 {
		return zero, fmt.Errorf("iges: concave periodic trim requires exactly two holes: %w", geom.ErrUnrepresentable)
	}
	outer, err := provePeriodicConcaveOuter(ctx, &source)
	if err != nil {
		return zero, err
	}
	var holes [2]PeriodicConvexPolygonTrimLoop
	for i, owner := range source.holes {
		holes[i], source.remaining, err = provePeriodicConcaveHole(ctx, source.graph, outer, owner, source.remaining)
		if err != nil {
			return zero, err
		}
	}
	if !periodicConvexWiresStrictlyDisjoint(periodicAffineWireVertices(holes[0].Edges), periodicAffineWireVertices(holes[1].Edges)) {
		return zero, fmt.Errorf("iges: concave trim holes #%d and #%d overlap or meet: %w", holes[0].ContourID, holes[1].ContourID, geom.ErrUnrepresentable)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConcavePolygonTrimmedFaceWithHoles{PeriodicConcavePolygonTrimmedFace: outer, Holes: holes}, nil
}
