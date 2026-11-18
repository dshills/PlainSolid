//go:build research

package trimmed

import (
	"container/heap"
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicGraphBoundaryDistance encloses the squared distance to the
// retained graph material boundary. A witness either names an original
// Boolean face or names the original section edge containing a verified
// point in Witness. On budget exhaustion the enclosing interval may remain
// valid but wider than the requested gap.
type AlgebraicGraphBoundaryDistance struct {
	Squared                       geom.ExactParameterInterval
	Witness                       geom.Box
	Face, RimEdge                 int
	ExactOperations, VisitedCells int
	Converged                     bool
}

// BoundBoundaryDistance bounds global distance to the prepared common's
// retained boundary. Native loop membership rejects buried support points
// and holes. It reuses exact original tube enclosures, uses support hulls
// for cell lower bounds, and keeps unresolved cells in the global bound.
func (q *AlgebraicGraphMaterialQuery) BoundBoundaryDistance(ctx context.Context, point [3]*big.Rat,
	o AlgebraicTubeLensDistanceOptions) (out AlgebraicGraphBoundaryDistance, err error) {
	out.Face, out.RimEdge = -1, -1
	if ctx == nil || q == nil || q.graphs[0] == nil || o.MaxDepth < 0 || o.MaxDepth > 40 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 64000000
	}
	if o.Patch.MaxCoefficientBits == 0 {
		o.Patch.MaxCoefficientBits = 16384
	}
	if o.SquaredGap == nil {
		o.SquaredGap = big.NewRat(1, 1000000)
	}
	if o.SquaredGap.Sign() <= 0 {
		return out, geom.ErrInvalid
	}
	exact, e := graphMaterialLimits(geom.ExactCurvePlaneEventOptions{
		MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
	})
	if e != nil {
		return out, e
	}
	for _, x := range [4]*big.Rat{point[0], point[1], point[2], o.SquaredGap} {
		if x == nil {
			return out, geom.ErrInvalid
		}
		if x.Num().BitLen() > exact.MaxCoefficientBits || x.Denom().BitLen() > exact.MaxCoefficientBits {
			return out, geom.ErrConvergence
		}
	}
	remaining := func() geom.ExactCurvePlaneEventOptions { p := exact; p.MaxOperations -= out.ExactOperations; return p }
	queue := tubeLensDistanceHeap{}
	for chart, patch := range q.patches {
		b, e := patch.Bounds()
		if e != nil {
			return out, e
		}
		lo, _, e := lensExactBoxDistance(point, b)
		if e != nil {
			return out, e
		}
		heap.Push(&queue, tubeLensDistanceCell{patch: patch, chart: chart, lower: lo})
	}
	var pending *big.Rat
	finish := func(e error, lower *big.Rat) (AlgebraicGraphBoundaryDistance, error) {
		if c := ctx.Err(); c != nil {
			return AlgebraicGraphBoundaryDistance{Face: -1, RimEdge: -1}, c
		}
		if len(queue) != 0 {
			lower = minLensDistance(lower, queue[0].lower)
		}
		if pending != nil {
			lower = minLensDistance(lower, pending)
		}
		if out.Squared.Upper != nil && (lower == nil || out.Squared.Upper.Cmp(lower) < 0) {
			lower = out.Squared.Upper
		}
		if lower != nil {
			out.Squared.Lower = new(big.Rat).Set(lower)
		}
		if out.Squared.Upper != nil && lower != nil && new(big.Rat).Sub(out.Squared.Upper, lower).Cmp(o.SquaredGap) <= 0 {
			out.Converged = true
			if errors.Is(e, geom.ErrConvergence) {
				e = nil
			}
		}
		return out, e
	}
	accept := func(value *big.Rat, witness geom.Box, face, rim int) {
		if out.Squared.Upper == nil || value.Cmp(out.Squared.Upper) < 0 {
			out.Squared.Upper, out.Witness, out.Face, out.RimEdge = value, witness, face, rim
		}
	}
	// If the original query fiber is retained, both exact endpoints give
	// immediate boundary witnesses. In particular a perpendicular cap
	// projection remains exact even when no subdivision midpoint hits it.
	fiber, e := q.locateRationalGraphMaterial(ctx, point, remaining(), true)
	out.ExactOperations += fiber.ExactOperations
	if e != nil {
		return finish(e, nil)
	}
	if fiber.Lower != nil && fiber.Upper != nil {
		ends := [2]*big.Rat{fiber.Lower, fiber.Upper}
		faces := fiber.Faces
		if fiber.Lower.Cmp(fiber.Upper) != 0 {
			for _, reg := range q.regions {
				if reg.faces == faces && reg.upper == 0 {
					faces[0], faces[1] = faces[1], faces[0]
					break
				}
			}
		}
		for i, z := range ends {
			w := point
			w[q.residual] = z
			var low, high [3]float64
			for j, x := range w {
				r, e := geom.RationalOutwardRange(x)
				if e != nil || !finite(r.Min) || !finite(r.Max) {
					return finish(geom.ErrConvergence, nil)
				}
				low[j], high[j] = r.Min, r.Max
			}
			box := geom.Box{Min: geom.Vec3{X: low[0], Y: low[1], Z: low[2]}, Max: geom.Vec3{X: high[0], Y: high[1], Z: high[2]}}
			d := new(big.Rat).Sub(z, point[q.residual])
			face, rim := faces[i], -1
			if len(fiber.RimEdges) != 0 {
				face, rim = -1, fiber.RimEdges[0]
			}
			accept(d.Mul(d, d), box, face, rim)
		}
		if new(big.Rat).Sub(out.Squared.Upper, queue[0].lower).Cmp(o.SquaredGap) <= 0 {
			return finish(nil, nil)
		}
	}
	var rims [2][]geom.ParameterRectangle
	for i, ring := range q.loops {
		for j, piece := range ring {
			if e := ctx.Err(); e != nil {
				return finish(e, nil)
			}
			p, e := piece.Segment.Evaluate(0)
			if e != nil || !p.Enclosed {
				if e == nil {
					e = geom.ErrConvergence
				}
				return finish(e, nil)
			}
			if p.ExactOperations > remaining().MaxOperations {
				return finish(geom.ErrConvergence, nil)
			}
			out.ExactOperations += p.ExactOperations
			_, hi, e := lensExactBoxDistance(point, p.Bounds)
			if e != nil {
				return finish(e, nil)
			}
			accept(hi, p.Bounds, -1, q.edges[i][j])
			for chart, t := range [2]intersect.TubeTrim{piece.Segment.TrimA(), piece.Segment.TrimB()} {
				if chart == 0 && len(q.windowTubeBoxes) != 0 {
					rims[chart] = append(rims[chart], q.windowTubeBoxes[i][j]...)
					continue
				}
				b, e := t.BoundsUV()
				if e != nil {
					return finish(e, nil)
				}
				rims[chart] = append(rims[chart], geom.ParameterRectangle{UMin: b.Min.X, UMax: b.Max.X, VMin: b.Min.Y, VMax: b.Max.Y})
			}
		}
	}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return finish(e, nil)
		}
		if out.VisitedCells >= o.MaxCells || remaining().MaxOperations < 1 {
			return finish(geom.ErrConvergence, nil)
		}
		low := queue[0].lower
		if pending != nil {
			low = minLensDistance(low, pending)
		}
		if out.Squared.Upper != nil && new(big.Rat).Sub(out.Squared.Upper, low).Cmp(o.SquaredGap) <= 0 {
			return finish(nil, nil)
		}
		x := heap.Pop(&queue).(tubeLensDistanceCell)
		out.VisitedCells++
		if out.Squared.Upper != nil && x.lower.Cmp(out.Squared.Upper) >= 0 {
			continue
		}
		d := x.patch.Domain()
		mid := func(a, b float64) *big.Rat {
			return new(big.Rat).Quo(new(big.Rat).Add(ratInset(a), ratInset(b)), big.NewRat(2, 1))
		}
		p := remaining()
		jet, e := x.patch.EvaluateRational(ctx, mid(d.UMin, d.UMax), mid(d.VMin, d.VMax),
			geom.SurfacePatchEvaluationOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
		out.ExactOperations += jet.ExactOperations
		if e != nil {
			return finish(e, x.lower)
		}
		if remaining().MaxOperations < 1 {
			return finish(geom.ErrConvergence, x.lower)
		}
		loc, e := q.locateRationalGraphMaterial(ctx, jet.Point, remaining(), true)
		out.ExactOperations += loc.ExactOperations
		if e != nil {
			return finish(e, x.lower)
		}
		if loc.Complete && loc.Faces[x.chart] >= 0 {
			value := new(big.Rat)
			for i, axis := range jet.Point {
				diff := new(big.Rat).Sub(axis, point[i])
				value.Add(value, diff.Mul(diff, diff))
			}
			if len(loc.RimEdges) != 0 {
				accept(value, jet.PointBounds, -1, loc.RimEdges[0])
			} else {
				accept(value, jet.PointBounds, loc.Faces[x.chart], -1)
			}
		} else if loc.Complete {
			hit := false
			for _, b := range rims[x.chart] {
				if b.UMin <= d.UMax && b.UMax >= d.UMin && b.VMin <= d.VMax && b.VMax >= d.VMin {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if x.depth >= o.MaxDepth {
			pending = minLensDistance(pending, x.lower)
			continue
		}
		a, b := d, d
		if d.UMax-d.UMin >= d.VMax-d.VMin {
			m := d.UMin/2 + d.UMax/2
			a.UMax, b.UMin = m, m
		} else {
			m := d.VMin/2 + d.VMax/2
			a.VMax, b.VMin = m, m
		}
		for _, child := range [2]geom.ParameterRectangle{a, b} {
			if !child.Valid() || remaining().MaxOperations < 1 {
				return finish(geom.ErrConvergence, x.lower)
			}
			p, used, e := x.patch.TrimWithWorkLimit(child, remaining().MaxOperations)
			out.ExactOperations += used
			if e != nil {
				return finish(e, x.lower)
			}
			box, e := p.Bounds()
			if e != nil {
				return finish(e, x.lower)
			}
			lo, _, e := lensExactBoxDistance(point, box)
			if e != nil {
				return finish(e, x.lower)
			}
			if lo.Cmp(x.lower) < 0 {
				lo = x.lower
			}
			if out.Squared.Upper == nil || lo.Cmp(out.Squared.Upper) < 0 {
				heap.Push(&queue, tubeLensDistanceCell{patch: p, chart: x.chart, depth: x.depth + 1, lower: lo})
			}
		}
	}
	return finish(geom.ErrConvergence, nil)
}
