package plaincadbridge

import (
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Circular ring strips preserve the analytic axial steps without introducing
// axial subdivisions on straight generators. The maximum chord sagitta and
// cylinder-normal deviation are bounded directly by the angular step.
func (p *coaxialBody) tessellate(o parametric.TessellationOptions) (mesh.Mesh, error) {
	if !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	limit := o.MaxSubdivisions
	if limit == 0 {
		limit = 128
	}
	budget := o.MaxTriangles
	if budget == 0 {
		budget = 1000000
	}
	radius := 0.
	for _, s := range p.spans {
		radius = max(radius, s.radius)
	}
	n := 8
	for radius*2*math.Pow(math.Sin(math.Pi/float64(n)/2), 2) > o.Deflection || o.Angle > 0 && math.Pi/float64(n) > o.Angle {
		if n > limit/2 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		n *= 2
	}
	if n > limit || len(p.spans) > budget/(4*n) {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	first, last := p.spans[0], p.spans[len(p.spans)-1]
	profile := []geom.Vec2{{Y: first.lo}, {X: first.radius, Y: first.lo}}
	for i, s := range p.spans {
		profile = append(profile, geom.Vec2{X: s.radius, Y: s.hi})
		if i+1 < len(p.spans) {
			profile = append(profile, geom.Vec2{X: p.spans[i+1].radius, Y: s.hi})
		}
	}
	profile = append(profile, geom.Vec2{Y: last.hi})
	m := mesh.Mesh{}
	rings := make([][]int, len(profile))
	for k, q := range profile {
		ring := make([]int, n)
		if q.X == 0 {
			id := len(m.Vertices)
			m.Vertices = append(m.Vertices, p.frame.ToWorld(geom.Vec3{Z: q.Y}))
			for j := range ring {
				ring[j] = id
			}
		} else {
			for j := range ring {
				angle := 2 * math.Pi * float64(j) / float64(n)
				sn, cs := math.Sincos(angle)
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
				ring[j] = len(m.Vertices)
				m.Vertices = append(m.Vertices, p.frame.ToWorld(geom.Vec3{X: q.X * cs, Y: q.X * sn, Z: q.Y}))
			}
		}
		rings[k] = ring
	}
	for k := 0; k+1 < len(rings); k++ {
		a, b := rings[k], rings[k+1]
		for j := range n {
			next := (j + 1) % n
			if a[j] != a[next] {
				m.Triangles = append(m.Triangles, [3]int{a[j], a[next], b[next]})
			}
			if b[j] != b[next] {
				m.Triangles = append(m.Triangles, [3]int{a[j], b[next], b[j]})
			}
		}
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}
