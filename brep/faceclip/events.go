package faceclip

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"sort"
)

// EdgeInterval names an original face-wire segment. Loop=0 is the outer wire,
// Loop>=1 indexes holes+1. Parameter0 is the oriented use's start, parameter1 its
// end, irrespective of the global edge orientation. From/To are nil when no
// point survives. Equal endpoints indicate only a zero-length contact.
type EdgeInterval struct {
	Loop, Segment, Edge  int
	Reversed             bool
	From, To             *big.Rat
	CoincidentHalfspaces []int
}

// SplitEvent identifies an interior original-segment crossing of cutting
// supports. Coincident segment/plane relationships are recorded on EdgeInterval
// instead. Coincident parameter events combine sorted halfspace indices.
// Retained says the point satisfies every halfspace, including equality.
type SplitEvent struct {
	Loop, Segment, Edge int
	Reversed            bool
	Parameter           *big.Rat
	Point               Point
	Halfspaces          []int
	Retained            bool
}
type EventResult struct {
	Region Result
	Edges  []EdgeInterval
	Events []SplitEvent
}

// ClipRegionWithEvents clips as ClipRegion and additionally reports exact
// source-wire intervals/events. Only original outer/hole segments are named,
// not internal tessellation diagonals. Endpoints at0/1 are not new split events.
// Geometry clipping and event collection have separate cumulative MaxWork
// budgets from RegionOptions. Event work includes edge*halfspace comparisons,
// event classification and sorting; all output rationals are caller-owned.
func ClipRegionWithEvents(ctx context.Context, s brep.Solid, face int, halfspaces []Halfspace, o RegionOptions) (EventResult, error) {
	empty := EventResult{}
	region, e := ClipRegion(ctx, s, face, halfspaces, o)
	if e != nil {
		return empty, e
	}
	result := EventResult{Region: region}
	limit, bits := o.Clip.MaxWork, o.Clip.MaxBits
	if limit == 0 {
		limit = 100000
	}
	if bits == 0 {
		bits = 16384
	}
	work := 0
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > limit {
			return ErrLimit
		}
		return nil
	}
	check := func(x *big.Rat) bool { return x.Num().BitLen() <= bits && x.Denom().BitLen() <= bits }
	f := s.Faces()[face]
	es, vs := s.Edges(), s.Vertices()
	loops := append([][]brep.EdgeUse{f.Boundary}, f.Holes...)
	for li, loop := range loops {
		for si, use := range loop {
			if e := tick(); e != nil {
				return empty, e
			}
			edge := es[use.Edge]
			a, b := vec(vs[edge.Start]), vec(vs[edge.End])
			if use.Reversed {
				a, b = b, a
			}
			lo, hi := new(big.Rat), big.NewRat(1, 1)
			entry := EdgeInterval{Loop: li, Segment: si, Edge: use.Edge, Reversed: use.Reversed}
			events := map[string]*SplitEvent{}
			possible := true
			for pi, h := range halfspaces {
				if e := tick(); e != nil {
					return empty, e
				}
				normal := vec(h.Normal)
				d0, d1 := sub(dot(normal, a), n(h.Offset)), sub(dot(normal, b), n(h.Offset))
				slope := sub(d1, d0)
				if slope.Sign() == 0 {
					if d0.Sign() > 0 {
						possible = false
					} else if d0.Sign() == 0 {
						entry.CoincidentHalfspaces = append(entry.CoincidentHalfspaces, pi)
					}
					continue
				}
				t := new(big.Rat).Neg(div(d0, slope))
				if !check(t) {
					return empty, ErrLimit
				}
				if slope.Sign() > 0 {
					if t.Cmp(hi) < 0 {
						hi = new(big.Rat).Set(t)
					}
				} else if t.Cmp(lo) > 0 {
					lo = new(big.Rat).Set(t)
				}
				if t.Sign() > 0 && t.Cmp(big.NewRat(1, 1)) < 0 {
					if e := tick(); e != nil {
						return empty, e
					}
					key := t.RatString()
					event, ok := events[key]
					if !ok {
						event = &SplitEvent{Loop: li, Segment: si, Edge: use.Edge, Reversed: use.Reversed, Parameter: new(big.Rat).Set(t)}
						for k := range a {
							event.Point[k] = add(a[k], mul(t, sub(b[k], a[k])))
							if !check(event.Point[k]) {
								return empty, ErrLimit
							}
						}
						events[key] = event
					}
					event.Halfspaces = append(event.Halfspaces, pi)
				}
			}
			if possible && lo.Cmp(hi) <= 0 && lo.Cmp(big.NewRat(1, 1)) <= 0 && hi.Sign() >= 0 {
				entry.From, entry.To = new(big.Rat).Set(lo), new(big.Rat).Set(hi)
			}
			var sorted []*SplitEvent
			for _, event := range events {
				sorted = append(sorted, event)
			}
			var sortError error
			sort.Slice(sorted, func(i, j int) bool {
				if sortError == nil {
					sortError = tick()
				}
				return sorted[i].Parameter.Cmp(sorted[j].Parameter) < 0
			})
			if sortError != nil {
				return empty, sortError
			}
			for _, event := range sorted {
				event.Retained = true
				for _, h := range halfspaces {
					if e := tick(); e != nil {
						return empty, e
					}
					if dot(vec(h.Normal), event.Point).Cmp(n(h.Offset)) > 0 {
						event.Retained = false
						break
					}
				}
				result.Events = append(result.Events, *event)
			}
			result.Edges = append(result.Edges, entry)
		}
	}
	return result, nil
}
