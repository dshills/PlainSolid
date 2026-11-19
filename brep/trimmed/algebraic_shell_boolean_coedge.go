//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellBooleanCoedge retains the exact original chart of one result
// edge use. Exactly one of Source and Section is set. Both chart evaluators
// use the canonical model-edge direction; Reverse tells how to traverse
// them in the stored UV loop, and PhysicalReverse includes face orientation.
type AlgebraicShellBooleanCoedge struct {
	Edge, Operand, OriginalFace, Fragment int
	Reverse, PhysicalReverse              bool
	Source                                *AlgebraicSubdivisionChart
	Section                               *intersect.TubeTrim
}

// Coedge retrieves the complete restricted p-curve of a selected Boolean
// incidence. Loop zero is the exterior. Positive loop numbers select holes.
// It ignores edits to the public topology diagnostics and never rebuilds a
// generated p-curve from sampled model vertices.
func (s AlgebraicShellBoolean) Coedge(ctx context.Context, face, loop, use int, o AlgebraicSeamOptions) (AlgebraicShellBooleanCoedge, error) {
	var empty AlgebraicShellBooleanCoedge
	if ctx == nil || s.source == nil || face < 0 || face >= len(s.faces) || loop < 0 || use < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	f := s.faces[face]
	ring := f.Outer
	if loop > 0 {
		if loop > len(f.Holes) {
			return empty, geom.ErrInvalid
		}
		ring = f.Holes[loop-1]
	}
	if use >= len(ring) {
		return empty, geom.ErrInvalid
	}
	u := ring[use]
	e := s.index[u.Edge]
	out := AlgebraicShellBooleanCoedge{Edge: u.Edge, Operand: f.Operand, OriginalFace: f.Original, Fragment: f.Fragment,
		Reverse: u.Reversed, PhysicalReverse: u.Reversed != f.Reversed}
	if e.SectionPiece >= 0 {
		seg, pair, err := s.SectionSegment(u.Edge)
		if err != nil {
			return empty, err
		}
		if pair[f.Operand] != f.Original {
			return empty, geom.ErrConvergence
		}
		trim := [2]intersect.TubeTrim{seg.TrimA(), seg.TrimB()}[f.Operand]
		out.Section = &trim
		return out, nil
	}
	if e.OriginalSubdivisionEdge < 0 || e.Operand != f.Operand {
		return empty, geom.ErrConvergence
	}
	chart, err := s.parts[f.Operand].Subdivision.CoedgeChart(ctx, e.OriginalSubdivisionEdge, f.Original, o)
	if err != nil {
		return empty, err
	}
	out.Source = &chart
	return out, nil
}
