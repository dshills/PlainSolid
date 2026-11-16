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

	owner := source.holes[0]
	var uv [4]periodicTrimUVEdge
	bounds, err := readPeriodicTrimUVEdges(source.graph, owner.uv, uv[:])
	if err != nil {
		return zero, err
	}
	inner, err := provePeriodicConvexLoop(uv, -1)
	if err != nil {
		return zero, fmt.Errorf("iges: convex trim hole #%d: %w", owner.contourID, err)
	}
	edges, _, err := provePeriodicAffineEdges(ctx, source.graph, uv, owner.model, outer.Patch, outer.Bounds, source.remaining)
	if err != nil {
		return zero, fmt.Errorf("iges: convex trim hole #%d: %w", owner.contourID, err)
	}
	var boundary [4][2]*big.Rat
	for i, edge := range outer.Edges {
		boundary[i] = edge.UVStart
	}
	if !periodicUVStrictlyContains(boundary, inner) {
		return zero, fmt.Errorf("iges: convex trim hole #%d is not strictly contained in its outer loop: %w", owner.contourID, geom.ErrUnrepresentable)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConvexTrimmedFaceWithHole{
		PeriodicConvexTrimmedFace: outer,
		Hole: PeriodicConvexTrimHole{ContourID: owner.contourID, UVCompositeID: owner.uvID, ModelCompositeID: owner.modelID,
			Bounds: bounds, Edges: edges},
	}, nil
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
