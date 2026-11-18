//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphCapBoundaryRun binds a coincident finite line portion to
// one original directed cap use. Extent is ordered by the given line
// parameter; CurveParameters[i] maps to the same endpoint as Extent[i].
// AlongWire is true when that order follows the cap wire's direction.
type AlgebraicGraphCapBoundaryRun struct {
	Wire, Use       int
	Extent          [2]*geom.ExactAlgebraicRoot
	CurveParameters [2]*geom.ExactAlgebraicRoot
	AlongWire       bool
}

// AlgebraicGraphCapBoundaryRuns reports each coincident use separately
// in original wire order. It does not merge adjacent original owners.
type AlgebraicGraphCapBoundaryRuns struct {
	Runs                          []AlgebraicGraphCapBoundaryRun
	ExactOperations, Subdivisions int
}

// IntersectRationalBoundaryRuns finds portions of original cap edges that
// coincide with a finite line in the cap plane. Isolated intersections with
// noncoincident edges are outside this overlap report. Mere contact between
// the line window and the endpoint of a coincident use has equal bounds.
func (c AlgebraicGraphCapClip) IntersectRationalBoundaryRuns(ctx context.Context,
	origin, direction [3]*big.Rat, extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapBoundaryRuns, error) {
	var zero AlgebraicGraphCapBoundaryRuns
	if c.regions == nil {
		return zero, geom.ErrInvalid
	}
	b, err := newCapClipBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	maxWork, maxSubdiv := b.work, b.subdiv
	for _, row := range [][]*big.Rat{origin[:], direction[:], extent[:]} {
		for _, x := range row {
			if x == nil {
				return zero, geom.ErrInvalid
			}
			if max(x.Num().BitLen(), x.Denom().BitLen()) > b.bits {
				return zero, geom.ErrConvergence
			}
		}
	}
	cap := c.regions.plane
	if extent[0].Cmp(extent[1]) > 0 || cap.dot(origin).Cmp(cap.offset) != 0 || cap.dot(direction).Sign() != 0 {
		return zero, geom.ErrDomain
	}
	x, y := orientedRationalGraphProjection(cap)[0], orientedRationalGraphProjection(cap)[1]
	axis := x
	if direction[axis].Sign() == 0 {
		axis = y
		if direction[axis].Sign() == 0 {
			return zero, geom.ErrDegenerate
		}
	}
	var plane [4]*big.Rat
	plane[x], plane[y] = new(big.Rat).Neg(direction[y]), new(big.Rat).Set(direction[x])
	plane[3] = new(big.Rat).Sub(new(big.Rat).Mul(direction[y], origin[x]), new(big.Rat).Mul(direction[x], origin[y]))
	scale := new(big.Rat).Inv(direction[axis])
	offset := new(big.Rat).Neg(new(big.Rat).Mul(scale, origin[axis]))
	var out AlgebraicGraphCapBoundaryRuns
	for wi, wire := range c.wires {
		for ui, use := range wire {
			contact, e := use.Cell.IntersectRawPlane(ctx, plane, b.limits())
			if e != nil {
				return zero, e
			}
			if e = b.charge(contact.Operations, contact.Subdivisions); e != nil {
				return zero, e
			}
			if !contact.Complete {
				return zero, geom.ErrConvergence
			}
			if !contact.Coincident {
				continue
			}
			// Each use is an embedded subarc of an original certified cap
			// wire (or a generated segment). An injective continuous arc
			// on a line is monotone. The spatial line solver clips the
			// exact continuation to both independent finite bounds.
			finite, e := use.Cell.IntersectRationalFiniteLine(ctx, origin, direction, extent, geom.ComposedCurveLineOptions{ExactCurvePlaneEventOptions: b.limits()})
			if e != nil {
				return zero, e
			}
			if e = b.charge(finite.Operations, finite.Subdivisions); e != nil {
				return zero, e
			}
			if !finite.Complete || !finite.Coincident || len(finite.Portions)+len(finite.Hits) > 1 {
				return zero, geom.ErrConvergence
			}
			image := func(ev *geom.ExactComposedPlaneEvent) (*geom.ExactAlgebraicRoot, error) {
				root, work, steps, e := ev.CoordinateRootWithWork(ctx, axis, b.limits())
				if e != nil {
					return nil, e
				}
				if e = b.charge(work, steps); e != nil {
					return nil, e
				}
				return b.mapRoot(root, offset, scale)
			}
			appendRun := func(lo, hi *geom.ExactComposedPlaneEvent, point bool) error {
				a, e := image(lo)
				if e != nil {
					return e
				}
				z := a
				if !point {
					z, e = image(hi)
					if e != nil {
						return e
					}
				}
				p, q := lo.Root(), hi.Root()
				// Even for singleton clips retain the direction of the
				// original use by ordering its full image endpoints.
				ends := [2]*geom.ExactAlgebraicRoot{a, z}
				if point {
					for i, ev := range contact.Boundary {
						ends[i], e = image(ev)
						if e != nil {
							return e
						}
					}
				}
				order, e := ends[0].Compare(ctx, ends[1], b.limits())
				if e != nil {
					return e
				}
				if e = b.charge(order.Operations, order.Subdivisions); e != nil {
					return e
				}
				if !order.Proven || order.Order == 0 {
					return geom.ErrConvergence
				}
				if order.Order > 0 {
					a, z, p, q = z, a, q, p
				}
				out.Runs = append(out.Runs, AlgebraicGraphCapBoundaryRun{wi, ui, [2]*geom.ExactAlgebraicRoot{a, z},
					[2]*geom.ExactAlgebraicRoot{p, q}, (order.Order < 0) != use.Reversed})
				return nil
			}
			for _, part := range finite.Portions {
				if e = appendRun(part.Start, part.End, false); e != nil {
					return zero, e
				}
			}
			for _, hit := range finite.Hits {
				if e = appendRun(hit.Event, hit.Event, true); e != nil {
					return zero, e
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations, out.Subdivisions = maxWork-b.work, maxSubdiv-b.subdiv
	return out, nil
}
