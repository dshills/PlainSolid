package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicConvexTrimHole retains the original clockwise affine hole and
// its directed UV/model owners. Bounds are in the original source chart.
type PeriodicConvexTrimHole PeriodicTrimHole

// PeriodicConvexTrimmedFaceWithHole carries a strictly convex outer
// quadrilateral with one strictly interior clockwise quadrilateral.
// Patch evaluates the enclosing rectangle, and Hole identifies the
// removed region of the outer loop.
type PeriodicConvexTrimmedFaceWithHole struct {
	PeriodicConvexTrimmedFace
	Hole PeriodicConvexTrimHole
}

// ReadPeriodicConvexTrimmedFaceWithHole proves one original strictly
// nested inner loop on the same single-cell chart as the outer loop.
// Exact composition proves every directed source curve, and strict
// affine half-planes exclude contact and crossing.
func ReadPeriodicConvexTrimmedFaceWithHole(ctx context.Context, r io.Reader, l Limits) (PeriodicConvexTrimmedFaceWithHole, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 4, periodicTrimAnyAxes, []int{4})
	if err != nil {
		return PeriodicConvexTrimmedFaceWithHole{}, err
	}
	return provePeriodicConvexTrimmedFaceWithHole(ctx, source)
}

func provePeriodicConvexTrimmedFaceWithHole(ctx context.Context, source periodicTrimSource) (PeriodicConvexTrimmedFaceWithHole, error) {
	var zero PeriodicConvexTrimmedFaceWithHole
	outer, err := provePeriodicConvexOuter(ctx, &source)
	if err != nil {
		return zero, err
	}

	hole, _, err := provePeriodicConvexHole(ctx, source.graph, outer, source.holes[0], source.remaining)
	if err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConvexTrimmedFaceWithHole{PeriodicConvexTrimmedFace: outer, Hole: hole}, nil
}

func provePeriodicConvexHole(ctx context.Context, graph periodicTrimGraph, outer PeriodicConvexTrimmedFace,
	owner periodicTrimLoopSource, remaining int) (PeriodicConvexTrimHole, int, error) {
	var zero PeriodicConvexTrimHole
	var uv [4]periodicTrimUVEdge
	bounds, err := readPeriodicTrimUVEdges(graph, owner.uv, uv[:])
	if err != nil {
		return zero, 0, err
	}
	inner, err := provePeriodicConvexLoop(uv, -1)
	if err != nil {
		return zero, 0, fmt.Errorf("iges: convex trim hole #%d: %w", owner.contourID, err)
	}
	edges, remaining, err := provePeriodicAffineEdges(ctx, graph, uv, owner.model, outer.Patch, outer.Bounds, remaining)
	if err != nil {
		return zero, 0, fmt.Errorf("iges: convex trim hole #%d: %w", owner.contourID, err)
	}
	if !periodicUVStrictlyContains(periodicAffineWireVertices(outer.Edges), inner) {
		return zero, 0, fmt.Errorf("iges: convex trim hole #%d is not strictly contained in its outer loop: %w", owner.contourID, geom.ErrUnrepresentable)
	}
	return PeriodicConvexTrimHole{ContourID: owner.contourID, UVCompositeID: owner.uvID, ModelCompositeID: owner.modelID,
		Bounds: bounds, Edges: edges}, remaining, nil
}

func periodicAffineWireVertices(wire [4]PeriodicTrimEdge) [4][2]*big.Rat {
	var points [4][2]*big.Rat
	for i, edge := range wire {
		points[i] = edge.UVStart
	}
	return points
}

func periodicUVStrictlyContains(outer, inner [4][2]*big.Rat) bool {
	for i, a := range outer {
		b := outer[(i+1)%len(outer)]
		for _, q := range inner {
			if periodicUVTurn(a, b, q) <= 0 {
				return false
			}
		}
	}
	return true
}
