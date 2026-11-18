//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AlgebraicGraphStockDistance is the exact minimum squared distance to a
// closed material boundary. Exactly one of Face or Edge identifies an
// indexed owner, with -1 for the other. An edge witness retains the original
// cell parameter. Point is present on an exactly projected face interior.
// Multiple owners may share the same minimum.
type AlgebraicGraphStockDistance struct {
	SquaredDistance, CurveParameter *geom.ExactAlgebraicRoot
	Point                           [3]*big.Rat
	Face, Edge                      int
	ExactOperations, Subdivisions   int
}

// NearestPlanarStockRational proves the finite boundary minimum over the
// original affine stock charts and generated caps. All algebraic edges
// are considered with their actual clipped domains; a projection outside
// the retained face contributes no interior candidate. Spatially curved
// stock walls or unsupported edge degrees fail atomically.
func (c AlgebraicGraphCapPair) NearestPlanarStockRational(ctx context.Context, point [3]*big.Rat,
	options RationalGraphShellSectionOptions, limits geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockDistance, error) {
	var zero AlgebraicGraphStockDistance
	b, e := newCapClipBudget(ctx, limits)
	if e != nil {
		return zero, e
	}
	initialWork, initialSplits := b.work, b.subdiv
	for _, p := range point {
		if p == nil {
			return zero, geom.ErrInvalid
		}
		if max(p.Num().BitLen(), p.Denom().BitLen()) > b.bits {
			return zero, geom.ErrConvergence
		}
	}
	boundary, edges, e := c.exactModelBoundary(options, &b)
	if e != nil {
		return zero, e
	}
	out := AlgebraicGraphStockDistance{Face: -1, Edge: -1}
	accept := func(q AlgebraicGraphStockDistance) error {
		if out.SquaredDistance != nil {
			order, e := q.SquaredDistance.Compare(ctx, out.SquaredDistance, b.limits())
			if e != nil {
				return e
			}
			if e := b.charge(order.Operations, order.Subdivisions); e != nil {
				return e
			}
			if !order.Proven {
				return geom.ErrConvergence
			}
			if order.Order >= 0 {
				return nil
			}
		}
		out = q
		return nil
	}
	for i, f := range boundary.Faces {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		var chart *geom.AffineSurfaceChart
		var plane polynomialPlaneEquation
		if f.Cap >= 0 {
			if f.CapRegion != 0 {
				return zero, geom.ErrUnrepresentable
			}
			plane = c.caps[f.Cap].regions.plane
		} else {
			chart, plane, e = c.planarStockChart(f.StockFace, options, &b)
			if e != nil {
				return zero, e
			}
		}
		norm := new(big.Rat)
		for _, n := range plane.normal {
			norm.Add(norm, new(big.Rat).Mul(n, n))
		}
		delta := new(big.Rat).Sub(plane.dot(point), plane.offset)
		scale := new(big.Rat).Quo(delta, norm)
		var witness [3]*big.Rat
		for j, p := range point {
			witness[j] = new(big.Rat).Sub(p, new(big.Rat).Mul(scale, plane.normal[j]))
			if max(witness[j].Num().BitLen(), witness[j].Denom().BitLen()) > b.bits {
				return zero, geom.ErrConvergence
			}
		}
		if f.Cap >= 0 {
			loc, e := c.caps[f.Cap].locateRationalBudget(witness, &b)
			if e != nil {
				return zero, e
			}
			if loc != trimvalidate.Inside {
				continue
			}
		} else {
			q, e := chart.InvertExactContext(ctx, witness, geom.AffineChartInverseOptions{MaxOperations: b.work, MaxCoefficientBits: b.bits})
			if e != nil {
				return zero, e
			}
			if e := b.charge(q.ExactOperations, 0); e != nil {
				return zero, e
			}
			d := chart.Domain()
			if !q.Inside || q.U.Cmp(ratInset(d.UMin)) == 0 || q.U.Cmp(ratInset(d.UMax)) == 0 ||
				q.V.Cmp(ratInset(d.VMin)) == 0 || q.V.Cmp(ratInset(d.VMax)) == 0 {
				continue
			}
			strict := true
			for _, cap := range c.caps {
				p := cap.regions.plane
				strict = strict && p.dot(witness).Cmp(p.offset)*p.orientation < 0
			}
			if !strict {
				continue
			}
		}
		value, used, e := geom.NewExactRationalRoot(ctx, new(big.Rat).Mul(delta, scale), b.limits())
		if e != nil {
			return zero, e
		}
		if e := b.charge(used, 0); e != nil {
			return zero, e
		}
		if e := accept(AlgebraicGraphStockDistance{SquaredDistance: value, Point: witness, Face: i, Edge: -1}); e != nil {
			return zero, e
		}
	}
	for i, edge := range edges {
		for _, cell := range edge.Cells.Cells {
			values, e := cell.RationalQuadraticDistanceCandidates(ctx, point, b.limits())
			if e != nil {
				return zero, e
			}
			if e := b.charge(values.Operations, values.Subdivisions); e != nil {
				return zero, e
			}
			for _, x := range values.Candidates {
				if e := accept(AlgebraicGraphStockDistance{SquaredDistance: x.SquaredDistance, CurveParameter: x.Parameter, Face: -1, Edge: i}); e != nil {
					return zero, e
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if out.SquaredDistance == nil {
		return zero, geom.ErrDomain
	}
	out.ExactOperations, out.Subdivisions = initialWork-b.work, initialSplits-b.subdiv
	return out, nil
}
