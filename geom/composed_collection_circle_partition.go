package geom

import (
	"context"
	"math/big"
)

// CollectionCircleContact retains both germs at a shared knot. Crossing/Touching
// use only strict signs on both sides, not any assumption of tangent continuity.
type CollectionCircleContact struct {
	Event              *ExactComposedCurveEvent
	Cells              []int
	Local              []PlanarCircleContact
	SideSigns          [2]int
	Crossing, Touching bool
}

type CollectionCircleInterval struct {
	PlanarCircleInterval
	Cell int
}

// CollectionCirclePartition covers only Domain over CellCount supplied cells;
// it does not assert that an omitted parent domain or periodic seam is covered.
// Intervals retain source-cell ancestry even across noncontact knots.
type CollectionCirclePartition struct {
	Complete  bool
	Domain    [2]*ExactAlgebraicRoot
	CellCount int
	Contacts  []CollectionCircleContact
	Intervals []CollectionCircleInterval
}

// ClassifyPlanarCircle assembles strict finite circle contacts over an exact
// continuous composition collection. It verifies every parameter/model seam,
// independently of public Complete. Coincident spans, gaps and discontinuities
// fail atomically. Equal model points at distinct domain ends are not welded.
// Options bound cell count and raw contacts (before seam deduplication); exact
// arithmetic limits apply separately to each seam or cell proof.
func (c SurfaceCurveCompositionCells) ClassifyPlanarCircle(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ComposedPlaneContactsOptions) (CollectionCirclePartition, error) {
	var zero CollectionCirclePartition
	if ctx == nil || !c.Complete || len(c.Cells) == 0 {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxCells == 0 {
		o.MaxCells = 64
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxCells < 1 || o.MaxCells > 4096 || o.MaxEvents < 1 || o.MaxEvents > 65536 {
		return zero, ErrInvalid
	}
	if len(c.Cells) > o.MaxCells {
		return zero, ErrConvergence
	}
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	if e = c.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	out := CollectionCirclePartition{}
	count := 0
	for i, cell := range c.Cells {
		part, e := cell.ClassifyPlanarCircle(ctx, axes, center, radiusSquared, opts)
		if e != nil {
			return zero, e
		}
		if !part.Complete || part.Coincident {
			return zero, ErrConvergence
		}
		count += len(part.Contacts)
		if count > o.MaxEvents {
			return zero, ErrConvergence
		}
		for _, interval := range part.Intervals {
			out.Intervals = append(out.Intervals, CollectionCircleInterval{PlanarCircleInterval: interval, Cell: i})
		}
		for _, contact := range part.Contacts {
			if contact.Event.endpointMask&1 != 0 && i > 0 {
				if len(out.Contacts) == 0 {
					return zero, ErrConvergence
				}
				last := &out.Contacts[len(out.Contacts)-1]
				if last.Cells[len(last.Cells)-1] != i-1 || last.Local[len(last.Local)-1].Event.endpointMask&2 == 0 {
					return zero, ErrConvergence
				}
				comparison, e := last.Event.CompareParameter(ctx, contact.Event, opts)
				if e != nil {
					return zero, e
				}
				if !comparison.Proven || comparison.Order != 0 {
					return zero, ErrConvergence
				}
				last.Cells = append(last.Cells, i)
				last.Local = append(last.Local, contact)
				last.SideSigns[1] = contact.SideSigns[1]
			} else {
				out.Contacts = append(out.Contacts, CollectionCircleContact{Event: contact.Event, Cells: []int{i}, Local: []PlanarCircleContact{contact}, SideSigns: contact.SideSigns})
			}
		}
	}
	for i := range out.Contacts {
		p := &out.Contacts[i]
		if p.SideSigns[0] != 0 && p.SideSigns[1] != 0 {
			p.Crossing = p.SideSigns[0] != p.SideSigns[1]
			p.Touching = !p.Crossing
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Domain[0], _ = c.Cells[0].EndpointRoot(0)
	out.Domain[1], _ = c.Cells[len(c.Cells)-1].EndpointRoot(1)
	out.CellCount = len(c.Cells)
	out.Complete = true
	return out, nil
}
