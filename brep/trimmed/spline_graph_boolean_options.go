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
	// TwoPolynomials selects exactly two strictly ordered cubic/quartic graphs.
	TwoPolynomials     *geom.GraphPointContactOptions
	TwoPolynomialsAxis int
	// PolynomialNode selects two cubic/quartic graph branches crossing once.
	// Common and Fuse require exactly representable node and terminal vertices.
	// Cut returns ErrDegenerate because its opposite sectors meet at the node.
	// CutSplineGraphPolynomialNodeComponents exposes those two material components.
	PolynomialNode *geom.GraphPointContactOptions
	// PolynomialNodeAxis selects the original free coordinate (0=U, 1=V).
	PolynomialNodeAxis int
	// TwoPolynomialNodes selects Common or Fuse across exactly two transverse
	// crossings of cubic/quartic graph branches. Native vertices must be exact.
	// CutSplineGraphTwoPolynomialNodeComponents exposes the touching Cut components.
	TwoPolynomialNodes     *geom.GraphPointContactOptions
	TwoPolynomialNodesAxis int // original free coordinate (0=U, 1=V)
	// ThreePolynomialNodes selects Common or Fuse across exactly three
	// transverse crossings of cubic/quartic graph branches. Native vertices
	// must be exact. CutSplineGraphThreePolynomialNodeComponents separately
	// proves and retains the point-contacting Cut components.
	ThreePolynomialNodes     *geom.GraphPointContactOptions
	ThreePolynomialNodesAxis int // original free coordinate (0=U, 1=V)
	Section                  intersect.SplinePlaneOptions
	DisjointEllipses         *geom.DisjointEllipseDiscoveryOptions
	ThreeEllipses            *geom.ThreeEllipseDiscoveryOptions
	Parabola                 *geom.GraphPointContactOptions
	// ParabolaAxis selects the original free coordinate (0=U, 1=V).
	ParabolaAxis int
	// PiecewiseParabola retains every original free-axis knot slab and exact join.
	PiecewiseParabola *geom.PiecewiseParabolaOptions
	// PiecewisePolynomial retains cubic/quartic slabs with one explicit complete proof mode.
	PiecewisePolynomial *geom.PiecewisePolynomialOptions
	// Polynomial selects a genuine cubic/quartic original graph.
	Polynomial     *geom.GraphPointContactOptions
	PolynomialAxis int
	// FactoredPolynomial proves a strictly signed nonconstant free-coordinate factor.
	FactoredPolynomial     *geom.GraphPointContactOptions
	FactoredPolynomialAxis int
	// BivariatePolynomial permits a strictly signed linear-dependent cofactor.
	BivariatePolynomial     *geom.GraphPointContactOptions
	BivariatePolynomialAxis int
}

// CombineSplineGraphSolidsWithOptions is the opt-in normal common-base graph
// Boolean entry. It requires no caller separator, ellipse or material labels.
// Original supports must satisfy the same strict identity-XY and positive-base
// requirements as CombineSplineGraphSolids. These options recognize only
// their complete original chart domains documented by the geometry APIs.
func CombineSplineGraphSolidsWithOptions(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o SplineGraphBooleanOptions) (Solid, SplineGraphBooleanHistory, error) {
	if o.ThreePolynomialNodes == nil && o.TwoPolynomialNodes == nil && o.PolynomialNode == nil && o.TwoPolynomials == nil && o.DisjointEllipses == nil && o.ThreeEllipses == nil && o.Parabola == nil && o.PiecewiseParabola == nil && o.Polynomial == nil && o.FactoredPolynomial == nil && o.BivariatePolynomial == nil && o.PiecewisePolynomial == nil {
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
	if o.FactoredPolynomial != nil {
		selected++
	}
	if o.BivariatePolynomial != nil {
		selected++
	}
	if o.PiecewisePolynomial != nil {
		selected++
	}
	if o.TwoPolynomials != nil {
		selected++
	}
	if o.PolynomialNode != nil {
		selected++
	}
	if o.TwoPolynomialNodes != nil {
		selected++
	}
	if o.ThreePolynomialNodes != nil {
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
	if o.ThreePolynomialNodes != nil {
		section, e := geom.DiscoverSplineGraphThreePolynomialNodes(ctx, patches[0], patches[1], o.ThreePolynomialNodesAxis, *o.ThreePolynomialNodes)
		if e != nil {
			return fail(e)
		}
		return assembleThreePolynomialNodeGraphs(ctx, base, a, b, op, tolerance, section, *o.ThreePolynomialNodes)
	}
	if o.TwoPolynomialNodes != nil {
		section, e := geom.DiscoverSplineGraphTwoPolynomialNodes(ctx, patches[0], patches[1], o.TwoPolynomialNodesAxis, *o.TwoPolynomialNodes)
		if e != nil {
			return fail(e)
		}
		return assembleTwoPolynomialNodeGraphs(ctx, base, a, b, op, tolerance, section, *o.TwoPolynomialNodes)
	}
	if o.PolynomialNode != nil {
		section, e := geom.DiscoverSplineGraphPolynomialNode(ctx, patches[0], patches[1], o.PolynomialNodeAxis, *o.PolynomialNode)
		if e != nil {
			return fail(e)
		}
		return assemblePolynomialNodeGraphBoolean(ctx, base, a, b, op, tolerance, section, *o.PolynomialNode)
	}
	if o.TwoPolynomials != nil {
		section, e := geom.DiscoverSplineGraphTwoPolynomials(ctx, patches[0], patches[1], o.TwoPolynomialsAxis, *o.TwoPolynomials)
		if e != nil {
			return fail(e)
		}
		return assembleTwoPolynomialGraphs(ctx, base, a, b, op, tolerance, section, *o.TwoPolynomials)
	}
	if o.PiecewisePolynomial != nil {
		section, e := geom.DiscoverPiecewiseSplineGraphPolynomial(ctx, patches[0], patches[1], *o.PiecewisePolynomial)
		if e != nil {
			return fail(e)
		}
		return assemblePiecewisePolynomialGraphContact(ctx, base, a, b, op, tolerance, section, o.PiecewisePolynomial.Proof)
	}
	if o.BivariatePolynomial != nil {
		section, e := geom.DiscoverSplineGraphBivariatePolynomialContact(ctx, patches[0], patches[1], o.BivariatePolynomialAxis, *o.BivariatePolynomial)
		if e != nil {
			return fail(e)
		}
		return assembleBivariatePolynomialGraphContact(ctx, base, a, b, op, tolerance, section, *o.BivariatePolynomial)
	}
	if o.FactoredPolynomial != nil {
		section, e := geom.DiscoverSplineGraphFactoredPolynomialContact(ctx, patches[0], patches[1], o.FactoredPolynomialAxis, *o.FactoredPolynomial)
		if e != nil {
			return fail(e)
		}
		return assembleFactoredPolynomialGraphContact(ctx, base, a, b, op, tolerance, section, *o.FactoredPolynomial)
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
