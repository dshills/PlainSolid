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

type AlgebraicTubeLensDistanceOptions struct {
	Patch              geom.SurfacePatchOptions
	SquaredGap         *big.Rat
	MaxDepth, MaxCells int
}

type AlgebraicTubeLensDistance struct {
	Squared                       geom.ExactParameterInterval
	Witness                       geom.Box
	WitnessChart                  int
	WitnessOnRim                  bool
	VisitedCells, ExactOperations int
	Converged                     bool
}

// BoundBoundaryDistance encloses squared distance to the exact lens boundary,
// using outward spline control hulls for lower bounds and certified
// material samples and rim events for upper bounds. Budget exhaustion can
// preserve valid unequal bounds; Converged means the requested gap is met.
func (l AlgebraicTubeLens) BoundBoundaryDistance(ctx context.Context, q [3]*big.Rat,
	o AlgebraicTubeLensDistanceOptions) (AlgebraicTubeLensDistance, error) {
	return l.queryData().boundBoundaryDistance(ctx, q, l.VerticalInterval, o)
}

// BoundBoundaryDistance encloses distance in original model coordinates.
// Partial bounds remain valid when a cell or arithmetic budget is exhausted.
func (l AlgebraicProjectedTubeLens) BoundBoundaryDistance(ctx context.Context, q [3]*big.Rat,
	o AlgebraicTubeLensDistanceOptions) (AlgebraicTubeLensDistance, error) {
	return l.queryData().boundBoundaryDistance(ctx, q, l.ProjectionInterval, o)
}

func (l tubeLensQuery) boundBoundaryDistance(ctx context.Context, q [3]*big.Rat,
	interval func(context.Context, [2]*big.Rat, geom.ExactCurvePlaneEventOptions) (AlgebraicTubeLensVerticalInterval, error),
	o AlgebraicTubeLensDistanceOptions) (out AlgebraicTubeLensDistance, err error) {
	source := l.section.runSource()
	if ctx == nil || source == nil || q[0] == nil || q[1] == nil || q[2] == nil ||
		o.MaxDepth < 0 || o.MaxDepth > 40 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 20
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
	if o.Patch.MaxOperations < 1 || o.Patch.MaxCoefficientBits < 64 || o.SquaredGap.Sign() <= 0 {
		return out, geom.ErrInvalid
	}
	for _, x := range []*big.Rat{q[0], q[1], q[2], o.SquaredGap} {
		if x.Num().BitLen() > o.Patch.MaxCoefficientBits || x.Denom().BitLen() > o.Patch.MaxCoefficientBits {
			return out, geom.ErrConvergence
		}
	}
	limits := func() geom.SurfacePatchOptions { p := o.Patch; p.MaxOperations -= out.ExactOperations; return p }
	trim := func(p *geom.SurfacePatch, d geom.ParameterRectangle) (*geom.SurfacePatch, error) {
		if p.Domain() == d {
			return p, nil
		}
		n := limits().MaxOperations
		if n < 1 {
			return nil, geom.ErrConvergence
		}
		result, used, e := p.TrimWithWorkLimit(d, n)
		out.ExactOperations += used
		return result, e
	}
	var roots [2]*geom.SurfacePatch
	work := tubeLensDistanceHeap{}
	for i, f := range source.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			return out, e
		}
		p := limits()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		s, used, e := exactLensQueryPatch(f, p)
		out.ExactOperations += used
		if e != nil {
			return out, e
		}
		roots[i], e = trim(s, d)
		if e != nil {
			return out, e
		}
		b, e := roots[i].Bounds()
		if e != nil {
			return out, e
		}
		lo, _, e := lensExactBoxDistance(q, b)
		if e != nil {
			return out, e
		}
		heap.Push(&work, tubeLensDistanceCell{patch: roots[i], chart: i, lower: lo})
	}
	finish := func(e error, unresolved *big.Rat) (AlgebraicTubeLensDistance, error) {
		if len(work) > 0 && (unresolved == nil || work[0].lower.Cmp(unresolved) < 0) {
			unresolved = work[0].lower
		}
		if unresolved == nil || out.Squared.Upper != nil && unresolved.Cmp(out.Squared.Upper) > 0 {
			unresolved = out.Squared.Upper
		}
		out.Squared.Lower = unresolved
		if out.Squared.Upper != nil && unresolved != nil && new(big.Rat).Sub(out.Squared.Upper, unresolved).Cmp(o.SquaredGap) <= 0 {
			out.Converged = true
			if errors.Is(e, geom.ErrConvergence) {
				e = nil
			}
		}
		return out, e
	}
	accept := func(upper *big.Rat, b geom.Box, chart int, rim bool) {
		if out.Squared.Upper == nil || upper.Cmp(out.Squared.Upper) < 0 {
			out.Squared.Upper, out.Witness, out.WitnessChart, out.WitnessOnRim = upper, b, chart, rim
		}
	}
	for _, p := range l.faces[0].Cut {
		k := limits()
		if k.MaxOperations < 1 {
			return finish(geom.ErrConvergence, nil)
		}
		b, e := p.StartPoint.Enclose(ctx, geom.ComposedCurveEndpointOptions{ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{
			MaxOperations: k.MaxOperations, MaxCoefficientBits: k.MaxCoefficientBits}})
		out.ExactOperations += b.ExactOperations
		if e != nil {
			return finish(e, nil)
		}
		_, hi, e := lensExactBoxDistance(q, b.Bounds)
		if e != nil {
			return finish(e, nil)
		}
		accept(hi, b.Bounds, -1, true)
	}
	var rims [2][]geom.UVBox
	for _, p := range l.faces[0].Cut {
		for i, trim := range [2]intersect.TubeTrim{p.Segment.TrimA(), p.Segment.TrimB()} {
			b, e := trim.BoundsUV()
			if e != nil {
				return finish(e, nil)
			}
			rims[i] = append(rims[i], b)
		}
	}
	var unresolved *big.Rat
	for len(work) > 0 {
		if e := ctx.Err(); e != nil {
			return finish(e, unresolved)
		}
		if out.VisitedCells >= o.MaxCells || limits().MaxOperations < 1 {
			return finish(geom.ErrConvergence, unresolved)
		}
		if out.Squared.Upper != nil && new(big.Rat).Sub(out.Squared.Upper, work[0].lower).Cmp(o.SquaredGap) <= 0 &&
			(unresolved == nil || new(big.Rat).Sub(out.Squared.Upper, unresolved).Cmp(o.SquaredGap) <= 0) {
			return finish(nil, unresolved)
		}
		x := heap.Pop(&work).(tubeLensDistanceCell)
		out.VisitedCells++
		if out.Squared.Upper != nil && x.lower.Cmp(out.Squared.Upper) >= 0 {
			continue
		}
		d := x.patch.Domain()
		u := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.UMin), ratInset(d.UMax)), big.NewRat(2, 1))
		v := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.VMin), ratInset(d.VMax)), big.NewRat(2, 1))
		k := limits()
		jet, e := x.patch.EvaluateRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{MaxOperations: k.MaxOperations, MaxCoefficientBits: k.MaxCoefficientBits})
		out.ExactOperations += jet.ExactOperations
		if e != nil {
			return finish(e, minLensDistance(unresolved, x.lower))
		}
		k = limits()
		if k.MaxOperations < 1 {
			return finish(geom.ErrConvergence, minLensDistance(unresolved, x.lower))
		}
		class, e := interval(ctx, [2]*big.Rat{jet.Point[l.axes[0]], jet.Point[l.axes[1]]}, geom.ExactCurvePlaneEventOptions{
			MaxOperations: k.MaxOperations, MaxCoefficientBits: k.MaxCoefficientBits})
		out.ExactOperations += class.ExactOperations
		if e != nil {
			return finish(e, minLensDistance(unresolved, x.lower))
		}
		if !class.Present {
			clear := true
			for _, b := range rims[x.chart] {
				if b.Min.X <= d.UMax && b.Max.X >= d.UMin && b.Min.Y <= d.VMax && b.Max.Y >= d.VMin {
					clear = false
					break
				}
			}
			if clear {
				continue
			}
		} else {
			value := new(big.Rat)
			for i, z := range jet.Point {
				a := new(big.Rat).Sub(z, q[i])
				value.Add(value, new(big.Rat).Mul(a, a))
			}
			accept(value, jet.PointBounds, x.chart, class.Rim)
		}
		if x.depth >= o.MaxDepth {
			unresolved = minLensDistance(unresolved, x.lower)
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
		for _, d := range [2]geom.ParameterRectangle{a, b} {
			if limits().MaxOperations < 1 {
				return finish(geom.ErrConvergence, minLensDistance(unresolved, x.lower))
			}
			p, e := trim(x.patch, d)
			if e != nil {
				return finish(e, minLensDistance(unresolved, x.lower))
			}
			box, e := p.Bounds()
			if e != nil {
				return finish(e, minLensDistance(unresolved, x.lower))
			}
			lo, _, e := lensExactBoxDistance(q, box)
			if e != nil {
				return finish(e, minLensDistance(unresolved, x.lower))
			}
			if lo.Cmp(x.lower) < 0 {
				lo = x.lower
			}
			if out.Squared.Upper == nil || lo.Cmp(out.Squared.Upper) < 0 {
				heap.Push(&work, tubeLensDistanceCell{p, x.chart, x.depth + 1, lo})
			}
		}
	}
	return finish(geom.ErrConvergence, unresolved)
}

