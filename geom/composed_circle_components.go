package geom

import (
	"context"
	"math/big"
)

// CoincidentCirclePiece retains the exact source interval and traversal sign:
// +1 is counterclockwise in the requested ordered model axes. No cancellation
// or welding of oppositely traversed pieces is implied.
type CoincidentCirclePiece struct {
	Cell        int
	Start, End  *ExactAlgebraicRoot
	Orientation int
}

// CircleContactComponent is either Point, or a maximal contiguous sequence of
// coincident source cells. Before/After are neighboring noncoincident endpoint
// germs, retained with source ownership. They do not turn a continuous contact
// interval into a crossing/touching point. Pieces may reverse at a C0 seam.
type CircleContactComponent struct {
	ConstantPieces []ConstantCirclePiece
	Point          *CollectionCircleContact
	Pieces         []CoincidentCirclePiece
	Before, After  *CollectionCircleContact
}

type CircleContactComponents struct {
	Complete   bool
	Domain     [2]*ExactAlgebraicRoot
	CellCount  int
	Components []CircleContactComponent
	Intervals  []CollectionCircleInterval
}

// ClassifyPlanarCircleComponents opts into whole coincident cells. It verifies
// every seam and proves a nonzero angular derivative throughout each coincident
// cell. Stationary/folding coincidence remains unresolved. Exact proof limits
// are per cell/stage; MaxEvents counts raw point contacts plus two endpoints per
// coincident cell. Failure publishes no partial components. Completeness covers
// only the supplied Domain, not an omitted parent or periodic quotient seam.
func (c SurfaceCurveCompositionCells) ClassifyPlanarCircleComponents(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ComposedPlaneContactsOptions) (CircleContactComponents, error) {
	r, e := c.circleContactComponents(ctx, axes, center, radiusSquared, o, false, false)
	return r.CircleContactComponents, e
}

