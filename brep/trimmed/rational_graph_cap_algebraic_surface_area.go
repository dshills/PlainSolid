//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockArea encloses the total exact boundary area, retaining
// one area interval per indexed face. The indexed faces are returned to make
// cap, original support and fragment ancestry explicit.
type AlgebraicGraphStockArea struct {
	Interval                      geom.ExactParameterInterval
	Faces                         []geom.ExactParameterInterval
	Boundary                      AlgebraicGraphShellBoundary
	ExactOperations, Subdivisions int
}

// BoundPlanarSurfaceArea measures each retained planar face by a signed Green
// integral over its exact model edges. Opposed uses of an original or shared
// cap edge share one authoritative model curve. Original stock supports must
// be affine; caps and all current rational edge models are accepted.
func (c AlgebraicGraphCapPair) BoundPlanarSurfaceArea(ctx context.Context, width *big.Rat,
	o RationalGraphShellSectionOptions, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockArea, error) {
	var zero AlgebraicGraphStockArea
	b, e := newCapClipBudget(ctx, opts)
	if e != nil {
		return zero, e
	}
	if width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	measureFits := func(q *big.Rat) bool { return max(q.Num().BitLen(), q.Denom().BitLen()) <= b.bits }
	if !measureFits(width) {
		return zero, geom.ErrConvergence
	}
	work, splits := b.work, b.subdiv
	index, model, e := c.exactModelBoundary(o, &b)
	if e != nil {
		return zero, e
	}
	if len(index.Faces) == 0 {
		return zero, geom.ErrDomain
	}
	var stockHull [3]*big.Rat
	for i := range stockHull {
		stockHull[i] = new(big.Rat)
	}
	// Include the outer-rounded original rational control hull. Cap
	// projections can cross curved charts even when the retained
	// original supports for this particular operation are affine.
	for _, f := range c.caps[0].recipe.stock.faces {
		p, e := exactFaceSupportPatch(f, o.Spans.Faces.Patches)
		if e != nil {
			return zero, e
		}
		box, e := p.Bounds()
		if e != nil {
			return zero, e
		}
		for _, corner := range [2]geom.Vec3{box.Min, box.Max} {
			for axis, x := range exactBoundPoint(corner) {
				q := new(big.Rat).Abs(x)
				if q.Cmp(stockHull[axis]) > 0 {
					stockHull[axis].Set(q)
				}
			}
		}
		if e := b.charge(12, 0); e != nil {
			return zero, e
		}
	}
	partWidth := new(big.Rat).Quo(width, big.NewRat(int64(len(index.Faces)), 1))
	out := AlgebraicGraphStockArea{Boundary: index, Interval: geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}}
	for _, face := range index.Faces {
		var plane polynomialPlaneEquation
		if face.Cap >= 0 {
			plane = c.caps[face.Cap].regions.plane
		} else {
			_, plane, e = c.planarStockChart(face.StockFace, o, &b)
			if e != nil {
				return zero, e
			}
		}
		scale := new(big.Rat)
		for _, n := range plane.normal {
			scale.Add(scale, new(big.Rat).Mul(n, n))
		}
		axes := orientedRationalGraphProjection(plane)
		count := 0
		hull := new(big.Rat)
		for _, axis := range axes {
			if stockHull[axis].Cmp(hull) > 0 {
				hull.Set(stockHull[axis])
			}
		}
		geomBound := new(big.Rat).Mul(hull, hull)
		geomBound.Mul(geomBound, big.NewRat(4, 1)).Add(geomBound, big.NewRat(1, 1))
		scaleLo, scaleHi, e := stockAreaProjectionScale(scale, geomBound, partWidth, &b)
		if e != nil {
			return zero, e
		}
		for _, loop := range face.Loops {
			for _, use := range loop {
				count += len(model[use.Edge].Cells.Cells)
			}
		}
		if count < 1 {
			return zero, geom.ErrConvergence
		}
		edgeWidth := new(big.Rat).Quo(partWidth, new(big.Rat).Mul(scaleHi, big.NewRat(int64(4*count), 1)))
		lo, hi := new(big.Rat), new(big.Rat)
		for _, loop := range face.Loops {
			for _, use := range loop {
				edge := model[use.Edge]
				for _, cell := range edge.Cells.Cells {
					r, e := cell.BoundProjectedGreen(ctx, axes, edgeWidth, b.limits())
					if e != nil {
						return zero, e
					}
					if e = b.charge(r.Operations, r.Subdivisions); e != nil {
						return zero, e
					}
					if use.Reversed != edge.Reversed {
						lo.Sub(lo, r.Interval.Upper)
						hi.Sub(hi, r.Interval.Lower)
					} else {
						lo.Add(lo, r.Interval.Lower)
						hi.Add(hi, r.Interval.Upper)
					}
					if !measureFits(lo) || !measureFits(hi) {
						return zero, geom.ErrConvergence
					}
				}
			}
		}
		if lo.Sign() < 0 {
			lo.SetInt64(0)
		}
		lo.Mul(lo, scaleLo)
		hi.Mul(hi, scaleHi)
		if hi.Cmp(lo) < 0 || new(big.Rat).Sub(hi, lo).Cmp(partWidth) > 0 || !measureFits(lo) || !measureFits(hi) {
			return zero, geom.ErrConvergence
		}
		out.Faces = append(out.Faces, geom.ExactParameterInterval{Lower: lo, Upper: hi})
		out.Interval.Lower.Add(out.Interval.Lower, lo)
		out.Interval.Upper.Add(out.Interval.Upper, hi)
		if !graphStockMomentFits(out.Interval, b.bits) {
			return zero, geom.ErrConvergence
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}

// Refine the positive square-root scale so its width consumes at most a
// quarter of this cap or face's allowed area enclosure.
func stockAreaProjectionScale(scale, geometryBound, width *big.Rat, b *capClipBudget) (*big.Rat, *big.Rat, error) {
	for _, q := range []*big.Rat{scale, geometryBound, width} {
		if max(q.Num().BitLen(), q.Denom().BitLen()) > b.bits {
			return nil, nil, geom.ErrConvergence
		}
	}
	prec := min(128, b.bits-2)
	for {
		if e := b.charge(prec, 0); e != nil {
			return nil, nil, e
		}
		lo, hi, e := graphAreaSqrt(scale, prec, b.bits)
		if e != nil {
			return nil, nil, e
		}
		if new(big.Rat).Mul(geometryBound, new(big.Rat).Sub(hi, lo)).Cmp(new(big.Rat).Quo(width, big.NewRat(4, 1))) <= 0 {
			return lo, hi, nil
		}
		if prec >= min(256, b.bits-1) {
			return nil, nil, geom.ErrConvergence
		}
		prec = min(2*prec, 256, b.bits-1)
	}
}
