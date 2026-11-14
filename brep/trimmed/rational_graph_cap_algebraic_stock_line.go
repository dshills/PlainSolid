package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphStockPlanarLineContact is a transverse intersection with
// the finite material shell. The point and parameter are exact rationals;
// IndexedFace identifies its source face or generated cap.
type AlgebraicGraphStockPlanarLineContact struct {
	Parameter   *big.Rat
	Point       [3]*big.Rat
	IndexedFace int
	Entry       bool
}
type AlgebraicGraphStockPlanarLine struct {
	Contacts []AlgebraicGraphStockPlanarLineContact
	Material [][2]*big.Rat
}

// IntersectPlanarStockRationalLine exactly intersects an arbitrary rational
// line with the closed clipped stock when every retained original support
// is planar. It tests each intersection against original chart and the two
// halfspaces, so an unretained support is not a material event. Edge,
// vertex, coincident, and nonaffine original chart contacts fail closed.
func (c AlgebraicGraphCapPair) IntersectPlanarStockRationalLine(ctx context.Context, origin, direction [3]*big.Rat, extent [2]*big.Rat,
	o RationalGraphShellSectionOptions, options geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockPlanarLine, error) {
	var zero AlgebraicGraphStockPlanarLine
	budget, e := newCapClipBudget(ctx, options)
	if e != nil {
		return zero, e
	}
	bits := budget.bits
	for _, row := range [][]*big.Rat{origin[:], direction[:], extent[:]} {
		for _, p := range row {
			if p == nil {
				return zero, geom.ErrInvalid
			}
			if p.Num().BitLen() > bits || p.Denom().BitLen() > bits {
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
	index, e := c.exactIndexedBoundary(o, &budget)
	if e != nil {
		return zero, e
	}
	var events []AlgebraicGraphStockPlanarLineContact
	for fi, face := range index.Faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		var plane polynomialPlaneEquation
		var chart *geom.AffineSurfaceChart
		if face.Cap >= 0 {
			plane = c.caps[face.Cap].regions.plane
		} else {
			chart, plane, e = c.planarStockChart(face.StockFace, o, &budget)
			if e != nil {
				return zero, e
			}
		}
		slope := plane.dot(direction)
		shift := new(big.Rat).Sub(plane.offset, plane.dot(origin))
		if slope.Sign() == 0 {
			if shift.Sign() == 0 {
				return zero, geom.ErrConvergence
			}
			continue
		}
		t := new(big.Rat).Quo(shift, slope)
		if t.Num().BitLen() > bits || t.Denom().BitLen() > bits {
			return zero, geom.ErrConvergence
		}
		var xyz [3]*big.Rat
		for i := range xyz {
			xyz[i] = new(big.Rat).Add(origin[i], new(big.Rat).Mul(direction[i], t))
			if xyz[i].Num().BitLen() > bits || xyz[i].Denom().BitLen() > bits {
				return zero, geom.ErrConvergence
			}
		}
		// Check all actual cap roles only once; a multi-component plane
		// requires an explicit exact region owner for each projection.
		if face.Cap >= 0 {
			if face.CapRegion != 0 {
				return zero, geom.ErrUnrepresentable
			}
			where, e := c.caps[face.Cap].locateRationalBudget(xyz, &budget)
			if e != nil {
				return zero, e
			}
			if where == trimvalidate.Outside {
				continue
			}
			if where != trimvalidate.Inside {
				return zero, geom.ErrConvergence
			}
		} else {
			uv, e := chart.InvertExactContext(ctx, xyz, geom.AffineChartInverseOptions{MaxOperations: budget.work, MaxCoefficientBits: bits})
			if e != nil {
				return zero, e
			}
			if e := budget.charge(uv.ExactOperations, 0); e != nil {
				return zero, e
			}
			if !uv.OnPlane {
				return zero, geom.ErrConvergence
			}
			d := chart.Domain()
			coords := [2]*big.Rat{uv.U, uv.V}
			in, border := true, false
			for i, span := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
				a, z := coords[i].Cmp(ratInset(span[0])), coords[i].Cmp(ratInset(span[1]))
				in = in && a >= 0 && z <= 0
				border = border || a == 0 || z == 0
			}
			if !in {
				continue
			}
			retained := true
			for _, cap := range c.caps {
				p := cap.regions.plane
				sign := p.dot(xyz).Cmp(p.offset) * p.orientation
				if sign > 0 {
					retained = false
					break
				}
				border = border || sign == 0
			}
			if !retained {
				continue
			}
			if border {
				return zero, geom.ErrConvergence
			}
		}
		events = append(events, AlgebraicGraphStockPlanarLineContact{Parameter: t, Point: xyz, IndexedFace: fi, Entry: slope.Sign()*plane.orientation < 0})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Parameter.Cmp(events[j].Parameter) < 0 })
	if len(events)%2 != 0 {
		return zero, geom.ErrConvergence
	}
	var out AlgebraicGraphStockPlanarLine
	for i, event := range events {
		if event.Entry != (i%2 == 0) || i > 0 && events[i-1].Parameter.Cmp(event.Parameter) >= 0 {
			return zero, geom.ErrConvergence
		}
		if event.Parameter.Cmp(extent[0]) >= 0 && event.Parameter.Cmp(extent[1]) <= 0 {
			out.Contacts = append(out.Contacts, event)
		}
		if i%2 == 0 {
			continue
		}
		a, z := events[i-1].Parameter, event.Parameter
		if extent[0].Cmp(a) > 0 {
			a = extent[0]
		}
		if extent[1].Cmp(z) < 0 {
			z = extent[1]
		}
		if a.Cmp(z) <= 0 {
			out.Material = append(out.Material, [2]*big.Rat{new(big.Rat).Set(a), new(big.Rat).Set(z)})
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}

func (c AlgebraicGraphCapPair) planarStockChart(id int, o RationalGraphShellSectionOptions, budget *capClipBudget) (*geom.AffineSurfaceChart, polynomialPlaneEquation, error) {
	var zero polynomialPlaneEquation
	ctx := budget.ctx
	src := c.caps[0].recipe.stock.faces[id]
	p, e := exactFaceSupportPatch(src, o.Spans.Faces.Patches)
	if e != nil {
		return nil, zero, e
	}
	d, e := faceUVBounds(src)
	if e != nil {
		return nil, zero, e
	}
	p, e = p.Trim(d)
	if e != nil {
		return nil, zero, e
	}
	work, width := o.Spans.Faces.Patches.MaxOperations, o.Spans.Faces.Patches.MaxCoefficientBits
	if work == 0 || work > budget.work {
		work = budget.work
	}
	if width == 0 || width > budget.bits {
		width = budget.bits
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{
		MaxOperations: work, MaxCoefficientBits: width,
	})
	if e != nil {
		return nil, zero, e
	}
	if e := budget.charge(chart.ExactOperations(), 0); e != nil {
		return nil, zero, e
	}
	// The affine proof already establishes a single plane. Its
	// normal follows its two exact chart directions and face sense.
	domain := chart.Domain()
	var corners [3]geom.Vec3
	for i, uv := range [3][2]float64{{domain.UMin, domain.VMin}, {domain.UMax, domain.VMin}, {domain.UMin, domain.VMax}} {
		p, e := chart.EvalExact(ratInset(uv[0]), ratInset(uv[1]))
		if e != nil {
			return nil, zero, e
		}
		corners[i], e = geom.RepresentedExactPoint(p)
		if e != nil {
			return nil, zero, e
		}
	}
	if src.Reversed {
		corners[1], corners[2] = corners[2], corners[1]
	}
	plane, e := polynomialPlaneFromPoints(corners)
	if e != nil {
		return nil, zero, e
	}
	return chart, plane, nil
}
