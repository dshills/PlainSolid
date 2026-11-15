package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineGraphBooleanOptions keeps Boolean-only discovery separate from stored
// section-trim recipes. Both discovery pointers nil select the unchanged legacy entry.
// Exactly one nonnil value selects bounded original-knot two- or three-loop discovery exclusively;
// an unsupported/exhausted proof fails atomically, without a speculative result.
type SplineGraphBooleanOptions struct {
	Section          intersect.SplinePlaneOptions
	DisjointEllipses *geom.DisjointEllipseDiscoveryOptions
	ThreeEllipses    *geom.ThreeEllipseDiscoveryOptions
}

// CombineSplineGraphSolidsWithOptions is the opt-in normal common-base graph
// Boolean entry. It requires no caller separator, ellipse or material labels.
// Original supports must satisfy the same strict identity-XY and positive-base
// requirements as CombineSplineGraphSolids. These options recognize only
// their complete original-U-knot partition domains documented by the geometry APIs.
func CombineSplineGraphSolidsWithOptions(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o SplineGraphBooleanOptions) (Solid, SplineGraphBooleanHistory, error) {
	if o.DisjointEllipses == nil && o.ThreeEllipses == nil {
		return CombineSplineGraphSolids(ctx, base, a, b, op, tolerance, o.Section)
	}
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	if o.DisjointEllipses != nil && o.ThreeEllipses != nil {
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
