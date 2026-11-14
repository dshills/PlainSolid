package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// TessellateObliqueBore uses conforming directional refinement for a rectangular
// through bore with elliptical rims, keeping the planar and cylindrical owners
// on the same model-edge samples.
func (s Solid) TessellateObliqueBore(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	if len(s.vertices) != 16 || len(s.edges) != 24 || len(s.faces) != 10 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	planes, openings, walls := 0, 0, 0
	for _, face := range s.faces {
		if len(face.Outer) != 4 {
			return mesh.Mesh{}, geom.ErrDomain
		}
		base, _ := parametric.SurfaceMapping(face.Surface)
		switch base.(type) {
		case geom.PlaneSurface:
			planes++
			if len(face.Holes) == 0 {
				continue
			}
			if len(face.Holes) != 1 || len(face.Holes[0]) != 4 {
				return mesh.Mesh{}, geom.ErrDomain
			}
			for _, rim := range face.Holes[0] {
				if _, ok := rim.Curve.(EllipseUV); !ok {
					return mesh.Mesh{}, geom.ErrDomain
				}
			}
			openings++
		case geom.CylinderSurface:
			if len(face.Holes) != 0 || !obliqueBoreWall(face.Outer) {
				return mesh.Mesh{}, geom.ErrDomain
			}
			walls++
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
	}
	if planes != 6 || openings != 2 || walls != 4 {
		return mesh.Mesh{}, geom.ErrDomain
	}
	if m, ok, e := s.compactCylinderMesh(ctx, o); ok {
		return m, e
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}

// The bore charts alternate an elliptical arc with a straight generator.
func obliqueBoreWall(loop []Use) bool {
	for i, use := range loop {
		if i%2 != 0 {
			if use.Curve != nil || use.UVStart.X != use.UVEnd.X {
				return false
			}
			continue
		}
		arc, ok := use.Curve.(EllipseUV)
		if !ok || arc.Linear != (geom.Vec2{X: 1}) || arc.Center.X != 0 || arc.Cos.X != 0 || arc.Sin.X != 0 {
			return false
		}
	}
	return true
}
