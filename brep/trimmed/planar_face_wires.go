package trimmed

import (
	"plainsolid/geom"
)

// PlanarStraightFaceWithCircularHoles resolves the exact straight outer
// boundary and complete analytic hole circles in a planar face's native chart.
func (s Solid) PlanarStraightFaceWithCircularHoles(id int) (geom.Frame, []geom.Vec3, []ExtrusionCircle, error) {
	if id < 0 || id >= len(s.faces) {
		return geom.Frame{}, nil, nil, geom.ErrDomain
	}
	f := s.faces[id]
	plane, ok := f.Surface.(geom.PlaneSurface)
	if !ok || !prismCardinal(plane.Frame()) {
		return geom.Frame{}, nil, nil, geom.ErrDomain
	}
	outer := make([]geom.Vec3, len(f.Outer))
	for i, use := range f.Outer {
		if use.Curve != nil {
			return geom.Frame{}, nil, nil, geom.ErrDomain
		}
		id := s.edges[use.Edge].Start
		if use.Reversed {
			id = s.edges[use.Edge].End
		}
		outer[i] = s.vertices[id]
	}
	holes := make([]ExtrusionCircle, len(f.Holes))
	for i, wire := range f.Holes {
		c, ok := prismWholeCircle(wire)
		if !ok {
			return geom.Frame{}, nil, nil, geom.ErrDomain
		}
		holes[i] = ExtrusionCircle{Center: c.Center, Radius: c.Cos.X}
	}
	return plane.Frame(), outer, holes, nil
}
