package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphVerticalLineContact is one exact event. Cap and OtherCap
// index cap roles; StockFace and OtherStockFace index original stock
// supports. An absent owner is -1. Junction indicates a shared boundary
// of several supports, even if the original chart is geometrically smooth.
type AlgebraicGraphVerticalLineContact struct {
	Parameter                 *geom.ExactAlgebraicRoot
	Cap, OtherCap             int
	StockFace, OtherStockFace int
	Entry, Junction           bool
	// Touch is true for an isolated tangency without a transition.
	Touch bool
}

type AlgebraicGraphVerticalLine struct {
	Contacts                      []AlgebraicGraphVerticalLineContact
	Material                      [][2]*geom.ExactAlgebraicRoot
	ExactOperations, Subdivisions int
}

// IntersectRationalLine computes exact finite line intervals through the
// prepared vertical material where every retained wall spans both caps. It pulls the line into the lower cap and
// intersects every original projected trim, including irrational crossings.
// Those candidates contribute wall contacts only between the two exact cap
// heights. The two cap interiors also contribute their own exact plane hits.
// Tangencies, vertices and coincident intervals remain unsupported.
func (m AlgebraicGraphVerticalMaterial) IntersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return m.intersectRationalLine(ctx, origin, direction, extent, opts, false, false)
}

// IntersectRationalLineWithTouches additionally retains regular isolated
// wall tangencies. These leave both open material sides unchanged.
func (m AlgebraicGraphVerticalMaterial) IntersectRationalLineWithTouches(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return m.intersectRationalLine(ctx, origin, direction, extent, opts, true, false)
}

// IntersectRationalLineWithJunctions also resolves isolated events shared by
// stock walls and caps. It combines exact material sides of the common
// projected perimeter and both cap halfspaces. A line on a cap plane,
// coincident with a projected trim, or singular remains unsupported.
func (m AlgebraicGraphVerticalMaterial) IntersectRationalLineWithJunctions(ctx context.Context, origin, direction [3]*big.Rat,
	extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphVerticalLine, error) {
	return m.intersectRationalLine(ctx, origin, direction, extent, opts, true, true)
}

