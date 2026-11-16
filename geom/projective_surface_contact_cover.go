package geom

import (
	"context"
	"errors"
)

// ProjectiveSurfaceContactCoverOptions bounds common height construction
// and local contact isolation. MaxExactOperations is shared across the
// whole query, defaulting to sixteen million exact operations.
type ProjectiveSurfaceContactCoverOptions struct {
	Atlas              GeneralSurfaceHeightOptions
	Arcs               SurfacePlaneArcOptions
	MaxExactOperations int
}

// ProjectiveSurfaceContactCover covers a positive-area contact projection
// with local exact arcs, tangent points and coincident cells. Isolations
// correspond to Atlas.Cells. Each uses that residual cell's unit square;
// Common and OriginalCells map the answer back to both original owners.
// Complete means all local cells are covered, without claiming global joins.
type ProjectiveSurfaceContactCover struct {
	Atlas           ProjectiveSurfaceHeightDifference
	Isolations      []SurfacePlaneArcs
	IncompleteCells []int
	Complete        bool
	ExactOperations int
}

// BoundProjectiveSurfaceContacts discovers the complete local zero set of
// the original graph height differences wherever the existing plane-arc
// proof succeeds. Resource and unsupported singular cases retain both the
// exact atlas and the list of incomplete cells.
func BoundProjectiveSurfaceContacts(ctx context.Context, a, b *SurfacePatch, axes [2]int,
	o ProjectiveSurfaceContactCoverOptions) (r ProjectiveSurfaceContactCover, err error) {
	if ctx == nil || o.MaxExactOperations < 0 || o.Atlas.MaxOperations < 0 || o.Arcs.MaxOperations < 0 {
		return r, ErrInvalid
	}
	if o.MaxExactOperations == 0 {
		o.MaxExactOperations = 16_000_000
	}
	if o.Atlas.MaxOperations == 0 {
		o.Atlas.MaxOperations = 8_000_000
	}
	if o.Arcs.MaxOperations == 0 {
		o.Arcs.MaxOperations = 2_000_000
	}
	o.Atlas.MaxOperations = min(o.Atlas.MaxOperations, o.MaxExactOperations)
	r.Atlas, err = BuildProjectiveSurfaceHeightDifference(ctx, a, b, axes, o.Atlas)
	r.ExactOperations = r.Atlas.ExactOperations
	if err != nil {
		return r, err
	}
	r.Isolations = make([]SurfacePlaneArcs, len(r.Atlas.Cells))
	defer func() {
		for i, cell := range r.Isolations {
			if !cell.Complete {
				r.IncompleteCells = append(r.IncompleteCells, i)
			}
		}
	}()
	plane, err := NewPlane(Vec3{}, Vec3{Z: 1})
	if err != nil {
		return r, err
	}
	all := true
	for i, cell := range r.Atlas.Cells {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		left := o.MaxExactOperations - r.ExactOperations
		if left < 1 {
			return r, ErrConvergence
		}
		arc := o.Arcs
		arc.MaxOperations = min(left, arc.MaxOperations)
		r.Isolations[i], err = CertifySurfacePlaneArcsContext(ctx, cell.Difference, plane, arc)
		r.ExactOperations += r.Isolations[i].ExactOperations
		if err != nil && !errors.Is(err, ErrConvergence) {
			return r, err
		}
		all = all && err == nil && r.Isolations[i].Complete
	}
	if !all {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
