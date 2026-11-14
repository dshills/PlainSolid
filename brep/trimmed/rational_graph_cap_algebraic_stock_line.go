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
	if ctx == nil || options.MaxOperations < 0 || options.MaxCoefficientBits < 0 || options.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	bits := options.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 {
		return zero, geom.ErrInvalid
	}
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
	index, e := c.ExactIndexedBoundary(ctx, o, options)
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
			src := c.caps[0].recipe.stock.faces[face.StockFace]
			p, e := exactFaceSupportPatch(src, o.Spans.Faces.Patches)
			if e != nil {
				return zero, e
			}
			d, e := faceUVBounds(src)
			if e != nil {
				return zero, e
			}
			p, e = p.Trim(d)
			if e != nil {
				return zero, e
			}
			chart, e = geom.NewAffineSurfaceChart(ctx, p, geom.AffineSurfaceChartOptions{
				MaxOperations: o.Spans.Faces.Patches.MaxOperations, MaxCoefficientBits: o.Spans.Faces.Patches.MaxCoefficientBits,
			})
			if e != nil {
				return zero, e
			}
			plane, e = exactGraphStockFacePlane(ctx, p, src.Reversed, o.Spans.Faces)
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
			where, e := c.caps[face.Cap].LocateRational(ctx, xyz)
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
			uv, e := chart.InvertExactContext(ctx, xyz, geom.AffineChartInverseOptions{MaxCoefficientBits: bits})
			if e != nil {
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
