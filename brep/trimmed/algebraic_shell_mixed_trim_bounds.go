package trimmed

import (
	"context"

	"plainsolid/geom"
)

// MixedAlgebraicTrimBounds encloses the retained solid using the same
// original face classification used for its material properties. The
// enclosure includes every unresolved trim cell, may be wider than the
// true extent, and does not depend on display tessellation.
type MixedAlgebraicTrimBounds struct {
	Box                          geom.Box
	Empty                        bool
	VisitedCells, AmbiguousCells int
}

// BoundTrimmedGeometry intersects conservative Boolean operand bounds
// with the union of outward hulls of retained or uncertain original
// face cells. It may exclude substantial discarded support space. An
// exhausted cell budget returns no enclosure.
func (s *MixedAlgebraicShellBoolean) BoundTrimmedGeometry(ctx context.Context,
	opts AlgebraicShellMeasureOptions) (MixedAlgebraicTrimBounds, error) {
	var zero MixedAlgebraicTrimBounds
	if ctx == nil || s == nil || s.section == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	box, empty, err := s.boundary.Bounds()
	if err != nil || empty {
		return MixedAlgebraicTrimBounds{Empty: empty}, err
	}
	var out MixedAlgebraicTrimBounds
	found := false
	counts, err := s.boundary.visitBooleanSurfaceCells(ctx, opts, func(p *geom.SurfacePatch, _ geom.Vec3, _, _ bool, _, _ int) error {
		b, err := p.Bounds()
		if err != nil {
			return err
		}
		if !found {
			out.Box, found = b, true
		} else {
			out.Box, err = out.Box.Union(b)
		}
		return err
	})
	if err != nil {
		return zero, err
	}
	if !found {
		return zero, geom.ErrConvergence
	}
	var ok bool
	out.Box, ok = out.Box.Intersect(box)
	if !ok {
		return zero, geom.ErrConvergence
	}
	out.VisitedCells, out.AmbiguousCells = counts.VisitedCells, counts.AmbiguousCells
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
