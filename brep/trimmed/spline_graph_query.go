package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineGraphQuery classifies points relative to all bounded regions between
// two spline graphs having the same projected chart. The regions have the
// opposite height order from the domain boundary. The query is immutable.
type SplineGraphQuery struct {
	a, b     *geom.SurfacePatch
	boundary int
	height   geom.SurfaceHeightDifferenceOptions
}

// NewSplineGraphQuery proves the shared XY chart's injectivity and a strict
// height order along its entire domain boundary. It accepts the same basis
// and proportional weights as SealSplineGraphPair. It also
// proves a complete regular section; this excludes isolated tangencies
// from the material boundary. A zero domain selects the whole chart.
func NewSplineGraphQuery(ctx context.Context, a, b *geom.BSplineSurface, domain geom.ParameterRectangle,
	patch geom.SurfacePatchOptions, inverse geom.SurfaceXYInverseOptions) (*SplineGraphQuery, error) {
	if ctx == nil || a == nil || b == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	d, pa, pb, e := exactGraphPatches(a, b, patch, domain)
	if e != nil {
		return nil, e
	}
	proof := inverse.Injectivity
	if proof.ProjectionAxes != ([2]int{}) && proof.ProjectionAxes != ([2]int{0, 1}) {
		return nil, geom.ErrInvalid
	}
	r, e := geom.CertifySurfaceInjectivity(d, proof)
	if e != nil {
		return nil, e
	}
	if !r.ProvenInjective {
		return nil, geom.ErrConvergence
	}
	plane, _ := geom.NewPlane(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := geom.ClassifyPlaneBoundarySigns(d, plane, geom.PlaneBoundarySignOptions{
		MaxOperations: proof.MaxOperations, MaxCoefficientBits: proof.MaxCoefficientBits,
	})
	if e != nil {
		return nil, e
	}
	if !s.Complete || s.StrictSign == 0 {
		return nil, fmt.Errorf("trimmed: graph query requires a strict boundary height order")
	}
	contours, e := geom.TraceSurfacePlaneContext(ctx, d, plane, geom.SurfaceTraceOptions{})
	if e != nil {
		return nil, e
	}
	if !contours.Complete || len(contours.Isolation.Coplanar) != 0 || len(contours.Isolation.Tangencies) != 0 {
		return nil, geom.ErrConvergence
	}
	for _, c := range contours.Contours {
		if !c.Closed {
			return nil, geom.ErrConvergence
		}
	}
	return &SplineGraphQuery{a: pa, b: pb, boundary: s.StrictSign, height: geom.SurfaceHeightDifferenceOptions{Inverse: inverse}}, nil
}

// Classify uses exact outward enclosures, with no tessellation or tolerance
// snapping. A height equality that cannot be established remains unresolved.
func (q *SplineGraphQuery) Classify(ctx context.Context, p geom.Vec3) (parametric.Location, error) {
	if q == nil || ctx == nil || !p.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return parametric.Outside, e
	}
	r, e := geom.ClassifySurfaceHeightDifference(q.a, q.b, p, q.height)
	if r.OutsideA && r.OutsideB {
		return parametric.Outside, nil
	}
	if int(r.Relation) == q.boundary {
		return parametric.Outside, nil
	}
	if r.PointVsA == geom.HeightAbove && r.PointVsB == geom.HeightAbove ||
		r.PointVsA == geom.HeightBelow && r.PointVsB == geom.HeightBelow {
		return parametric.Outside, nil
	}
	if e != nil {
		return parametric.Outside, e
	}
	if r.OutsideA || r.OutsideB || r.Relation == geom.HeightUnresolved ||
		r.PointVsA == geom.HeightUnresolved || r.PointVsB == geom.HeightUnresolved {
		return parametric.Outside, geom.ErrConvergence
	}
	if r.PointVsA == geom.HeightBoundary || r.PointVsB == geom.HeightBoundary {
		return parametric.Boundary, nil
	}
	if r.Relation == geom.HeightBoundary {
		return parametric.Outside, nil
	}
	return parametric.Inside, nil
}