func minLensDistance(a, b *big.Rat) *big.Rat {
	if a == nil || b.Cmp(a) < 0 {
		return b
	}
	return a
}

func lensExactBoxDistance(q [3]*big.Rat, b geom.Box) (lo, hi *big.Rat, err error) {
	if !b.Min.IsFinite() || !b.Max.IsFinite() {
		return nil, nil, geom.ErrConvergence
	}
	lo, hi = new(big.Rat), new(big.Rat)
	for i, p := range [3][2]float64{{b.Min.X, b.Max.X}, {b.Min.Y, b.Max.Y}, {b.Min.Z, b.Max.Z}} {
		a, z := new(big.Rat).Sub(ratInset(p[0]), q[i]), new(big.Rat).Sub(ratInset(p[1]), q[i])
		if a.Sign() > 0 {
			lo.Add(lo, new(big.Rat).Mul(a, a))
		} else if z.Sign() < 0 {
			lo.Add(lo, new(big.Rat).Mul(z, z))
		}
		hi.Add(hi, maxRat(new(big.Rat).Mul(a, a), new(big.Rat).Mul(z, z)))
	}
	return lo, hi, nil
}

type tubeLensDistanceCell struct {
	patch        *geom.SurfacePatch
	chart, depth int
	lower        *big.Rat
}
type tubeLensDistanceHeap []tubeLensDistanceCell

func (h tubeLensDistanceHeap) Len() int           { return len(h) }
func (h tubeLensDistanceHeap) Less(i, j int) bool { return h[i].lower.Cmp(h[j].lower) < 0 }
func (h tubeLensDistanceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *tubeLensDistanceHeap) Push(x any)        { *h = append(*h, x.(tubeLensDistanceCell)) }
func (h *tubeLensDistanceHeap) Pop() any {
	a := *h
	x := a[len(a)-1]
	a[len(a)-1] = tubeLensDistanceCell{}
	*h = a[:len(a)-1]
	return x
}
