package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SharedBaseGraphMaterialLine lists all material intervals and boundary
// events of a supported finite line. Endpoints are exact rational or
// isolated algebraic parameters, in the original line parameter.
type SharedBaseGraphMaterialLine struct {
	Intervals [][2]geom.ToroidLineRoot
	Hits      []SharedBaseGraphEnvelopeHit
}

// ClipEnvelopeMaterialLine returns complete finite material intervals for
// a nonvertical line with isolated envelope boundary events. A tangent
// can be an isolated boundary event or join two incident material spans.
// Side-wall coincidence uses IntersectEnvelopeSideLine instead; coincidence
// with a top or the base is outside this API.
func (p SharedBaseSplineGraphMaterial) ClipEnvelopeMaterialLine(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, op parametric.BooleanOp, o geom.ToroidLineOptions) (SharedBaseGraphMaterialLine, error) {
	var empty SharedBaseGraphMaterialLine
	hits, e := p.IntersectEnvelopeLine(ctx, origin, direction, minT, maxT, op, o)
	if e != nil {
		return empty, e
	}
	r := SharedBaseGraphMaterialLine{Hits: hits}
	r.Intervals, e = p.graphMaterialIntervals(ctx, origin, direction, minT, maxT, op, o, hits)
	if e != nil {
		return empty, e
	}
	return r, nil
}

func (p SharedBaseSplineGraphMaterial) graphMaterialIntervals(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	op parametric.BooleanOp, o geom.ToroidLineOptions, hits []SharedBaseGraphEnvelopeHit) ([][2]geom.ToroidLineRoot, error) {
	var spans [][2]geom.ToroidLineRoot
	exact := func(t float64) geom.ToroidLineRoot {
		return geom.ToroidLineRoot{Min: ratInset(t), Max: ratInset(t), Multiplicity: 1}
	}
	a, z := exact(minT), exact(maxT)
	breaks := []geom.ToroidLineRoot{a}
	for _, h := range hits {
		if h.Parameter.Min.Cmp(a.Min) == 0 && h.Parameter.Max.Cmp(a.Min) == 0 {
			breaks[0] = h.Parameter
		} else {
			breaks = append(breaks, h.Parameter)
		}
	}
	if breaks[len(breaks)-1].Max.Cmp(z.Max) < 0 {
		breaks = append(breaks, z)
	}
	x, d := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		[3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	previous := false
	for i := 1; i < len(breaks); i++ {
		l, h := breaks[i-1], breaks[i]
		if l.Max.Cmp(h.Min) >= 0 {
			return nil, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(l.Max, h.Min), big.NewRat(2, 1))
		var point [3]*big.Rat
		for j := range point {
			point[j] = new(big.Rat).Add(x[j], new(big.Rat).Mul(t, d[j]))
		}
		loc, e := p.LocateEnvelopeRational(ctx, point, op, geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits,
		})
		if e != nil {
			return nil, e
		}
		inside := loc != parametric.Outside
		if inside {
			if previous {
				spans[len(spans)-1][1] = h
			} else {
				spans = append(spans, [2]geom.ToroidLineRoot{l, h})
			}
		}
		previous = inside
	}
	return spans, nil
}
