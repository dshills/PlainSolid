package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// BoxFinitePartialSkewBores retains the partial-skew through intersection,
// stopping either bore beyond its entire crossing. Zero depth means through.
// Original ten faces are unchanged; X then Z floors are appended with explicit IDs.
func BoxFinitePartialSkewBores(ctx context.Context, frame geom.Frame, dims, crossing geom.Vec3, xRadius, zRadius, offset, xDepth, zDepth float64, xOpposite, zOpposite bool, tolerance float64) (FiniteSkewCrossedBoresResult, error) {
	return boxFiniteRadicalCrossedBores(ctx, frame, dims, crossing, xRadius, zRadius, offset,
		xDepth, zDepth, xOpposite, zOpposite, tolerance, BoxPartialSkewBores)
}

// TessellateFinitePartialSkewBores meshes retained partial-skew trims and floors.
func (s Solid) TessellateFinitePartialSkewBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if len(s.faces) < 10 || len(s.faces) > 12 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	found := false
	for _, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.PlaneSurface, geom.CylinderSurface:
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				switch u.Curve.(type) {
				case partialSkewTrim:
					found = true
				case nil, EllipseUV, unionArcUV:
				default:
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
		}
	}
	if !found {
		return mesh.Mesh{}, geom.ErrDomain
	}
	// Repeated diagonal flips are expensive between a retained oval and a
	// nearby floor. Try refinement without those flips and the final cleanup
	// pass. The mesh still shares edge samples and passes all surface checks.
	fast := o
	fast.MaxTriangles = min(fast.MaxTriangles, 80_000)
	m, err := s.refineAnalyticMesh(ctx, fast, false)
	if err == nil {
		return m, nil
	}
	if ctx.Err() != nil {
		return mesh.Mesh{}, ctx.Err()
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}
