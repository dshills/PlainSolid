package collision

import (
	"plainsolid/geom"
	"plainsolid/mesh"
)

// coordinatePlaneExpectedContact proves that two represented triangles are
// disjoint or meet only in their indexed shared simplex. It never equates
// duplicate positions with different IDs. Strict edge-halfplane separation
// confines any intersection to the convex hull of the zero-side vertices;
// those vertices must be actual shared edge endpoints. All other cases retain
// the general exact rational intersection predicate. No tolerance is used.
func coordinatePlaneExpectedContact(m mesh.Mesh, ta, tb int) bool {
	a, b := m.Triangles[ta], m.Triangles[tb]
	component := func(v geom.Vec3, k int) float64 {
		switch k {
		case 0:
			return v.X
		case 1:
			return v.Y
		default:
			return v.Z
		}
	}
	axis := -1
	for k := 0; k < 3; k++ {
		v := component(m.Vertices[a[0]], k)
		same := true
		for _, tri := range [][3]int{a, b} {
			for _, id := range tri {
				if component(m.Vertices[id], k) != v {
					same = false
					break
				}
			}
		}
		if same {
			axis = k
			break
		}
	}
	if axis < 0 {
		return false
	}
	var ap, bp [3]geom.Vec2
	for i := 0; i < 3; i++ {
		ap[i] = geom.Vec2{X: component(m.Vertices[a[i]], (axis+1)%3), Y: component(m.Vertices[a[i]], (axis+2)%3)}
		bp[i] = geom.Vec2{X: component(m.Vertices[b[i]], (axis+1)%3), Y: component(m.Vertices[b[i]], (axis+2)%3)}
	}
	separated := func(a, b [3]int, ap, bp [3]geom.Vec2) bool {
		side, e := geom.Orient2D(ap[0], ap[1], ap[2])
		if e != nil || side == 0 {
			return false
		}
		for i := 0; i < 3; i++ {
			j := (i + 1) % 3
			outside := true
			zeros := 0
			shared := true
			for k, id := range b {
				sign := 0
				if id != a[i] && id != a[j] {
					var e error
					sign, e = geom.Orient2D(ap[i], ap[j], bp[k])
					if e != nil {
						return false
					}
				}
				if sign*side > 0 {
					outside = false
					break
				}
				if sign == 0 {
					zeros++
					if id != a[i] && id != a[j] {
						shared = false
					}
				}
			}
			if outside && zeros < 3 && shared {
				return true
			}
		}
		return false
	}
	return separated(a, b, ap, bp) || separated(b, a, bp, ap)
}
