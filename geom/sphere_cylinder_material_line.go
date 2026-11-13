package geom

import "context"

// SphereCylinderMaterialLine is the complete filled intersection of the query's
// material with a bounded raw line. Zero interval Boundary means solid interior;
// nonzero identifies continuous exposed support contact. Endpoint masks retain
// sphere/side/cap ownership. Points are isolated material contacts in the finite
// line range, not proof of isolated three-dimensional components. Exact roots
// are immutable and parameter enclosures are caller-owned.
type SphereCylinderMaterialLine struct {
	Intervals []BoreBoundaryInterval
	Points    []BoreLineHit
}

// ClipLine returns the regularized sphere-minus-finite-cylinder material, not
// sphere minus the tool's open interior. It preserves constructor qualification:
// unresolved internal tangencies remain rejected; proved unchanged/empty modes
// discard tool supports or all remnants respectively. Cap/generator coincidence
// is supported. ParameterTolerance only bounds root display brackets; decisions
// use exact signs. Arithmetic, comparison and cancellation limits follow the
// existing IntersectLineBoundary engine. Failure returns an empty report.
func (s OffAxisSphereFiniteCylinderDifference) ClipLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (SphereCylinderMaterialLine, error) {
	return s.model.clipRegularizedMaterial(ctx, origin, direction, minT, maxT, o, s.location)
}

// ClipLine returns the filled regular-closed union. As with this query's
// constructor, external touching components can meet at one point; no manifold
// B-rep is asserted. Hidden supports have zero masks. Finite ray/line endpoints
// inside the material have zero masks and are not artificial boundary contacts.
func (s OffAxisSphereFiniteCylinderUnion) ClipLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (SphereCylinderMaterialLine, error) {
	return s.model.clipRegularizedMaterial(ctx, origin, direction, minT, maxT, o, s.location)
}

func (s OffAxisSphereFiniteCylinderCommon) clipRegularizedMaterial(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (SphereCylinderMaterialLine, error) {
	r, e := s.intersectMaterial(ctx, origin, direction, minT, maxT, o, func(signs []int) (BorePointLocation, BoreBoundaryMask) {
		location, mask := classify(signs)
		if location == BoreInside {
			location = BoreBoundary
		}
		return location, mask
	})
	if e != nil {
		return SphereCylinderMaterialLine{}, e
	}
	if e = ctx.Err(); e != nil {
		return SphereCylinderMaterialLine{}, e
	}
	return SphereCylinderMaterialLine{Intervals: r.Intervals, Points: r.Hits}, nil
}
