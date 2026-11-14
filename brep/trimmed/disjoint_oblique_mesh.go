package trimmed

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// TessellateDisjointObliqueBores uses conforming directional refinement for a rectangular
// through bore with elliptical rims, keeping the planar and cylindrical owners
// on the same model-edge samples.
func (s Solid) TessellateDisjointObliqueBores(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
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
	n := (len(s.faces) - 6) / 4
	if n < 2 || n > 3 || len(s.vertices) != 8+8*n || len(s.edges) != 12+12*n || len(s.faces) != 6+4*n {
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
			if len(face.Holes) > n {
				return mesh.Mesh{}, geom.ErrDomain
			}
			for _, hole := range face.Holes {
				if len(hole) != 4 {
					return mesh.Mesh{}, geom.ErrDomain
				}
				for _, rim := range hole {
					if _, ok := rim.Curve.(EllipseUV); !ok {
						return mesh.Mesh{}, geom.ErrDomain
					}
				}
				openings++
			}
		case geom.CylinderSurface:
			if len(face.Holes) != 0 || !disjointObliqueBoreWall(face.Outer) {
				return mesh.Mesh{}, geom.ErrDomain
			}
			walls++
		default:
			return mesh.Mesh{}, geom.ErrDomain
		}
	}
	if planes != 6 || openings != 2*n || walls != 4*n {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptive(ctx, o)
}

func disjointObliqueBoreWall(loop []Use) bool {
	for i, u := range loop {
		if i%2 != 0 {
			if u.Curve != nil || u.UVStart.X != u.UVEnd.X {
				return false
			}
			continue
		}
		if u.Curve == nil {
			if u.UVStart.Y != u.UVEnd.Y {
				return false
			}
			continue
		}
		a, ok := u.Curve.(EllipseUV)
		if !ok || a.Linear != (geom.Vec2{X: 1}) || a.Center.X != 0 || a.Cos.X != 0 || a.Sin.X != 0 {
			return false
		}
	}
	return true
}
