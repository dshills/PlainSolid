//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicCapLineOwner is one original cap use meeting a line point.
// The parameter is exact on that use's immutable curve cell.
type AlgebraicCapLineOwner struct {
	Wire, Use      int
	CurveParameter *geom.ExactAlgebraicRoot
}

// AlgebraicCapCoveragePoint reports the original owners of one trim
// contact or an endpoint of a clipped coincident run. A caller window
// limit without a trim owner does not become a point.
type AlgebraicCapCoveragePoint struct {
	Parameter *geom.ExactAlgebraicRoot
	Owners    []AlgebraicCapLineOwner
}

// AlgebraicGraphCapCoverage partitions a finite line into the closures of
// open interior spans and original coincident boundary runs. Points reports
// zero-dimensional trim contacts too. The line's closed material is the
// union of those three sets. Interior spans do not cross a boundary point.
type AlgebraicGraphCapCoverage struct {
	Interior                      [][2]*geom.ExactAlgebraicRoot
	Boundary                      []AlgebraicGraphCapBoundaryRun
	Points                        []AlgebraicCapCoveragePoint
	ExactOperations, Subdivisions int
}

// CoverRationalLine proves the complete intersection of a finite rational
// line with a cap, including coexistence of point contacts and coincident
// boundary runs. Exact curve roots partition the line before interior
// classification. This method does not require regular crossing germs.
func (c AlgebraicGraphCapClip) CoverRationalLine(ctx context.Context, origin, direction [3]*big.Rat,
	extent [2]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphCapCoverage, error) {
	var zero AlgebraicGraphCapCoverage
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
	if extent[0].Cmp(extent[1]) >= 0 || cap.dot(origin).Cmp(cap.offset) != 0 || cap.dot(direction).Sign() != 0 {
		return zero, geom.ErrDomain
	}
	axis := -1
	for _, a := range orientedRationalGraphProjection(cap) {
		if direction[a].Sign() != 0 {
			axis = a
			break
		}
	}
	if axis < 0 {
		return zero, geom.ErrDegenerate
	}
	type knot struct {
		root   *geom.ExactAlgebraicRoot
		owners []AlgebraicCapLineOwner
		delta  int
	}
	var knots []knot
	cmp := func(x, y *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		v, e := x.Compare(ctx, y, b.limits())
		if e != nil {
			return v, e
		}
		if e = b.charge(v.Operations, v.Subdivisions); e != nil {
			return v, e
		}
		if !v.Proven {
			return v, geom.ErrConvergence
		}
		return v, nil
	}
	insert := func(r *geom.ExactAlgebraicRoot, owner *AlgebraicCapLineOwner, delta int) error {
		lo, hi := 0, len(knots)
		for lo < hi {
			j := lo + (hi-lo)/2
			q, e := cmp(knots[j].root, r)
			if e != nil {
				return e
			}
			if q.Order == 0 {
				if owner != nil {
					knots[j].owners = append(knots[j].owners, *owner)
				}
				knots[j].delta += delta
				return nil
			}
			if q.Order < 0 {
				lo = j + 1
			} else {
				hi = j
			}
		}
		k := knot{root: r, delta: delta}
		if owner != nil {
			k.owners = []AlgebraicCapLineOwner{*owner}
		}
		knots = append(knots, knot{})
		copy(knots[lo+1:], knots[lo:])
		knots[lo] = k
		return nil
	}
	for _, t := range extent {
		r, work, e := geom.NewExactRationalRoot(ctx, t, b.limits())
		if e != nil {
			return zero, e
		}
		if e = b.charge(work, 0); e != nil {
			return zero, e
		}
		if e = insert(r, nil, 0); e != nil {
			return zero, e
		}
	}
	var out AlgebraicGraphCapCoverage
	scale := new(big.Rat).Inv(direction[axis])
	offset := new(big.Rat).Neg(new(big.Rat).Mul(scale, origin[axis]))
	image := func(ev *geom.ExactComposedPlaneEvent, known *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		if known != nil {
			r, work, e := geom.NewExactRationalRoot(ctx, known, b.limits())
			if e != nil {
				return nil, e
			}
			return r, b.charge(work, 0)
		}
		r, work, steps, e := ev.CoordinateRootWithWork(ctx, axis, b.limits())
		if e != nil {
			return nil, e
		}
		if e = b.charge(work, steps); e != nil {
			return nil, e
		}
		return b.mapRoot(r, offset, scale)
	}
	for wi, wire := range c.wires {
		for ui, use := range wire {
			finite, e := use.Cell.IntersectRationalFiniteLine(ctx, origin, direction, extent, geom.ComposedCurveLineOptions{ExactCurvePlaneEventOptions: b.limits()})
			if e != nil {
				return zero, e
			}
			if e = b.charge(finite.Operations, finite.Subdivisions); e != nil {
				return zero, e
			}
			if !finite.Complete || finite.Coincident && len(finite.Portions)+len(finite.Hits) > 1 {
				return zero, geom.ErrConvergence
			}
			for _, hit := range finite.Hits {
				var endpoint *big.Rat
				if hit.BoundaryMask&1 != 0 {
					endpoint = extent[0]
				} else if hit.BoundaryMask&2 != 0 {
					endpoint = extent[1]
				}
				r, e := image(hit.Event, endpoint)
				if e != nil {
					return zero, e
				}
				if e = insert(r, &AlgebraicCapLineOwner{wi, ui, hit.Event.Root()}, 0); e != nil {
					return zero, e
				}
			}
			for _, part := range finite.Portions {
				if !finite.Coincident {
					return zero, geom.ErrConvergence
				}
				a, e := image(part.Start, nil)
				if e != nil {
					return zero, e
				}
				z, e := image(part.End, nil)
				if e != nil {
					return zero, e
				}
				q, e := cmp(a, z)
				if e != nil || q.Order == 0 {
					if e != nil {
						return zero, e
					}
					return zero, geom.ErrConvergence
				}
				ta, tz := part.Start.Root(), part.End.Root()
				if q.Order > 0 {
					a, z, ta, tz = z, a, tz, ta
				}
				if e = insert(a, &AlgebraicCapLineOwner{wi, ui, ta}, 1); e != nil {
					return zero, e
				}
				if e = insert(z, &AlgebraicCapLineOwner{wi, ui, tz}, -1); e != nil {
					return zero, e
				}
				out.Boundary = append(out.Boundary, AlgebraicGraphCapBoundaryRun{wi, ui, [2]*geom.ExactAlgebraicRoot{a, z},
					[2]*geom.ExactAlgebraicRoot{ta, tz}, (q.Order < 0) != use.Reversed})
			}
		}
	}
	active := 0
	for i, k := range knots {
		active += k.delta
		if active < 0 || active > 1 {
			return zero, geom.ErrConvergence
		}
		if len(k.owners) != 0 {
			out.Points = append(out.Points, AlgebraicCapCoveragePoint{k.root, k.owners})
		}
		if i+1 == len(knots) || active != 0 {
			continue
		}
		z := knots[i+1].root
		q, e := cmp(k.root, z)
		if e != nil {
			return zero, e
		}
		// Exact root order establishes a strict inter-root rational gap.
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 {
			return zero, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		var point [3]*big.Rat
		for j := range point {
			point[j] = new(big.Rat).Add(origin[j], new(big.Rat).Mul(direction[j], t))
		}
		if e = b.charge(12, 0); e != nil {
			return zero, e
		}
		loc, e := c.locateRationalBudget(point, &b)
		if e != nil {
			return zero, e
		}
		switch loc {
		case trimvalidate.Inside:
			out.Interior = append(out.Interior, [2]*geom.ExactAlgebraicRoot{k.root, z})
		case trimvalidate.Outside:
		default:
			return zero, geom.ErrConvergence
		}
	}
	if active != 0 {
		return zero, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = maxWork-b.work, maxSubdiv-b.subdiv
	return out, nil
}
