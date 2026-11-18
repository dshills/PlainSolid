//go:build research

package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// DisjointEllipseGraphHistory retains complete original-domain section evidence
// and original roof ownership, including distinct disconnected material shells.
type DisjointEllipseGraphHistory struct {
	Section  geom.SplineGraphDisjointEllipses
	Material SplineGraphBooleanHistory
}

// CombineDisjointEllipseContactSplineGraphs is an additive two-loop Boolean
// entry point. Both closed U slabs are re-proved on the actual original roofs;
// hints cannot omit an additional contact. Common/Fuse retain an outside region
// with two holes; positive-inside Cut retains two separate closed lens shells.
// Native arc conversion must be lossless. This does not discover arbitrary
// multiple contours or widen the existing single-loop automatic dispatcher.
func CombineDisjointEllipseContactSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, o intersect.SplinePlaneOptions, splitU float64, hints [2]geom.GraphEllipseHint, proof geom.GraphPointContactOptions) (Solid, DisjointEllipseGraphHistory, error) {
	fail := func(e error) (Solid, DisjointEllipseGraphHistory, error) {
		return Solid{}, DisjointEllipseGraphHistory{}, e
	}
	if ctx == nil || base == nil || a == nil || b == nil || op > parametric.Difference || !finite(tolerance) || tolerance <= 0 || o.Domain != (geom.ParameterRectangle{}) {
		return fail(geom.ErrInvalid)
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o)
	if e != nil {
		return fail(e)
	}
	section, e := geom.CertifySplineGraphDisjointEllipses(ctx, patches[0], patches[1], splitU, hints, proof)
	if e != nil {
		return fail(e)
	}
	return assembleDisjointEllipseGraphContact(ctx, base, a, b, op, tolerance, section, proof)
}

func assembleDisjointEllipseGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.SplineGraphDisjointEllipses, proof geom.GraphPointContactOptions) (Solid, DisjointEllipseGraphHistory, error) {
	fail := func(e error) (Solid, DisjointEllipseGraphHistory, error) {
		return Solid{}, DisjointEllipseGraphHistory{}, e
	}
	if !section.Complete || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return fail(geom.ErrConvergence)
	}
	solid, history, e := assembleSeparatedEllipseGraphRegions(ctx, base, a, b, op, tolerance, section.Domain, section.Contours[:], proof)
	if e != nil {
		return fail(e)
	}
	return solid, DisjointEllipseGraphHistory{Section: section, Material: history}, nil
}

// Consumes only complete independently re-proved original slab certificates
// from the private two- and three-loop constructors.
func assembleSeparatedEllipseGraphRegions(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, domain geom.ParameterRectangle, contours []geom.SplineGraphEllipse, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	outside := openGraphRegion{loop: partitionGraphRectangleLoop(domain), sign: contours[0].OutsideSide}
	regions := []openGraphRegion{outside}
	for _, q := range contours {
		loop, e := ellipseGraphContactLoop(q.Ellipse)
		if e != nil {
			return fail(e)
		}
		reverse := make([]openSectionPiece, len(loop))
		for i, x := range loop {
			reverse[len(loop)-1-i] = openSectionPiece{a: x.b, b: x.a, c: reversePCurve(x.c), side: -1}
		}
		regions[0].holes = append(regions[0].holes, reverse)
		regions = append(regions, openGraphRegion{loop: loop, sign: q.InsideSide})
	}
	solid, history, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance)
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for _, f := range solid.faces {
		if _, ok := f.Surface.(*geom.BSplineSurface); !ok {
			continue
		}
		checked, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if checked.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	return solid, history, nil
}
