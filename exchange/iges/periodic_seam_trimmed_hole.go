package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/geom"
)

// PeriodicSeamTrimHole retains a clockwise inner contour split at the same
// original U seam as its outer contour. Edges use the original lifted UV
// chart, and Cells contain the two removed rectangles in that chart.
type PeriodicSeamTrimHole struct {
	ContourID, UVCompositeID, ModelCompositeID int
	Bounds                                     [2]geom.ExactParameterInterval
	Cells                                      [2]PeriodicSeamTrimCell
	Edges                                      [6]PeriodicTrimEdge
}

// PeriodicSeamTrimmedFaceWithHole carries both sides of the outer lift and
// an original strictly interior hole split at the same U seam. The outer
// cells cover the material together with the removed region.
type PeriodicSeamTrimmedFaceWithHole struct {
	PeriodicSeamTrimmedFace
	Hole PeriodicSeamTrimHole
}

// ReadPeriodicSeamTrimmedFaceWithHole recognizes one type-144 face whose
// rectangular outer and inner loops both cross the original upper U seam.
// Each loop must have six complete paired edges in the same chart and the
// original inner winding must be opposite to the outer winding.
func ReadPeriodicSeamTrimmedFaceWithHole(ctx context.Context, r io.Reader, l Limits) (PeriodicSeamTrimmedFaceWithHole, error) {
	var zero PeriodicSeamTrimmedFaceWithHole
	source, err := readPeriodicTrimSourceWithHoles(ctx, r, l, 6, 6, 1)
	if err != nil {
		return zero, err
	}
	outer, err := provePeriodicSeamTrimmedOuter(ctx, &source)
	if err != nil {
		return zero, err
	}
	owner := source.holes[0]
	var uv [6]periodicTrimUVEdge
	bounds, err := readPeriodicTrimUVEdges(source.graph, owner.uv, uv[:])
	if err != nil {
		return zero, err
	}
	inner := PeriodicSeamTrimmedFace{Support: outer.Support, Bounds: bounds, SeamU: outer.SeamU}
	cells, err := inner.proveSeamCells(ctx)
	if err != nil {
		return zero, fmt.Errorf("iges: periodic hole #%d: %w", owner.contourID, err)
	}
	edges, _, err := proveLiftedTrimEdges(ctx, source.graph, bounds, outer.SeamU, uv, owner.model, cells, -1, source.remaining)
	if err != nil {
		return zero, fmt.Errorf("iges: periodic hole #%d: %w", owner.contourID, err)
	}
	if err := provePeriodicTrimHoleContainment(owner.contourID, bounds, outer.Bounds); err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicSeamTrimmedFaceWithHole{
		PeriodicSeamTrimmedFace: outer,
		Hole: PeriodicSeamTrimHole{
			ContourID: owner.contourID, UVCompositeID: owner.uvID, ModelCompositeID: owner.modelID,
			Bounds: bounds, Cells: inner.Cells, Edges: edges,
		},
	}, nil
}
