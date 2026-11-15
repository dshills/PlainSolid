package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/geom"
	"plainsolid/mesh"
)

func (s stored) analyticDisplayMesh(q request) (mesh.Mesh, [][3]geom.Vec3, bool, error) {
	o, err := tessellationOptions(q)
	if err != nil {
		return mesh.Mesh{}, nil, true, err
	}
	if s.analytic != nil && s.analytic.stepArcLoop != nil {
		// The source-aware sampler includes exact axis support extrema.
		// Generic compact charts sample only at evenly spaced parameters.
		var normals [][3]geom.Vec3
		m, ok, err := s.analytic.stepArcLoop.tessellateNormals(o, &normals)
		if ok {
			return m, normals, true, err
		}
	}
	if s.cylinder != nil {
		m, err := s.tessellate(q)
		if err != nil {
			return mesh.Mesh{}, nil, true, err
		}
		frame := s.cylinder.frame
		normals := make([][3]geom.Vec3, len(m.Triangles))
		for i, tri := range m.Triangles {
			a, b, c := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
			n, err := b.Sub(a).Cross(c.Sub(a)).Unit()
			if err != nil {
				return mesh.Mesh{}, nil, true, err
			}
			if math.Abs(n.Dot(frame.ZAxis())) > .5 {
				continue
			}
			for j, id := range tri {
				p := frame.ToLocal(m.Vertices[id])
				normals[i][j], _ = frame.XAxis().Scale(p.X).Add(frame.YAxis().Scale(p.Y)).Unit()
			}
		}
		return m, normals, true, nil
	}
	if s.treated != nil && s.treated.bore != nil {
		o.Angle = min(o.Angle, .045)
	}
	if s.analytic != nil && (s.analytic.segment != nil || s.analytic.arcLoop != nil) {
		o.Angle = min(o.Angle, .025)
	}
	native, placement, ok := s.retainedTrimmed()
	if !ok || s.analytic != nil && (s.analytic.helical || s.analytic.patches != nil || s.analytic.coaxial != nil) {
		return mesh.Mesh{}, nil, false, nil
	}
	m, normals, ok, err := native.TessellateCompactNormals(context.Background(), o)
	if err != nil || !ok || placement == geom.Identity() {
		return m, normals, ok, err
	}
	for i, p := range m.Vertices {
		m.Vertices[i] = placement.Apply(p)
	}
	for i := range normals {
		for j, n := range normals[i] {
			normals[i][j] = placement.ApplyVector(n)
		}
	}
	return m, normals, true, nil
}
