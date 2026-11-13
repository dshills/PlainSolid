package geom

import "context"

// OffAxisSphereThroughBore is immutable closed material |p|²≤R² and
// (x-cx)²+(y-cy)²≥r² in sphere-centered coordinates. Its Z-directed bore
// passes completely through the sphere; no planar floor is present.
// Queries describe exact represented geometry independently of native sewing.
type OffAxisSphereThroughBore struct{ model OffAxisSphereBore }

// NewOffAxisSphereThroughBore proves the strict transverse domain d+r<R
// using exact rational inequalities. Both overlapping and disjoint axes are
// supported. Tangent or protruding cylinder footprints are rejected.
func NewOffAxisSphereThroughBore(R, r float64, offset Vec2) (OffAxisSphereThroughBore, error) {
	m, e := NewOffAxisSphereBore(R, r, offset, 0)
	if e != nil {
		return OffAxisSphereThroughBore{}, e
	}
	// A private nil floor means a constant positive clipping predicate, not a
	// numeric approximation to an infinitely distant floor.
	m.floor = nil
	return OffAxisSphereThroughBore{model: m}, nil
}

func (s OffAxisSphereThroughBore) Classify(p Vec3) (BorePointLocation, BoreBoundaryMask, error) {
	return s.model.Classify(p)
}

// IntersectLine returns isolated exact boundary hits, including both shared
// sphere/cylinder rims. Cylinder-coincident lines require IntersectLineBoundary.
// Options and bounded arithmetic semantics are the same as the blind-bore API.
func (s OffAxisSphereThroughBore) IntersectLine(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) ([]BoreLineHit, error) {
	return s.model.IntersectLine(ctx, origin, direction, minT, maxT, o)
}

// IntersectLineBoundary additionally reports complete closed cylindrical
// contact intervals clipped to the finite original parameter range. Endpoint
// support masks include both sphere and cylinder at either rim. Returned bounds
// are caller-owned; root identities are immutable. BoreFloor is never reported.
func (s OffAxisSphereThroughBore) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	return s.model.IntersectLineBoundary(ctx, origin, direction, minT, maxT, o)
}
