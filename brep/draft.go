package brep

import (
	"fmt"
	"math"

	"plainsolid/geom"
)

// DraftPrism joins a planar profile to a scaled copy in a parallel plane.
// Each point p on the upper profile is center+offset+(p-center)*factor,
// where center lies in the input profile plane. The straight generators
// produce planar side faces. Factor must be positive, so intermediate
// sections stay simple and retain their inner wires. Winding is normalized
// as in PrismWithHoles. The factor controls taper; individual side angles
// depend on their distance from center.
func DraftPrism(outer []geom.Vec3, holes [][]geom.Vec3, center, offset geom.Vec3, factor float64) (Solid, error) {
	if !center.IsFinite() || !offset.IsFinite() || factor <= 0 || math.IsInf(factor, 0) || math.IsNaN(factor) {
		return Solid{}, fmt.Errorf("brep: invalid draft parameters")
	}
	s, err := PrismWithHoles(outer, holes, offset)
	if err != nil {
		return Solid{}, err
	}
	n := len(s.vertices) / 2
	if n < 3 {
		return Solid{}, fmt.Errorf("brep: empty draft profile")
	}
	plane, err := s.FacePlane(0)
	if err != nil {
		return Solid{}, err
	}
	scale := 0.
	for _, p := range s.vertices[:n] {
		scale = math.Max(scale, p.Sub(plane.Origin()).Length())
	}
	if d := plane.SignedDistance(center); math.IsNaN(d) || math.Abs(d) > scale*1e-12 {
		return Solid{}, fmt.Errorf("brep: draft center outside profile plane")
	}
	v := s.Vertices()
	for i, p := range v[:n] {
		q := center.Add(p.Sub(center).Scale(factor)).Add(offset)
		if !q.IsFinite() {
			return Solid{}, fmt.Errorf("brep: draft vertex overflow")
		}
		v[n+i] = q
	}
	return NewSolid(v, s.edges, s.faces)
}
