package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicConcavePolygonTrimmedFaceWithHole retains one convex clockwise
// hole in the original concave outer contour. Both contours carry their
// exact directed UV and model curve pairs.
type PeriodicConcavePolygonTrimmedFaceWithHole struct {
	PeriodicConcavePolygonTrimmedFace
	Hole PeriodicConvexPolygonTrimLoop
}

// ReadPeriodicConcavePolygonTrimmedFaceWithHole proves one original
// strictly contained convex affine hole in the certified concave face.
// The hole must have at least one nonisoparametric edge.
func ReadPeriodicConcavePolygonTrimmedFaceWithHole(ctx context.Context, r io.Reader, l Limits) (PeriodicConcavePolygonTrimmedFaceWithHole, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 0, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicConcavePolygonTrimmedFaceWithHole{}, err
	}
	return provePeriodicConcavePolygonTrimmedFaceWithHole(ctx, source)
}

func provePeriodicConcavePolygonTrimmedFaceWithHole(ctx context.Context, source periodicTrimSource) (PeriodicConcavePolygonTrimmedFaceWithHole, error) {
	var zero PeriodicConcavePolygonTrimmedFaceWithHole
	if len(source.holes) != 1 {
		return zero, fmt.Errorf("iges: concave periodic trim requires exactly one hole: %w", geom.ErrUnrepresentable)
	}
	outer, err := provePeriodicConcaveOuter(ctx, &source)
	if err != nil {
		return zero, err
	}
	hole, _, err := provePeriodicConvexInnerWire(ctx, source.graph, outer.Patch, outer.Bounds, source.holes[0], source.remaining)
	if err != nil {
		return zero, err
	}
	if err := provePeriodicConcaveHoleContainment(periodicAffineWireVertices(outer.Edges), periodicAffineWireVertices(hole.Edges)); err != nil {
		return zero, fmt.Errorf("iges: concave trim hole #%d: %w", hole.ContourID, err)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConcavePolygonTrimmedFaceWithHole{PeriodicConcavePolygonTrimmedFace: outer, Hole: hole}, nil
}

func provePeriodicConcaveHoleContainment(outer, inner [][2]*big.Rat) error {
	for i, a := range outer {
		b := outer[(i+1)%len(outer)]
		for j, c := range inner {
			d := inner[(j+1)%len(inner)]
			if periodicUVSegmentsMeet(a, b, c, d) {
				return fmt.Errorf("iges: outer edge %d and hole edge %d meet: %w", i+1, j+1, geom.ErrUnrepresentable)
			}
		}
	}
	// With disjoint simple boundaries, the hole's connected boundary
	// lies entirely inside or entirely outside the outer contour. If
	// it is inside, the outer exterior can still reach infinity without
	// crossing the hole, so the hole's interior is contained as well.
	if periodicUVWindingNumber(outer, inner[0]) != 1 {
		return fmt.Errorf("iges: hole lies outside the actual concave UV domain: %w", geom.ErrUnrepresentable)
	}
	return nil
}

// periodicUVWindingNumber counts signed crossings of a horizontal ray.
// Half-open vertical intervals give a consistent result at vertex
// heights. The witness must be disjoint from the polygon boundary.
func periodicUVWindingNumber(wire [][2]*big.Rat, witness [2]*big.Rat) int {
	winding := 0
	for i, a := range wire {
		b := wire[(i+1)%len(wire)]
		start, end := a[1].Cmp(witness[1]), b[1].Cmp(witness[1])
		if start <= 0 && end > 0 && periodicUVTurn(a, b, witness) > 0 {
			winding++
		}
		if start > 0 && end <= 0 && periodicUVTurn(a, b, witness) < 0 {
			winding--
		}
	}
	return winding
}
