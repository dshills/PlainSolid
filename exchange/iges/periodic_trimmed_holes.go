package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/geom"
)

// PeriodicTrimHole retains the original clockwise rectangular inner wire and
// its exact directed UV/model pairs in the original file coordinate chart.
type PeriodicTrimHole struct {
	ContourID, UVCompositeID, ModelCompositeID int
	Bounds                                     [2]geom.ExactParameterInterval
	Edges                                      [4]PeriodicTrimEdge
}

// PeriodicTrimmedFaceWithHoles carries the source outer face and one or two
// pairwise disjoint inner wires. The shared Patch covers the full outer
// rectangle. Holes describe the regions removed from its material.
type PeriodicTrimmedFaceWithHoles struct {
	PeriodicTrimmedFace
	Holes []PeriodicTrimHole
}

// ReadPeriodicTrimmedFaceWithHoles proves a type-144 face with an explicit
// CCW rectangular outer contour and one or two strictly interior clockwise
// rectangular holes in the same original U-periodic type-128 knot cell.
// The original type-110 and type-126 owners of every edge must agree over
// the entire directed edge. One projection proves injectivity of the whole
// outer rectangle before the result is published.
func ReadPeriodicTrimmedFaceWithHoles(ctx context.Context, r io.Reader, l Limits) (PeriodicTrimmedFaceWithHoles, error) {
	source, err := readPeriodicTrimSourceWithHoles(ctx, r, l, 4, 2)
	if err != nil {
		return PeriodicTrimmedFaceWithHoles{}, err
	}
	return provePeriodicTrimmedFaceWithHoles(ctx, source)
}

func provePeriodicTrimmedFaceWithHoles(ctx context.Context, source periodicTrimSource) (PeriodicTrimmedFaceWithHoles, error) {
	var zero PeriodicTrimmedFaceWithHoles
	m := source.face
	cell, err := m.proveOriginalKnotCell(ctx)
	if err != nil {
		return zero, err
	}
	remaining, err := m.proveEdges(ctx, source.graph, cell, source.uv, source.model, source.remaining)
	if err != nil {
		return zero, err
	}
	holes := make([]PeriodicTrimHole, len(source.holes))
	for i, owner := range source.holes {
		uv, bounds, err := readPeriodicTrimUVRectangle(source.graph, owner.uv)
		if err != nil {
			return zero, err
		}
		cells, _, err := m.Support.ExactBezierCells(ctx, bounds[0], bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 1})
		if err != nil {
			return zero, fmt.Errorf("iges: periodic hole #%d is not an exact rectangle in one knot cell: %w", owner.contourID, err)
		}
		if len(cells) != 1 {
			return zero, geom.ErrUnrepresentable
		}
		edges, left, err := provePeriodicTrimRectangle(ctx, source.graph, cells[0], bounds, uv, owner.model, -1, remaining)
		if err != nil {
			return zero, fmt.Errorf("iges: periodic hole #%d: %w", owner.contourID, err)
		}
		remaining = left
		for axis, interval := range bounds {
			if interval.Lower.Cmp(m.Bounds[axis].Lower) <= 0 || interval.Upper.Cmp(m.Bounds[axis].Upper) >= 0 {
				return zero, fmt.Errorf("iges: periodic hole #%d is not strictly contained in its outer loop: %w", owner.contourID, geom.ErrUnrepresentable)
			}
		}
		for _, prior := range holes[:i] {
			if !disjointPeriodicTrimRectangles(bounds, prior.Bounds) {
				return zero, fmt.Errorf("iges: periodic holes #%d and #%d overlap or meet: %w", prior.ContourID, owner.contourID, geom.ErrUnrepresentable)
			}
		}
		holes[i] = PeriodicTrimHole{ContourID: owner.contourID, UVCompositeID: owner.uvID, ModelCompositeID: owner.modelID,
			Bounds: bounds, Edges: edges}
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	if err := m.proveInjectivePatch(ctx, cell); err != nil {
		return zero, err
	}
	return PeriodicTrimmedFaceWithHoles{PeriodicTrimmedFace: m, Holes: holes}, nil
}

func disjointPeriodicTrimRectangles(a, b [2]geom.ExactParameterInterval) bool {
	for i := range a {
		if a[i].Upper.Cmp(b[i].Lower) < 0 || b[i].Upper.Cmp(a[i].Lower) < 0 {
			return true
		}
	}
	return false
}
