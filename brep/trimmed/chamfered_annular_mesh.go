package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate uses shared circular strips along the exact linear meridian.
// Chord sagitta and normal turning determine angular resolution; straight
// generators do not need axial refinement. This is a numerical display mesh.
func (p ChamferedAnnularPrism) Tessellate(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if e := p.validate(); e != nil {
		return mesh.Mesh{}, e
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	limit, budget := o.MaxSubdivisions, o.MaxTriangles
	if limit == 0 {
		limit = 128
	}
	if budget == 0 {
		budget = 1000000
	}
	n := 8
	for 2*p.OuterRadius*math.Pow(math.Sin(math.Pi/float64(n)/2), 2) > o.Deflection || o.Angle > 0 && math.Pi/float64(n) > o.Angle {
		if n > limit/2 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		n *= 2
	}
	profile := p.profile()
	if n > limit || len(profile) > budget/(2*n) {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	m := mesh.Mesh{Vertices: make([]geom.Vec3, 0, len(profile)*n), Triangles: make([][3]int, 0, 2*len(profile)*n)}
	for _, q := range profile {
		for j := range n {
			if e := ctx.Err(); e != nil {
				return mesh.Mesh{}, e
			}
			sn, cs := math.Sincos(2 * math.Pi * float64(j) / float64(n))
			switch j {
			case 0:
				cs, sn = 1, 0
			case n / 4:
				cs, sn = 0, 1
			case n / 2:
				cs, sn = -1, 0
			case 3 * n / 4:
				cs, sn = 0, -1
			}
			m.Vertices = append(m.Vertices, p.Frame.ToWorld(geom.Vec3{X: q.X * cs, Y: q.X * sn, Z: q.Y}))
		}
	}
	for i := range profile {
		k := (i + 1) % len(profile)
		for j := range n {
			next := (j + 1) % n
			m.Triangles = append(m.Triangles, [3]int{i*n + j, i*n + next, k*n + next}, [3]int{i*n + j, k*n + next, k*n + j})
		}
	}
	if e := m.ValidateSolid(); e != nil {
		return mesh.Mesh{}, e
	}
	return m, nil
}
