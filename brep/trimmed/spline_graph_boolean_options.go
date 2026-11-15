package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineGraphBooleanOptions keeps Boolean-only discovery separate from stored
// section-trim recipes. Nil DisjointEllipses selects the unchanged legacy entry.
// A nonnil value selects bounded original-knot two-loop discovery exclusively;
// an unsupported/exhausted proof fails atomically, without a speculative result.
type SplineGraphBooleanOptions struct {
	Section          intersect.SplinePlaneOptions
	DisjointEllipses *geom.DisjointEllipseDiscoveryOptions
}

// CombineSplineGraphSolidsWithOptions is the opt-in normal common-base graph
// Boolean entry. It requires no caller separator, ellipse or material labels.
// Original supports must satisfy the same strict identity-XY and positive-base
// requirements as CombineSplineGraphSolids. The two-loop option recognizes only
// the complete original-U-knot partition domain documented by its geometry API.
func CombineSplineGraphSolidsWithOptions(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o SplineGraphBooleanOptions) (Solid, SplineGraphBooleanHistory, error) {
	if o.DisjointEllipses == nil {
		return CombineSplineGraphSolids(ctx, base, a, b, op, tolerance, o.Section)
	}
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
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
