package geom

import (
	"context"
	"math/big"
)

// ComposedPlaneComponent is either an isolated event (Point nonnil) or one
// maximal interval of coincident cells (Start/End nonnil). Cells identifies
// every coincident source cell. Before/After retain adjacent noncoincident
// endpoint germs; nil means that side is outside the collection. Zero signs
// within the interval imply neither crossing nor touching across its extent.
type ComposedPlaneComponent struct {
	Point         *ComposedPlaneContactEntry
	Start, End    *ExactComposedPlaneEvent
	Cells         []int
	Before, After *ComposedPlaneContact
}
type ComposedPlaneComponents struct {
	Complete   bool
	Components []ComposedPlaneComponent
}

// DiscoverPlaneContactComponents opts into whole coincident spans. Adjacent
// spans merge only after exact domain/model seam validation. Their endpoint
// events are absorbed, not duplicated as isolated contacts. Limits are those
// of DiscoverPlaneContacts; MaxEvents counts raw roots plus span endpoints.
// No conclusion about material arrangement or tangent continuity is implied.
func (c SurfaceCurveCompositionCells) DiscoverPlaneContactComponents(ctx context.Context, plane [4]*big.Rat, o ComposedPlaneContactsOptions) (ComposedPlaneComponents, error) {
	var zero ComposedPlaneComponents
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
	hits := make([]ComposedCurvePlaneIntersection, len(c.Cells))
	count := 0
	for i, cell := range c.Cells {
		r, e := cell.IntersectRawPlane(ctx, plane, opts)
		if e != nil {
			return zero, e
		}
		if !r.Complete {
			return zero, ErrConvergence
		}
		count += len(r.Events)
		if r.Coincident {
			count += 2
		}
		if count > o.MaxEvents {
			return zero, ErrConvergence
		}
		hits[i] = r
	}
	adjacent := func(index int, mask uint8, boundary *ExactComposedPlaneEvent) (*ComposedPlaneContact, error) {
		for _, event := range hits[index].Events {
			if event.endpointMask != mask {
				continue
			}
			q, e := event.CompareParameter(ctx, boundary, opts)
			if e != nil {
				return nil, e
			}
			if !q.Proven || q.Order != 0 {
				return nil, ErrDomain
			}
			local, e := event.ClassifyPlaneContact(ctx, plane, opts)
			if e != nil {
				return nil, e
			}
			return &local, nil
		}
		return nil, ErrConvergence
	}
	out := ComposedPlaneComponents{}
	for i := 0; i < len(hits); i++ {
		if hits[i].Coincident {
			end := i
			for end+1 < len(hits) && hits[end+1].Coincident {
				end++
			}
			component := ComposedPlaneComponent{Start: hits[i].Boundary[0], End: hits[end].Boundary[1]}
			for j := i; j <= end; j++ {
				component.Cells = append(component.Cells, j)
			}
			if i > 0 {
				component.Before, e = adjacent(i-1, 2, component.Start)
				if e != nil {
					return zero, e
				}
			}
			if end+1 < len(hits) {
				component.After, e = adjacent(end+1, 1, component.End)
				if e != nil {
					return zero, e
				}
			}
			out.Components = append(out.Components, component)
			i = end
			continue
		}
		for _, event := range hits[i].Events {
			if event.endpointMask&1 != 0 && i > 0 && hits[i-1].Coincident {
				continue
			}
			if event.endpointMask&2 != 0 && i+1 < len(hits) && hits[i+1].Coincident {
				continue
			}
			local, e := event.ClassifyPlaneContact(ctx, plane, opts)
			if e != nil {
				return zero, e
			}
			if len(out.Components) > 0 {
				prev := out.Components[len(out.Components)-1].Point
				if prev != nil {
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
						prev.Cells = append(prev.Cells, i)
						prev.Local = append(prev.Local, local)
						prev.Junction = &join
						continue
					}
				}
			}
			point := &ComposedPlaneContactEntry{Event: event, Cells: []int{i}, Local: []ComposedPlaneContact{local}}
			out.Components = append(out.Components, ComposedPlaneComponent{Point: point})
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	return out, nil
}

// DiscoverPlaneContactComponents is the standalone exact model-edge opt-in.
func (c *SurfaceIsoCurve) DiscoverPlaneContactComponents(ctx context.Context, plane [4]*big.Rat, o ComposedPlaneContactsOptions) (ComposedPlaneComponents, error) {
	var zero ComposedPlaneComponents
	opts, e := graphRootOptions(o.EventOptions)
	if e != nil {
		return zero, e
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits}, o.MaxCells)
	if e != nil {
		return zero, e
	}
	return cells.DiscoverPlaneContactComponents(ctx, plane, o)
}
