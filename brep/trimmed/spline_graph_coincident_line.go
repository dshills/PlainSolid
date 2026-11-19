//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SharedBaseGraphEnvelopeSpan lies on one original graph over an entire
// parameter interval. Its endpoints are rational or exact root brackets.
type SharedBaseGraphEnvelopeSpan struct {
	Start, End geom.ToroidLineRoot
	Base       bool
	TopFaces   []int
}

// CoincidentEnvelopeSpans identifies positive-length intervals where the
// finite line coincides with a retained union or intersection graph. It
// splits top coincidence at all opposing isolated roots and span endpoints.
// The result describes only coincident intervals, not isolated boundary or
// side-wall contacts. Algebraic events overlapping a rational span endpoint
// conservatively leave that query unresolved.
func (p SharedBaseSplineGraphMaterial) CoincidentEnvelopeSpans(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, op parametric.BooleanOp, o geom.ToroidLineOptions) ([]SharedBaseGraphEnvelopeSpan, error) {
	if op != parametric.Union && op != parametric.Intersection {
		return nil, geom.ErrInvalid
	}
	report, e := p.SectionOriginalGraphsLine(ctx, origin, direction, minT, maxT, o)
	if e != nil || report.Clip == nil {
		return nil, e
	}
	root := func(t *big.Rat) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: t, Max: new(big.Rat).Set(t), Multiplicity: 1}
	}
	var spans []SharedBaseGraphEnvelopeSpan
	for _, s := range report.Graph[0].Coincident {
		spans = append(spans, SharedBaseGraphEnvelopeSpan{Start: root(s.Lower), End: root(s.Upper), Base: true})
	}
	oEval := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits}
	x := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}
	d := [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	for i, graph := range [2]*geom.SurfacePatch{p.b, p.a} {
		other := report.Graph[2-i]
		for _, s := range report.Graph[i+1].Coincident {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			cuts := []geom.ToroidLineRoot{root(s.Lower), root(s.Upper)}
			for _, q := range other.Coincident {
				for _, t := range [2]*big.Rat{q.Lower, q.Upper} {
					if t.Cmp(s.Lower) > 0 && t.Cmp(s.Upper) < 0 {
						cuts = append(cuts, root(t))
					}
				}
			}
			for _, h := range other.Hits {
				t := h.Parameter
				if t.Max.Cmp(s.Lower) <= 0 || t.Min.Cmp(s.Upper) >= 0 {
					continue
				}
				if t.Min.Cmp(s.Lower) <= 0 || t.Max.Cmp(s.Upper) >= 0 {
					return nil, geom.ErrConvergence
				}
				cuts = append(cuts, t)
			}
			sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].Min.Cmp(cuts[j].Min) < 0 })
			distinct := cuts[:0]
			for _, q := range cuts {
				if len(distinct) > 0 {
					z := distinct[len(distinct)-1]
					if z.Max.Cmp(q.Min) >= 0 {
						if z.Min.Cmp(z.Max) != 0 || q.Min.Cmp(q.Max) != 0 || z.Min.Cmp(q.Min) != 0 {
							return nil, geom.ErrConvergence
						}
						continue
					}
				}
				distinct = append(distinct, q)
			}
			for j := 1; j < len(distinct); j++ {
				l, h := distinct[j-1], distinct[j]
				t := new(big.Rat).Quo(new(big.Rat).Add(l.Max, h.Min), big.NewRat(2, 1))
				var point [3]*big.Rat
				for k := range point {
					point[k] = new(big.Rat).Add(x[k], new(big.Rat).Mul(d[k], t))
				}
				v, e := graph.EvaluateRational(ctx, point[0], point[1], oEval)
				if e != nil {
					return nil, e
				}
				sign := point[2].Cmp(v.Point[2])
				if sign == 0 {
					// Whole-interval coincidence is checked from the
					// other complete contact report, not from this sample.
					common := false
					for _, span := range other.Coincident {
						common = common || span.Lower.Cmp(l.Min) <= 0 && span.Upper.Cmp(h.Max) >= 0
					}
					if !common {
						return nil, geom.ErrConvergence
					}
					if i == 0 {
						spans = appendEnvelopeSpan(spans, SharedBaseGraphEnvelopeSpan{Start: l, End: h, TopFaces: []int{0, 1}})
					}
				} else if (sign > 0) == (op == parametric.Union) {
					spans = appendEnvelopeSpan(spans, SharedBaseGraphEnvelopeSpan{Start: l, End: h, TopFaces: []int{i}})
				}
			}
		}
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].Start.Min.Cmp(spans[j].Start.Min) < 0 })
	return spans, nil
}

func appendEnvelopeSpan(spans []SharedBaseGraphEnvelopeSpan, next SharedBaseGraphEnvelopeSpan) []SharedBaseGraphEnvelopeSpan {
	if len(spans) > 0 {
		last := &spans[len(spans)-1]
		same := last.Base == next.Base && len(last.TopFaces) == len(next.TopFaces)
		if same {
			for i, v := range last.TopFaces {
				same = same && v == next.TopFaces[i]
			}
		}
		if same && last.End.Min.Cmp(next.Start.Min) == 0 && last.End.Max.Cmp(next.Start.Max) == 0 {
			last.End = next.End
			return spans
		}
	}
	return append(spans, next)
}
