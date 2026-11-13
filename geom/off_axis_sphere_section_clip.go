package geom

import "context"

// SphereCylinderSectionClip is a closed horizontal section clipped to a finite
// XY parameterized line. Intervals contain material, not merely boundary.
// Their Boundary mask is zero in strict 3D material interior, or identifies a
// support containing the interval. Endpoint masks report all exposed supports.
// Points are isolated section contacts not already covered by an interval.
// Adjacent intervals may share an endpoint when their interior support changes.
// All roots are immutable and all displayed bounds are caller-owned.
type SphereCylinderSectionClip struct {
	Intervals []BoreBoundaryInterval
	Points    []BoreLineHit
}

// ClipSectionLine computes all t in [minT,maxT] such that
// (origin.X+t*direction.X, origin.Y+t*direction.Y, height) belongs to the
// regularized finite-difference material. Height and line data are interpreted
// exactly as represented. In particular a pocket floor is retained material:
// its section is not the sphere disk minus the closed cutter disk. Tangent
// section points are preserved. No polygonal approximation or shell is built.
func (s OffAxisSphereFiniteCylinderDifference) ClipSectionLine(ctx context.Context, height float64, origin, direction Vec2, minT, maxT float64, o ToroidLineOptions) (SphereCylinderSectionClip, error) {
	return s.model.clipSectionMaterial(ctx, height, origin, direction, minT, maxT, o, s.location)
}

// ClipSectionLine returns the filled horizontal section of the finite common
// material, including partial cap disks and isolated tangent section points.
func (s OffAxisSphereFiniteCylinderCommon) ClipSectionLine(ctx context.Context, height float64, origin, direction Vec2, minT, maxT float64, o ToroidLineOptions) (SphereCylinderSectionClip, error) {
	return s.clipSectionMaterial(ctx, height, origin, direction, minT, maxT, o, finiteCommonLocation)
}

// ClipSectionLine returns the filled horizontal section of the regular closed
// union. Covered cap/wall events have no boundary ancestry; exposed or tangent
// support events remain interval endpoints even when material is continuous.
func (s OffAxisSphereFiniteCylinderUnion) ClipSectionLine(ctx context.Context, height float64, origin, direction Vec2, minT, maxT float64, o ToroidLineOptions) (SphereCylinderSectionClip, error) {
	return s.model.clipSectionMaterial(ctx, height, origin, direction, minT, maxT, o, s.location)
}

func (s OffAxisSphereFiniteCylinderCommon) clipSectionMaterial(ctx context.Context, height float64, origin, direction Vec2, minT, maxT float64, o ToroidLineOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (SphereCylinderSectionClip, error) {
	r, e := s.intersectMaterial(ctx, Vec3{X: origin.X, Y: origin.Y, Z: height}, Vec3{X: direction.X, Y: direction.Y}, minT, maxT, o, func(v []int) (BorePointLocation, BoreBoundaryMask) {
		location, mask := classify(v)
		if location == BoreInside {
			location = BoreBoundary
		}
		return location, mask
	})
	if e != nil {
		return SphereCylinderSectionClip{}, e
	}
	return SphereCylinderSectionClip{Intervals: r.Intervals, Points: r.Hits}, nil
}
