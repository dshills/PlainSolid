package geom

import (
	"context"
	"math/big"
)

// ComposedPlaneContactsOptions bounds the collection and each exact proof stage.
// At most MaxCells seam validations/isolations and MaxEvents local/junction
// classifications run. EventOptions limits each stage, not their aggregate.
type ComposedPlaneContactsOptions struct {
	EventOptions        ExactCurvePlaneEventOptions
	MaxCells, MaxEvents int
}

// ComposedPlaneContactEntry retains source indices and immutable event provenance.
// Local is in original cell order. Junction is nonnil only for a proved shared
// endpoint. No cross-cell velocity continuity follows from that join.
type ComposedPlaneContactEntry struct {
	Event    *ExactComposedPlaneEvent
	Cells    []int
	Local    []ComposedPlaneContact
	Junction *ComposedPlaneJunction
}
type ComposedPlaneContactTable struct {
	Complete bool
	Events   []ComposedPlaneContactEntry
}

// DiscoverPlaneContacts proves the finite ordered contact table on this entire
// collection's domain. Public Complete is necessary but not trusted as proof:
// every adjacent exact endpoint and model point is revalidated. Coincident
// intervals, gaps, overlaps, discontinuities and unsupported model-coordinate
// algebraic degrees fail atomically. A successful empty table proves no contact.
func (c SurfaceCurveCompositionCells) DiscoverPlaneContacts(ctx context.Context, plane [4]*big.Rat, o ComposedPlaneContactsOptions) (ComposedPlaneContactTable, error) {
	var zero ComposedPlaneContactTable
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
	if e := c.validateContactSeams(ctx, opts); e != nil {
		return zero, e
	}
	out := ComposedPlaneContactTable{}
	rawCount := 0
	for i, cell := range c.Cells {
		hits, e := cell.IntersectRawPlane(ctx, plane, opts)
		if e != nil {
			return zero, e
		}
		if !hits.Complete || hits.Coincident {
			return zero, ErrConvergence
		}
		rawCount += len(hits.Events)
		if rawCount > o.MaxEvents {
			return zero, ErrConvergence
		}
		for _, event := range hits.Events {
			local, e := event.ClassifyPlaneContact(ctx, plane, opts)
			if e != nil {
				return zero, e
			}
			if !local.Proven {
				return zero, ErrConvergence
			}
			if len(out.Events) > 0 {
				prev := &out.Events[len(out.Events)-1]
				q, e := prev.Event.CompareParameter(ctx, event, opts)
				if e != nil {
					return zero, e
				}
				if !q.Proven || q.Order > 0 {
					return zero, ErrConvergence
				}
				if q.Order == 0 {
					if len(prev.Cells) != 1 || prev.Cells[0] != i-1 {
						return zero, ErrDomain
					}
					join, e := ClassifyComposedPlaneJunction(ctx, prev.Event, event, plane, opts)
					if e != nil {
						return zero, e
					}
					if !join.Proven {
						return zero, ErrConvergence
					}
					prev.Cells = append(prev.Cells, i)
					prev.Local = append(prev.Local, local)
					prev.Junction = &join
					continue
				}
			}
			out.Events = append(out.Events, ComposedPlaneContactEntry{Event: event, Cells: []int{i}, Local: []ComposedPlaneContact{local}})
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	return out, nil
}

func (c SurfaceCurveCompositionCells) validateContactSeams(ctx context.Context, opts ExactCurvePlaneEventOptions) error {
	for i, cell := range c.Cells {
		if cell == nil {
			return ErrInvalid
		}
		start, e := cell.EndpointRoot(0)
		if e != nil {
			return e
		}
		end, e := cell.EndpointRoot(1)
		if e != nil {
			return e
		}
		order, e := start.Compare(ctx, end, opts)
		if e != nil {
			return e
		}
		if !order.Proven || order.Order != -1 {
			return ErrDomain
		}
		if i > 0 {
			prev := c.Cells[i-1]
			last, e := prev.EndpointRoot(1)
			if e != nil {
				return e
			}
			q, e := last.Compare(ctx, start, opts)
			if e != nil {
				return e
			}
			if !q.Proven || q.Order != 0 {
				return ErrDomain
			}
			a, e := prev.BindRoot(ctx, last, opts)
			if e != nil {
				return e
			}
			b, e := cell.BindRoot(ctx, start, opts)
			if e != nil {
				return e
			}
			equal, _, e := a.CompareModelPoint(ctx, b, opts)
			if e != nil {
				return e
			}
			if !equal {
				return ErrDomain
			}
		}
	}
	return nil
}
