//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// AlgebraicGraphCapLineContact binds an exact event on origin+t*direction
// to the retained cap wire and use, including its exact curve parameter.
type AlgebraicGraphCapLineContact struct {
	Parameter, CurveParameter *geom.ExactAlgebraicRoot
	Wire, Use                 int
	Entry                     bool
	Touch                     bool // tangency with no change in material
	Junction                  bool // contact at the shared endpoint of two uses
	OtherUse                  int  // outgoing use when Junction, otherwise -1
	OtherCurveParameter       *geom.ExactAlgebraicRoot
	insideBefore, insideAfter bool
}

// AlgebraicGraphCapLine reports the ordered boundary events and clipped
// material intervals within the requested finite line range. Isolated
// touches of an otherwise exterior line are contacts, not material spans.
// Endpoints are immutable exact roots, including caller limits when the
// line is already inside at either end.
type AlgebraicGraphCapLine struct {
	Contacts                      []AlgebraicGraphCapLineContact
	Material                      [][2]*geom.ExactAlgebraicRoot
	ExactOperations, Subdivisions int
}

// IntersectRationalLine classifies a finite rational line in the cap plane.
// It isolates every regular crossing on the actual algebraic curve domains,
// orders coordinates exactly, and rejects contact with a trim vertex,
// tangencies, and coincident runs. Direction is not normalized.
func (c AlgebraicGraphCapClip) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapLine, error) {
	return c.intersectRationalLine(ctx, origin, direction, extent, options, false, false)
}

// IntersectRationalLineWithTouches additionally reports regular isolated
// even-order tangencies without toggling interior. Coincidence, trim
// vertices and higher odd-order crossings remain outside this method.
func (c AlgebraicGraphCapClip) IntersectRationalLineWithTouches(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapLine, error) {
	return c.intersectRationalLine(ctx, origin, direction, extent, options, true, false)
}

// IntersectRationalLineWithJunctions additionally handles a line through
// a shared trim vertex. Both incident curve germs are matched on their
// original wire; their exact available signs determine a crossing or
// touch. Regular odd-order interior contacts likewise cross, even if the
// curve is tangent to the line. Coincident runs and singular germs remain
// unsupported.
func (c AlgebraicGraphCapClip) IntersectRationalLineWithJunctions(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapLine, error) {
	return c.intersectRationalLine(ctx, origin, direction, extent, options, true, true)
}

