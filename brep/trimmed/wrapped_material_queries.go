package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateCanonicalWrappedCylinder returns an exact material classification for
// a certified capped rational cylinder, independent of modeling tolerance.
func (s Solid) LocateCanonicalWrappedCylinder(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	w, _, e := s.ValidateCanonicalWrappedCylinder(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	r, e := geom.LocateWrappedCylinder(ctx, w, point, o)
	switch r {
	case geom.CylinderInside:
		return parametric.Inside, e
	case geom.CylinderBoundary:
		return parametric.Boundary, e
	default:
		return parametric.Outside, e
	}
}

// IntersectCanonicalWrappedCylinderLine intersects the certified complete
// lateral and cap boundary. Roots and overlap endpoints are in the original
// represented finite line parameter, independent of modeling tolerance.
func (s Solid) IntersectCanonicalWrappedCylinderLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (geom.CylinderLineIntersections, error) {
	w, _, e := s.ValidateCanonicalWrappedCylinder(ctx, o.Surface)
	if e != nil {
		return geom.CylinderLineIntersections{}, e
	}
	return geom.IntersectWrappedCylinderLine(ctx, w, origin, direction, minT, maxT, o)
}

// IntersectCanonicalWrappedToroidLine intersects a complete canonical
// circular-profile wrapped toroid. The input line need not be normalized.
// Result brackets use its original parameter and retain exact multiplicity.
func (s Solid) IntersectCanonicalWrappedToroidLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) ([]geom.ToroidLineRoot, error) {
	p, e := s.ValidateDisjointSplineComponents(ctx, SplineRectangleEmbeddingOptions{Patches: o.Surface, MaxFaces: 1})
	if e != nil {
		return nil, e
	}
	if len(p) != 1 || p[0].Toroid == nil {
		return nil, geom.ErrConvergence
	}
	w := s.faces[0].Surface.(*geom.WrappedSurfacePatch)
	return geom.IntersectWrappedCircularToroidLine(ctx, w, origin, direction, minT, maxT, o)
}

// LocateCanonicalWrappedFrustum classifies an exact point against a
// certified, completely capped rational frustum.
func (s Solid) LocateCanonicalWrappedFrustum(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	w, _, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	r, e := geom.LocateWrappedFrustum(ctx, w, point, o)
	switch r {
	case geom.CylinderInside:
		return parametric.Inside, e
	case geom.CylinderBoundary:
		return parametric.Boundary, e
	default:
		return parametric.Outside, e
	}
}

// IntersectCanonicalWrappedFrustumLine returns exact parameter brackets and
// coincident spans on a complete canonical rational frustum.
func (s Solid) IntersectCanonicalWrappedFrustumLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (geom.CylinderLineIntersections, error) {
	w, _, e := s.ValidateCanonicalWrappedFrustum(ctx, o.Surface)
	if e != nil {
		return geom.CylinderLineIntersections{}, e
	}
	return geom.IntersectWrappedFrustumLine(ctx, w, origin, direction, minT, maxT, o)
}

// SectionCanonicalWrappedFrustum returns the exact closed spline section
// at a represented Z height, including original rational parameters.
func (s Solid) SectionCanonicalWrappedFrustum(ctx context.Context, height float64, o geom.SurfacePatchOptions) (geom.WrappedFrustumSection, error) {
	w, _, e := s.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return geom.WrappedFrustumSection{}, e
	}
	return geom.SectionWrappedFrustum(ctx, w, height, o)
}

// BoundCanonicalWrappedCylinderDistance certifies squared distance to the
// complete rational cylinder boundary and returns an exact boundary witness.
func (s Solid) BoundCanonicalWrappedCylinderDistance(ctx context.Context, point geom.Vec3, o geom.CurveDistanceOptions) (geom.CylinderDistanceBounds, error) {
	w, _, e := s.ValidateCanonicalWrappedCylinder(ctx, geom.SurfacePatchOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if e != nil {
		return geom.CylinderDistanceBounds{}, e
	}
	return geom.BoundPointWrappedCylinderDistance(ctx, w, point, o)
}
