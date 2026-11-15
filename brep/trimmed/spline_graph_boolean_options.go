package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineGraphBooleanOptions keeps Boolean-only discovery separate from stored
// section-trim recipes. All discovery pointers nil select the unchanged legacy entry.
// Exactly one nonnil value selects bounded complete contact discovery exclusively;
// an unsupported/exhausted proof fails atomically, without a speculative result.
// Parabola selects one open quadratic graph with exact original side terminals.
type SplineGraphBooleanOptions struct {
	Section          intersect.SplinePlaneOptions
	DisjointEllipses *geom.DisjointEllipseDiscoveryOptions
	ThreeEllipses    *geom.ThreeEllipseDiscoveryOptions
	Parabola         *geom.GraphPointContactOptions
	// ParabolaAxis selects the original free coordinate (0=U, 1=V).
	ParabolaAxis int
	// PiecewiseParabola retains every original free-axis knot slab and exact join.
	PiecewiseParabola *geom.PiecewiseParabolaOptions
	// Polynomial selects a genuine cubic/quartic original graph.
	Polynomial     *geom.GraphPointContactOptions
	PolynomialAxis int
}

// CombineSplineGraphSolidsWithOptions is the opt-in normal common-base graph
// Boolean entry. It requires no caller separator, ellipse or material labels.
// Original supports must satisfy the same strict identity-XY and positive-base
// requirements as CombineSplineGraphSolids. These options recognize only
// their complete original chart domains documented by the geometry APIs.
func CombineSplineGraphSolidsWithOptions(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o SplineGraphBooleanOptions) (Solid, SplineGraphBooleanHistory, error) {
	if o.DisjointEllipses == nil && o.ThreeEllipses == nil && o.Parabola == nil && o.PiecewiseParabola == nil && o.Polynomial == nil {
		return CombineSplineGraphSolids(ctx, base, a, b, op, tolerance, o.Section)
	}
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	selected := 0
	if o.DisjointEllipses != nil {
		selected++
	}
	if o.ThreeEllipses != nil {
		selected++
	}
	if o.Parabola != nil {
		selected++
	}
	if o.PiecewiseParabola != nil {
		selected++
	}
	if o.Polynomial != nil {
		selected++
	}
	if selected > 1 {
		return fail(geom.ErrInvalid)
	}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference || !finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return fail(e)
	}
	if o.Polynomial != nil {
		section, e := geom.DiscoverSplineGraphPolynomialContact(ctx, patches[0], patches[1], o.PolynomialAxis, *o.Polynomial)
		if e != nil {
			return fail(e)
		}
		return assemblePolynomialGraphContact(ctx, base, a, b, op, tolerance, section, *o.Polynomial)
	}
	if o.PiecewiseParabola != nil {
		section, e := geom.DiscoverPiecewiseSplineGraphParabola(ctx, patches[0], patches[1], *o.PiecewiseParabola)
		if e != nil {
			return fail(e)
		}
		return assemblePiecewiseParabolicGraphContact(ctx, base, a, b, op, tolerance, section, o.PiecewiseParabola.Proof)
	}
	if o.Parabola != nil {
		section, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, patches[0], patches[1], o.ParabolaAxis, *o.Parabola)
		if e != nil {
			return fail(e)
		}
		return assembleParabolicGraphContact(ctx, base, a, b, op, tolerance, section, *o.Parabola)
	}
	if o.ThreeEllipses != nil {
		discovery, e := geom.DiscoverSplineGraphThreeEllipses(ctx, patches[0], patches[1], *o.ThreeEllipses)
		if e != nil {
			return fail(e)
		}
		if !discovery.Complete || discovery.Section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
			return fail(geom.ErrConvergence)
		}
		solid, history, e := assembleSeparatedEllipseGraphRegions(ctx, base, a, b, op, tolerance, discovery.Section.Domain, discovery.Section.Contours[:], o.ThreeEllipses.GraphPointContactOptions)
		if e != nil {
			return fail(e)
		}
		history.ThreeEllipses = &discovery
		return solid, history, nil
	}
	discovery, e := geom.DiscoverSplineGraphDisjointEllipses(ctx, patches[0], patches[1], *o.DisjointEllipses)
	if e != nil {
		return fail(e)
	}
	if !discovery.Complete {
		return fail(geom.ErrConvergence)
	}
	solid, history, e := assembleDisjointEllipseGraphContact(ctx, base, a, b, op, tolerance, discovery.Section, o.DisjointEllipses.GraphPointContactOptions)
	if e != nil {
		return fail(e)
	}
	history.Material.DisjointEllipses = &discovery
	return solid, history.Material, nil
}
