package parametric

import (
	"context"
	"errors"
	"math"

	"plainsolid/geom"
	"plainsolid/mesh"
)

// TessellateRevolutionRims meshes coaxial cylinder, torus and straight
// meridian patches with shared angular rings. Straight generators use a bounded radial or axial grid. The same triangle
// checks apply as on a full grid.
func (s Solid) TessellateRevolutionRims(ctx context.Context, o TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || o.Deflection <= 0 || !finite(o.Deflection) || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 || o.AnalyticBound {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return mesh.Mesh{}, err
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 256
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	vertical := make([]int, len(s.faces))
	curved := make([]bool, len(s.faces))
	for i, face := range s.faces {
		surface, _ := SurfaceMapping(face.Surface)
		switch surface.(type) {
		case geom.CylinderSurface, revolvedSide, disk:
		case geom.TorusSurface:
			curved[i] = true
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
	}
	for n := 4; n <= o.MaxSubdivisions; n *= 2 {
		columns := 0
		for i := range vertical {
			vertical[i] = min(8, n)
			if curved[i] {
				vertical[i] = max(4, n/2)
			}
			columns += vertical[i]
		}
		if columns > o.MaxTriangles/(2*n) {
			break
		}
		m, err := s.gridWithDivisions(ctx, n, vertical, true, o.Deflection, o.Angle)
		if !errors.Is(err, errRefine) {
			return m, err
		}
	}
	return mesh.Mesh{}, ErrBudget
}