func (c AlgebraicGraphCapClip) intersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	options geom.ExactCurvePlaneEventOptions, includeTouches, includeJunctions bool) (AlgebraicGraphCapLine, error) {
	var zero AlgebraicGraphCapLine
	if ctx == nil || c.regions == nil || options.MaxOperations < 0 || options.MaxCoefficientBits < 0 || options.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if options.MaxOperations == 0 {
		options.MaxOperations = 2000000
	}
	if options.MaxCoefficientBits == 0 {
		options.MaxCoefficientBits = 16384
	}
	if options.MaxSubdivisions == 0 {
		options.MaxSubdivisions = 8192
	}
	if options.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, options.MaxOperations, options.MaxSubdivisions, options.MaxCoefficientBits}
	for _, row := range [][]*big.Rat{origin[:], direction[:], extent[:]} {
		for _, x := range row {
			if x == nil {
				return zero, geom.ErrInvalid
			}
			if x.Num().BitLen() > b.bits || x.Denom().BitLen() > b.bits {
				return zero, geom.ErrConvergence
			}
		}
	}
	if extent[0].Cmp(extent[1]) >= 0 {
		return zero, geom.ErrDomain
	}
	cap := c.regions.plane
	if cap.dot(origin).Cmp(cap.offset) != 0 || cap.dot(direction).Sign() != 0 {
		return zero, geom.ErrDomain
	}
	x, y := orientedRationalGraphProjection(cap)[0], orientedRationalGraphProjection(cap)[1]
	axis := x
	if direction[x].Sign() == 0 {
		axis = y
		if direction[axis].Sign() == 0 {
			return zero, geom.ErrDegenerate
		}
	}
	// cross(direction, point-origin), positive to the line's left.
	var plane [4]*big.Rat
	plane[x], plane[y] = new(big.Rat).Neg(direction[y]), new(big.Rat).Set(direction[x])
	plane[3] = new(big.Rat).Sub(new(big.Rat).Mul(direction[y], origin[x]), new(big.Rat).Mul(direction[x], origin[y]))
	scale := new(big.Rat).Inv(direction[axis])
	offset := new(big.Rat).Neg(new(big.Rat).Mul(scale, origin[axis]))
	var events []AlgebraicGraphCapLineContact
	type vertexKey struct{ wire, before int }
	type vertexGerm struct {
		contact AlgebraicGraphCapLineContact
		sign    int
	}
	junctions := map[vertexKey][2]vertexGerm{}
	for wi, wire := range c.wires {
		for ui, u := range wire {
			intersection, e := u.Cell.IntersectRawPlane(ctx, plane, b.limits())
			if e != nil {
				return zero, e
			}
			if e := b.charge(intersection.Operations, intersection.Subdivisions); e != nil {
				return zero, e
			}
			if !intersection.Complete || intersection.Coincident {
				return zero, geom.ErrConvergence
			}
			for _, candidate := range intersection.Events {
				p, e := candidate.ClassifyPlaneContact(ctx, plane, b.limits())
				if e != nil {
					return zero, e
				}
				if e := b.charge(p.Operations, p.Subdivisions); e != nil {
					return zero, e
				}
				touch := includeTouches && p.Tangent && p.Touching
				endpoint := p.EndpointMask != 0
				crossing := p.Crossing && (p.Transverse || includeJunctions && !p.Singular)
				if !p.Proven || endpoint && (!includeJunctions || p.Singular || p.EndpointMask > 2) ||
					!endpoint && !touch && !crossing {
					return zero, geom.ErrConvergence
				}
				a, used, steps, e := candidate.CoordinateRootWithWork(ctx, axis, b.limits())
				if e != nil {
					return zero, e
				}
				if e := b.charge(used, steps); e != nil {
					return zero, e
				}
				a, e = b.mapRoot(a, offset, scale)
				if e != nil {
					return zero, e
				}
				if endpoint {
					wireSide := p.EndpointMask == 2
					if u.Reversed {
						wireSide = !wireSide
					}
					k, side := vertexKey{wi, ui}, 0
					if !wireSide {
						k.before = (ui + len(wire) - 1) % len(wire)
						side = 1
					}
					pair := junctions[k]
					if pair[side].contact.Parameter != nil {
						return zero, geom.ErrConvergence
					}
					germ := p.SideSigns[0]
					if p.EndpointMask == 1 {
						germ = p.SideSigns[1]
					}
					if germ == 0 {
						return zero, geom.ErrConvergence
					}
					pair[side] = vertexGerm{AlgebraicGraphCapLineContact{Parameter: a, CurveParameter: candidate.Root(), Wire: wi, Use: ui, OtherUse: -1}, germ}
					junctions[k] = pair
					continue
				}
				// Material is on the left of an oriented wire. A wire
				// crossing the line from left to right therefore enters.
				entry := (p.SideSigns[0] > 0) != u.Reversed
				events = append(events, AlgebraicGraphCapLineContact{Parameter: a, CurveParameter: candidate.Root(), Wire: wi, Use: ui, Entry: !touch && entry, Touch: touch, OtherUse: -1})
			}
		}
	}
	cmp := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		q, e := a.Compare(ctx, z, b.limits())
		if e != nil {
			return 0, e
		}
		if e := b.charge(q.Operations, q.Subdivisions); e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, geom.ErrConvergence
		}
		return q.Order, nil
	}
	// Traverse original wire order, independent of map iteration.
	for wi, w := range c.wires {
		for ui := range w {
			pair, ok := junctions[vertexKey{wi, ui}]
			if !ok {
				continue
			}
			a, z := pair[0], pair[1]
			if a.contact.Parameter == nil || z.contact.Parameter == nil {
				return zero, geom.ErrConvergence
			}
			// Both curve events lie on this exact line, and its
			// nonzero axis component is injective. Equality of their
			// line parameters independently proves model identity.
			order, e := cmp(a.contact.Parameter, z.contact.Parameter)
			if e != nil {
				return zero, e
			}
			if order != 0 {
				return zero, geom.ErrConvergence
			}
			ev := a.contact
			ev.Junction = true
			ev.OtherUse = z.contact.Use
			ev.OtherCurveParameter = z.contact.CurveParameter
			ev.Touch = a.sign == z.sign
			ev.Entry = !ev.Touch && a.sign > 0
			events = append(events, ev)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Parameter.Interval().Lower.Cmp(events[j].Parameter.Interval().Lower) < 0
	})
	for i := 1; i < len(events); i++ {
		for j := i; j > 0; j-- {
			k, e := cmp(events[j-1].Parameter, events[j].Parameter)
			if e != nil {
				return zero, e
			}
			if k == 0 {
				return zero, geom.ErrConvergence
			}
			if k < 0 {
				break
			}
			events[j-1], events[j] = events[j], events[j-1]
		}
	}
	var limits [2]*geom.ExactAlgebraicRoot
	for i, z := range extent {
		a, used, e := geom.NewExactRationalRoot(ctx, z, b.limits())
		if e != nil {
			return zero, e
		}
		if e := b.charge(used, 0); e != nil {
			return zero, e
		}
		limits[i] = a
	}
	var out AlgebraicGraphCapLine
	var start *geom.ExactAlgebraicRoot
	for _, ev := range events {
		ev.insideBefore = start != nil
		ev.insideAfter = ev.insideBefore
		if !ev.Touch {
			ev.insideAfter = ev.Entry
		}
		lo, e := cmp(limits[0], ev.Parameter)
		if e != nil {
			return zero, e
		}
		hi, e := cmp(ev.Parameter, limits[1])
		if e != nil {
			return zero, e
		}
		if lo <= 0 && hi <= 0 {
			out.Contacts = append(out.Contacts, ev)
		}
		if ev.Touch {
			continue
		}
		if ev.Entry {
			if start != nil {
				return zero, geom.ErrConvergence
			}
			start = ev.Parameter
			continue
		}
		if start == nil {
			return zero, geom.ErrConvergence
		}
		span := [2]*geom.ExactAlgebraicRoot{start, ev.Parameter}
		before, e := cmp(span[0], limits[0])
		if e != nil {
			return zero, e
		}
		if before < 0 {
			span[0] = limits[0]
		}
		if hi > 0 {
			span[1] = limits[1]
		}
		ordered, e := cmp(span[0], span[1])
		if e != nil {
			return zero, e
		}
		if ordered <= 0 {
			out.Material = append(out.Material, span)
		}
		start = nil
	}
	if start != nil {
		return zero, geom.ErrConvergence
	}
	out.ExactOperations, out.Subdivisions = options.MaxOperations-b.work, options.MaxSubdivisions-b.subdiv
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
