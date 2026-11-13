package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplineGraphStraightSection describes the exact model-space curve shared
// by both graph supports along an exhaustive oblique contact. The original
// terminal coordinates and model points are retained even if nonbinary.
type SplineGraphStraightSection struct {
	Line   geom.SplineGraphObliqueLine
	Curves [2]geom.SurfaceAffineCurvePieces
	Points [2][3]*big.Rat
}

// SectionObliqueSplineGraphs produces the complete exact model section of
// two common-chart spline graphs in the certified oblique-line family.
// Each original support is restricted independently and the resulting
// model curves are compared across the union of their exact knot cuts.
func SectionObliqueSplineGraphs(ctx context.Context, a, b *geom.SurfacePatch,
	o geom.GraphFactorAxisLineOptions) (SplineGraphStraightSection, error) {
	var empty SplineGraphStraightSection
	p, e := geom.DiscoverSplineGraphObliqueLine(ctx, a, b, o)
	if e != nil {
		return empty, e
	}
	if !p.Proven {
		return empty, geom.ErrConvergence
	}
	out := SplineGraphStraightSection{Line: p}
	budget := o.MaxOperations
	if budget == 0 {
		budget = 8000000
	}
	budget -= p.ExactOperations
	for i, s := range [2]*geom.SurfacePatch{a, b} {
		if budget < 1 {
			return empty, geom.ErrConvergence
		}
		r, e := s.AffineParameterCurvePieces(ctx, p.Terminals[0], p.Terminals[1], geom.SurfaceAffineCurveOptions{
			TubeAffineLineOptions: geom.TubeAffineLineOptions{MaxOperations: budget, MaxCoefficientBits: o.MaxCoefficientBits},
			MaxPieces:             o.MaxCells,
		})
		if e != nil {
			return empty, e
		}
		if !r.Complete || !r.Continuous {
			return empty, geom.ErrConvergence
		}
		budget -= r.ExactOperations
		out.Curves[i] = r
	}
	if budget < 1 {
		return empty, geom.ErrConvergence
	}
	match, e := geom.CompareAffineParameterCurvePieces(ctx, out.Curves[0], out.Curves[1], geom.ExactCurveComparisonOptions{
		MaxOperations: budget, MaxCoefficientBits: o.MaxCoefficientBits, MaxSpans: o.MaxCells,
	})
	if e != nil {
		return empty, e
	}
	if !match.Complete || !match.Equal {
		return empty, geom.ErrConvergence
	}
	budget -= match.ExactOperations
	for i, x := range p.Terminals {
		if budget < 1 {
			return empty, geom.ErrConvergence
		}
		v, e := a.EvaluateRational(ctx, x[0], x[1], geom.SurfacePatchEvaluationOptions{
			MaxOperations: budget, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if e != nil {
			return empty, e
		}
		budget -= v.ExactOperations
		out.Points[i] = v.Point
	}
	return out, nil
}
