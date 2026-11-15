package collision

import (
	"plainsolid/geom"
	"plainsolid/mesh"
)

// projectedExpectedContact proves separation or contact confined to original
// shared vertex IDs. Every intersection must project into both triangles.
// A separating projected edge restricts it to the convex hull of the other
// triangle's vertices on its supporting line. Only common indexed endpoints can certify
// an incidental contact; projection-coincident but distinct IDs fall back to
// exact 3-D intersection. The triangles need not be coplanar.
func projectedExpectedContact(m mesh.Mesh, ta, tb int) bool {
	a, b := m.Triangles[ta], m.Triangles[tb]
	pa, pb := triangle(m, ta), triangle(m, tb)
	for _, axes := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		var ap, bp [3]geom.Vec2
		for i := range ap {
			ap[i] = geom.Vec2{X: contactCoordinate(pa[i], axes[0]), Y: contactCoordinate(pa[i], axes[1])}
			bp[i] = geom.Vec2{X: contactCoordinate(pb[i], axes[0]), Y: contactCoordinate(pb[i], axes[1])}
		}
		if projectedSharedSimplex(a, b[:], ap, bp[:]) || projectedSharedSimplex(b, a[:], bp, ap[:]) {
			return true
		}
	}
	return false
}
