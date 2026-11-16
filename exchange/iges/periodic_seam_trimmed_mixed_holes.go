package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/geom"
)

// PeriodicSeamTrimmedFaceWithMixedHoles retains two original clockwise
// holes in the same lifted outer chart. SeamHole crosses the upper U
// seam. CellHole lies in the original knot span of Cells[CellHoleSpan].
// HoleContourIDs preserves the original type-144 inner contour order.
type PeriodicSeamTrimmedFaceWithMixedHoles struct {
	PeriodicSeamTrimmedFace
	SeamHole       PeriodicSeamTrimHole
	CellHole       PeriodicTrimHole
	CellHoleSpan   int
	HoleContourIDs [2]int
}

// ReadPeriodicSeamTrimmedFaceWithMixedHoles proves an upper-U-seam
// rectangle with one seam-crossing hole and one single-span hole. Both
// original inner contours must be strictly interior and disjoint within
// the same injective lifted chart, with complete directed UV/model pairs.
func ReadPeriodicSeamTrimmedFaceWithMixedHoles(ctx context.Context, r io.Reader, l Limits) (PeriodicSeamTrimmedFaceWithMixedHoles, error) {
	source, err := readPeriodicTrimSource(ctx, r, l, 6, []int{4, 6})
	if err != nil {
		return PeriodicSeamTrimmedFaceWithMixedHoles{}, err
	}
	return provePeriodicSeamTrimmedFaceWithMixedHoles(ctx, source)
}

func provePeriodicSeamTrimmedFaceWithMixedHoles(ctx context.Context, source periodicTrimSource) (PeriodicSeamTrimmedFaceWithMixedHoles, error) {
	var zero PeriodicSeamTrimmedFaceWithMixedHoles
	outer, err := provePeriodicSeamTrimmedOuter(ctx, &source)
	if err != nil {
		return zero, err
	}

	m := PeriodicSeamTrimmedFaceWithMixedHoles{PeriodicSeamTrimmedFace: outer}
	remaining := source.remaining
	for i, owner := range source.holes {
		m.HoleContourIDs[i] = owner.contourID
		switch len(owner.uv) {
		case 4:
			m.CellHole, remaining, err = provePeriodicTrimCellHole(ctx, source.graph, m.Support, owner, remaining)
		case 6:
			m.SeamHole, remaining, err = provePeriodicSeamHole(ctx, source.graph, outer, owner, remaining)
		}
		if err != nil {
			return zero, err
		}
	}
	if err := provePeriodicTrimHoleContainment(m.CellHole.ContourID, m.CellHole.Bounds, m.Bounds); err != nil {
		return zero, err
	}
	u := m.CellHole.Bounds[0]
	switch {
	case u.Upper.Cmp(m.SeamU) < 0:
		m.CellHoleSpan = 0
	case u.Lower.Cmp(m.SeamU) > 0:
		m.CellHoleSpan = 1
	default:
		return zero, fmt.Errorf("iges: single-span hole meets the original periodic seam: %w", geom.ErrUnrepresentable)
	}
	if !disjointPeriodicTrimRectangles(m.SeamHole.Bounds, m.CellHole.Bounds) {
		return zero, fmt.Errorf("iges: periodic holes #%d and #%d overlap or meet: %w", m.SeamHole.ContourID, m.CellHole.ContourID, geom.ErrUnrepresentable)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return m, nil
}