func (m AlgebraicGraphVerticalMaterial) intersectRationalLine(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	opts geom.ExactCurvePlaneEventOptions, includeTouches, includeJunctions bool) (AlgebraicGraphVerticalLine, error) {
	var zero AlgebraicGraphVerticalLine
	if ctx == nil || m.caps[0].recipe == nil || opts.MaxOperations < 0 || opts.MaxCoefficientBits < 0 || opts.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if !m.lineReady {
		return zero, geom.ErrUnrepresentable
	}
	if opts.MaxOperations == 0 {
		opts.MaxOperations = 2000000
	}
	if opts.MaxCoefficientBits == 0 {
		opts.MaxCoefficientBits = m.maxBits
	}
	if opts.MaxSubdivisions == 0 {
		opts.MaxSubdivisions = 8192
	}
	if opts.MaxCoefficientBits < 64 {
		return zero, geom.ErrInvalid
	}
	b := capClipBudget{ctx, opts.MaxOperations, opts.MaxSubdivisions, opts.MaxCoefficientBits}
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
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 {
		return zero, geom.ErrDegenerate
	}
	root := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		p, used, e := geom.NewExactRationalRoot(ctx, t, b.limits())
		if e != nil {
			return nil, e
		}
		return p, b.charge(used, 0)
	}
	point := func(t *big.Rat) (p [3]*big.Rat) {
		for i := range p {
			p[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(direction[i], t))
		}
		return p
	}
	var events []AlgebraicGraphVerticalLineContact
	var residual [2][]*big.Rat
	var capBoundary [2]bool
	var capTimes [2]*big.Rat
	for i, cap := range m.caps {
		plane := cap.regions.plane
		slope := plane.dot(direction)
		cst := new(big.Rat).Sub(plane.dot(origin), plane.offset)
		residual[i] = []*big.Rat{new(big.Rat).Mul(cst, big.NewRat(int64(plane.orientation), 1)),
			new(big.Rat).Mul(slope, big.NewRat(int64(plane.orientation), 1))}
		if slope.Sign() == 0 {
			if cst.Sign() == 0 {
				return zero, geom.ErrConvergence
			}
			continue
		}
		t := new(big.Rat).Quo(new(big.Rat).Neg(cst), slope)
		capTimes[i] = t
		p := point(t)
		where, e := cap.locateRationalBudget(p, &b)
		if e != nil {
			return zero, e
		}
		if where == trimvalidate.Outside {
			continue
		}
		if where == trimvalidate.Boundary && includeJunctions {
			capBoundary[i] = true
			continue
		}
		if where != trimvalidate.Inside {
			return zero, geom.ErrConvergence
		}
		h, e := root(t)
		if e != nil {
			return zero, e
		}
		events = append(events, AlgebraicGraphVerticalLineContact{Parameter: h, Cap: i, OtherCap: -1,
			StockFace: -1, OtherStockFace: -1, Entry: slope.Sign()*plane.orientation < 0})
	}
	if includeJunctions && direction[0].Sign() == 0 && direction[1].Sign() == 0 &&
		capBoundary[0] && capBoundary[1] && capTimes[0].Cmp(capTimes[1]) == 0 {
		// On an exact vertical line the complete fiber is between the
		// two cap heights. Here it is a singleton; a positive height
		// would be a line coincident with a wall, which we still reject.
		p := point(capTimes[0])
		owners, e := m.exactVerticalPinchOwners(p, &b)
		if e != nil {
			return zero, e
		}
		h, e := root(capTimes[0])
		if e != nil {
			return zero, e
		}
		events = append(events, AlgebraicGraphVerticalLineContact{Parameter: h, Cap: 0, OtherCap: 1,
			StockFace: owners[0], OtherStockFace: owners[1], Junction: true, Touch: true})
		capBoundary = [2]bool{}
	}
	if direction[0].Sign() != 0 || direction[1].Sign() != 0 {
		cap := m.caps[m.lower]
		plane := cap.regions.plane
		a, d := origin, direction
		a[2] = new(big.Rat).Sub(origin[2], new(big.Rat).Sub(plane.dot(origin), plane.offset))
		d[2] = new(big.Rat).Sub(direction[2], plane.dot(direction))
		axis := 0
		if d[0].Sign() == 0 {
			axis = 1
		}
		// All projected vertices are bounded by the exact positive control
		// hull of the original cap. Extend beyond both extrema to discover
		// the complete crossing ledger even for a window already inside.
		var lo, hi *big.Rat
		project := orientedRationalGraphProjection(plane)
		k := 0
		if project[0] != axis {
			k = 1
		}
		for _, w := range cap.regions.wires {
			for _, e := range w {
				for _, h := range e.control {
					if e := b.charge(4, 0); e != nil {
						return zero, e
					}
					x := new(big.Rat).Quo(h[k], h[2])
					t := new(big.Rat).Quo(new(big.Rat).Sub(x, a[axis]), d[axis])
					if max(t.Num().BitLen(), t.Denom().BitLen()) > b.bits {
						return zero, geom.ErrConvergence
					}
					if lo == nil || t.Cmp(lo) < 0 {
						lo = t
					}
					if hi == nil || t.Cmp(hi) > 0 {
						hi = t
					}
				}
			}
		}
		if lo == nil || hi == nil {
			return zero, geom.ErrConvergence
		}
		h, e := cap.intersectRationalLine(ctx, a, d, [2]*big.Rat{new(big.Rat).Sub(lo, big.NewRat(1, 1)), new(big.Rat).Add(hi, big.NewRat(1, 1))}, b.limits(), includeTouches, includeJunctions)
		if e != nil {
			return zero, e
		}
		if e := b.charge(h.ExactOperations, h.Subdivisions); e != nil {
			return zero, e
		}
		for _, hit := range h.Contacts {
			signs := [2]int{}
			for i, poly := range residual {
				s, e := hit.Parameter.SignPolynomial(ctx, poly, b.limits())
				if e != nil {
					return zero, e
				}
				if e := b.charge(s.Operations, s.Subdivisions); e != nil {
					return zero, e
				}
				signs[i] = s.Sign
			}
			if signs[0] > 0 || signs[1] > 0 {
				continue
			}
			event := AlgebraicGraphVerticalLineContact{Parameter: hit.Parameter, Cap: -1, OtherCap: -1,
				StockFace: -1, OtherStockFace: -1, Entry: hit.Entry, Touch: hit.Touch}
			before, after := hit.insideBefore, hit.insideAfter
			for i, sign := range signs {
				if sign != 0 {
					continue
				}
				// Vanishing of this exact affine residual establishes
				// identity with the independently found cap plane hit.
				// Its slope gives both strict halfspace signs.
				slope := residual[i][1].Sign()
				if !includeJunctions || !capBoundary[i] || slope == 0 {
					return zero, geom.ErrConvergence
				}
				capBoundary[i] = false
				if event.Cap < 0 {
					event.Cap = i
				} else {
					event.OtherCap = i
				}
				before = before && slope > 0
				after = after && slope < 0
			}
			uses := [2]int{hit.Use, hit.OtherUse}
			for _, use := range uses {
				if use < 0 {
					continue
				}
				u := cap.wires[hit.Wire][use]
				if u.Seam >= 0 {
					// Generated cap seams have no original wall owner.
					if event.OtherCap < 0 {
						return zero, geom.ErrConvergence
					}
					continue
				}
				if u.Source.Face < 0 {
					return zero, geom.ErrConvergence
				}
				if event.StockFace < 0 {
					event.StockFace = u.Source.Face
				} else if event.StockFace != u.Source.Face {
					event.OtherStockFace = u.Source.Face
				}
			}
			if event.StockFace < 0 && event.OtherCap < 0 {
				return zero, geom.ErrConvergence
			}
			event.Junction = hit.Junction || event.Cap >= 0
			event.Touch, event.Entry = before == after, !before && after
			events = append(events, event)
		}
	}
	if capBoundary[0] || capBoundary[1] {
		return zero, geom.ErrConvergence
	}
	cmp := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		r, e := a.Compare(ctx, z, b.limits())
		if e != nil {
			return 0, e
		}
		if e := b.charge(r.Operations, r.Subdivisions); e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
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
	var limit [2]*geom.ExactAlgebraicRoot
	for i, t := range extent {
		a, e := root(t)
		if e != nil {
			return zero, e
		}
		limit[i] = a
	}
	var out AlgebraicGraphVerticalLine
	var start *geom.ExactAlgebraicRoot
	for _, event := range events {
		lo, e := cmp(limit[0], event.Parameter)
		if e != nil {
			return zero, e
		}
		hi, e := cmp(event.Parameter, limit[1])
		if e != nil {
			return zero, e
		}
		if lo <= 0 && hi <= 0 {
			out.Contacts = append(out.Contacts, event)
		}
		if event.Touch {
			continue
		}
		if event.Entry {
			if start != nil {
				return zero, geom.ErrConvergence
			}
			start = event.Parameter
			continue
		}
		if start == nil {
			return zero, geom.ErrConvergence
		}
		span := [2]*geom.ExactAlgebraicRoot{start, event.Parameter}
		before, e := cmp(span[0], limit[0])
		if e != nil {
			return zero, e
		}
		if before < 0 {
			span[0] = limit[0]
		}
		if hi > 0 {
			span[1] = limit[1]
		}
		order, e := cmp(span[0], span[1])
		if e != nil {
			return zero, e
		}
		if order <= 0 {
			out.Material = append(out.Material, span)
		}
		start = nil
	}
	if start != nil {
		return zero, geom.ErrConvergence
	}
	out.ExactOperations, out.Subdivisions = opts.MaxOperations-b.work, opts.MaxSubdivisions-b.subdiv
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
