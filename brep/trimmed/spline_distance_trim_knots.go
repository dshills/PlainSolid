package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
)

type composedKnotTrimSample struct {
	UV    geom.UVBox
	Point geom.Box
	// Original curve parameter, not a rounded algebraic knot-event parameter.
	Parameter *big.Rat
}

// Each original Bézier span retains its own exact parameter map. Algebraic
// event enclosures select candidate samples only; EvaluateRational re-proves
// membership against the immutable true endpoints before accepting a witness.
// All results are discarded on error. This bounded optional path proves no
// curve or face lower bound and does not assert coverage by the samples.
func composedKnotTrimWitness(ctx context.Context, p *geom.SurfacePatch, use Use, query geom.Vec3, limit int, o SplineBoundaryDistanceOptions) (best composedKnotTrimSample, used int, hit bool, err error) {
	type span struct {
		curve *geom.ExactSplineCurve
		a, b  float64
	}
	var spans []span
	fail := func(e error) (composedKnotTrimSample, int, bool, error) {
		return composedKnotTrimSample{}, used, false, e
	}
	switch q := use.Curve.(type) {
	case ExactBezierUV:
		c, e := q.descriptor(ctx, o.Patches)
		if e != nil {
			return fail(e)
		}
		spans = append(spans, span{c, 0, 1})
	case SplineUV:
		original, e := geom.NewExactSplineCurve(q.Curve, o.Patches)
		if e != nil {
			return fail(e)
		}
		cuts := []float64{q.Range.Min}
		for _, k := range q.Curve.Knots() {
			if k > q.Range.Min && k < q.Range.Max && k > cuts[len(cuts)-1] {
				cuts = append(cuts, k)
				if len(cuts) > 32 {
					return fail(geom.ErrConvergence)
				}
			}
		}
		cuts = append(cuts, q.Range.Max)
		for i := 1; i < len(cuts); i++ {
			c, e := original.Restrict(cuts[i-1], cuts[i], o.Patches)
			if e != nil {
				return fail(e)
			}
			net, _, e := c.BezierControlNet(ctx, o.Patches)
			if e != nil {
				return fail(e)
			}
			normalized, e := geom.NewExactHomogeneousBezier(ctx, net.Control, o.Patches)
			if e != nil {
				return fail(e)
			}
			spans = append(spans, span{normalized, cuts[i-1], cuts[i]})
		}
	default:
		return best, 0, false, nil
	}
	var upper *big.Rat
	for _, span := range spans {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		if used == limit {
			break
		}
		partition, e := p.BuildSurfaceCurveCompositionCells(ctx, span.curve, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: o.Patches, MaxEvents: 64, MaxSubdivisions: 1024})
		if e != nil {
			return fail(e)
		}
		if !partition.Complete {
			return fail(geom.ErrConvergence)
		}
		for _, cell := range partition.Cells {
			if used == limit {
				break
			}
			used++
			interval := cell.EndpointIntervals()
			if interval[0].Upper.Cmp(interval[1].Lower) >= 0 {
				continue
			}
			middle := new(big.Rat).Quo(new(big.Rat).Add(interval[0].Upper, interval[1].Lower), big.NewRat(2, 1))
			t, _ := middle.Float64()
			parameter := ratInset(t)
			point, e := cell.EvaluateRational(ctx, parameter, geom.ExactCurvePlaneEventOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits, MaxSubdivisions: 1024})
			if errors.Is(e, geom.ErrDomain) {
				continue
			}
			if e != nil {
				return fail(e)
			}
			uv, e := span.curve.EvaluateJet(t)
			if e != nil {
				return fail(e)
			}
			_, d := splinePointBoxSquared(query, point.Bounds)
			if upper == nil || d.Cmp(upper) < 0 {
				upper = d
				hit = true
				original := new(big.Rat).Add(ratInset(span.a), new(big.Rat).Mul(parameter, new(big.Rat).Sub(ratInset(span.b), ratInset(span.a))))
				best = composedKnotTrimSample{geom.UVBox{Min: geom.Vec2{X: uv.PointBounds.Min.X, Y: uv.PointBounds.Min.Y}, Max: geom.Vec2{X: uv.PointBounds.Max.X, Y: uv.PointBounds.Max.Y}}, point.Bounds, original}
			}
		}
	}
	return best, used, hit, nil
}