func (c SurfaceCurveCompositionCells) circleContactComponents(ctx context.Context, axes [2]int, center [3]*big.Rat, radiusSquared *big.Rat, o ComposedPlaneContactsOptions, stationary, constants bool) (StationaryCircleContactComponents, error) {
	var zero StationaryCircleContactComponents
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
	parts := make([]PlanarCirclePartition, len(c.Cells))
	oriented := make([][]CoincidentCirclePiece, len(c.Cells))
	var stationaryEvents []CollectionCircleStationaryEvent
	constantPieces := make([]*ConstantCirclePiece, len(c.Cells))
	count := 0
	for i, cell := range c.Cells {
		p, e := cell.ClassifyPlanarCircle(ctx, axes, center, radiusSquared, opts)
		if e != nil {
			return zero, e
		}
		if !p.Complete {
			return zero, ErrConvergence
		}
		parts[i] = p
		count += len(p.Contacts)
		if p.Coincident {
			count += 2
		}
		if count > o.MaxEvents {
			return zero, ErrConvergence
		}
		if p.Coincident {
			if constants {
				point, ok, e := cell.constantCirclePoint(ctx, opts)
				if e != nil {
					return zero, e
				}
				if ok {
					constantPieces[i] = &ConstantCirclePiece{Cell: i, Start: p.Domain[0], End: p.Domain[1], Point: point}
					continue
				}
			}
			proof, e := cell.SplitCoincidentCircleOrientation(ctx, axes, center, radiusSquared, opts)
			if e != nil {
				return zero, e
			}
			if !proof.Complete {
				return zero, ErrConvergence
			}
			if !stationary && len(proof.Stationary) != 0 {
				return zero, ErrConvergence
			}
			count += len(proof.Stationary)
			if count > o.MaxEvents {
				return zero, ErrConvergence
			}
			for _, piece := range proof.Pieces {
				piece.Cell = i
				oriented[i] = append(oriented[i], piece)
			}
			for _, event := range proof.Stationary {
				if event.Event.endpointMask&1 != 0 && i > 0 && parts[i-1].Coincident && len(stationaryEvents) > 0 {
					last := &stationaryEvents[len(stationaryEvents)-1]
					if last.Cells[len(last.Cells)-1] == i-1 && last.Local[len(last.Local)-1].Event.endpointMask&2 != 0 {
						cmp, e := last.Event.CompareParameter(ctx, event.Event, opts)
						if e != nil {
							return zero, e
						}
						if !cmp.Proven || cmp.Order != 0 {
							return zero, ErrConvergence
						}
						last.Cells = append(last.Cells, i)
						last.Local = append(last.Local, event)
						last.SideOrientations[1] = event.SideOrientations[1]
						continue
					}
				}
				stationaryEvents = append(stationaryEvents, CollectionCircleStationaryEvent{Event: event.Event, Cells: []int{i}, Local: []CircleStationaryEvent{event}, SideOrientations: event.SideOrientations})
			}
		}
	}
	out := StationaryCircleContactComponents{Stationary: stationaryEvents}
	entry := func(i int, p PlanarCircleContact) *CollectionCircleContact {
		return &CollectionCircleContact{Event: p.Event, Cells: []int{i}, Local: []PlanarCircleContact{p}, SideSigns: p.SideSigns}
	}
	for i := 0; i < len(parts); {
		p := parts[i]
		if p.Coincident {
			component := CircleContactComponent{}
			first := i
			for i < len(parts) && parts[i].Coincident {
				component.Pieces = append(component.Pieces, oriented[i]...)
				if constantPieces[i] != nil {
					component.ConstantPieces = append(component.ConstantPieces, *constantPieces[i])
				}
				i++
			}
			if first > 0 {
				previous := parts[first-1].Contacts
				if len(previous) == 0 || previous[len(previous)-1].Event.endpointMask&2 == 0 {
					return zero, ErrConvergence
				}
				component.Before = entry(first-1, previous[len(previous)-1])
			}
			if i < len(parts) {
				next := parts[i].Contacts
				if len(next) == 0 || next[0].Event.endpointMask&1 == 0 {
					return zero, ErrConvergence
				}
				component.After = entry(i, next[0])
			}
			out.Components = append(out.Components, component)
			continue
		}
		for _, interval := range p.Intervals {
			out.Intervals = append(out.Intervals, CollectionCircleInterval{PlanarCircleInterval: interval, Cell: i})
		}
		for _, contact := range p.Contacts {
			if contact.Event.endpointMask&1 != 0 && i > 0 && parts[i-1].Coincident {
				continue
			}
			if contact.Event.endpointMask&2 != 0 && i+1 < len(parts) && parts[i+1].Coincident {
				continue
			}
			if contact.Event.endpointMask&1 != 0 && i > 0 {
				if len(out.Components) == 0 {
					return zero, ErrConvergence
				}
				last := out.Components[len(out.Components)-1].Point
				if last == nil || last.Cells[len(last.Cells)-1] != i-1 || last.Local[len(last.Local)-1].Event.endpointMask&2 == 0 {
					return zero, ErrConvergence
				}
				last.Cells = append(last.Cells, i)
				last.Local = append(last.Local, contact)
				last.SideSigns[1] = contact.SideSigns[1]
			} else {
				out.Components = append(out.Components, CircleContactComponent{Point: entry(i, contact)})
			}
		}
		i++
	}
	for i := range out.Components {
		p := out.Components[i].Point
		if p != nil && p.SideSigns[0] != 0 && p.SideSigns[1] != 0 {
			p.Crossing = p.SideSigns[0] != p.SideSigns[1]
			p.Touching = !p.Crossing
		}
	}
	for i := range out.Stationary {
		p := &out.Stationary[i]
		p.Reverses = p.SideOrientations[0] != 0 && p.SideOrientations[1] != 0 && p.SideOrientations[0] != p.SideOrientations[1]
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.Domain = [2]*ExactAlgebraicRoot{parts[0].Domain[0], parts[len(parts)-1].Domain[1]}
	out.CellCount = len(parts)
	out.Complete = true
	return out, nil
}
