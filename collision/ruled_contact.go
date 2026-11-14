package collision

import (
	"plainsolid/geom"
	"plainsolid/mesh"
)

func contactCoordinate(v geom.Vec3, k int) float64 {
	switch k {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}
func projectedSharedSimplex(a [3]int, b []int, ap [3]geom.Vec2, bp []geom.Vec2) bool {
	side, e := geom.Orient2D(ap[0], ap[1], ap[2])
	if e != nil || side == 0 {
		return false
	}
	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		outside, shared := true, true
		zeros := 0
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
		if outside && shared && zeros < 3 {
			return true
		}
	}
	return false
}

// ruledExpectedContact is an exact positive-only certificate. A coordinate
// plane cuts a triangle lying wholly in one closed halfspace in precisely the
// convex hull of its zero-coordinate vertices. Alternatively, exact collinear
// transverse projections prove a common axial ruled plane; (transverse,z) is
// an injective affine chart for that plane. In both cases exact orientation
// proves separation or contact confined to the original indexed simplex.
// Crossings, duplicate-position IDs and unresolved cases use the full kernel.
func ruledExpectedContact(m mesh.Mesh, ta, tb int) bool {
	a, b := m.Triangles[ta], m.Triangles[tb]
	cap := func(a, b [3]int) bool {
		for axis := 0; axis < 3; axis++ {
			level := contactCoordinate(m.Vertices[a[0]], axis)
			if contactCoordinate(m.Vertices[a[1]], axis) != level || contactCoordinate(m.Vertices[a[2]], axis) != level {
				continue
			}
			positive, negative := false, false
			var zero [3]int
			count := 0
			for _, id := range b {
				v := contactCoordinate(m.Vertices[id], axis)
				if v > level {
					positive = true
				} else if v < level {
					negative = true
				} else {
					zero[count] = id
					count++
				}
			}
			if positive && negative || count == 3 {
				continue
			}
			if count == 0 {
				return true
			}
			var ap [3]geom.Vec2
			var bp [3]geom.Vec2
			for i, id := range a {
				p := m.Vertices[id]
				ap[i] = geom.Vec2{X: contactCoordinate(p, (axis+1)%3), Y: contactCoordinate(p, (axis+2)%3)}
			}
			for i, id := range zero[:count] {
				p := m.Vertices[id]
				bp[i] = geom.Vec2{X: contactCoordinate(p, (axis+1)%3), Y: contactCoordinate(p, (axis+2)%3)}
			}
			if projectedSharedSimplex(a, zero[:count], ap, bp[:count]) {
				return true
			}
		}
		return false
	}
	if cap(a, b) || cap(b, a) {
		return true
	}
	ids := [6]int{a[0], a[1], a[2], b[0], b[1], b[2]}
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		var projected [6]geom.Vec2
		for i, id := range ids {
			p := m.Vertices[id]
			projected[i] = geom.Vec2{X: contactCoordinate(p, u), Y: contactCoordinate(p, v)}
		}
		first := projected[0]
		other := -1
		for i, p := range projected {
			if p != first {
				other = i
				break
			}
		}
		if other < 0 {
			continue
		}
		second := projected[other]
		collinear := true
		for _, p := range projected {
			if p == first || p == second {
				continue
			}
			sign, e := geom.Orient2D(first, second, p)
			if e != nil || sign != 0 {
				collinear = false
				break
			}
		}
		if !collinear {
			continue
		}
		transverse := u
		if first.X == second.X {
			transverse = v
		}
		var ap, bp [3]geom.Vec2
		for i := 0; i < 3; i++ {
			x, y := m.Vertices[a[i]], m.Vertices[b[i]]
			ap[i] = geom.Vec2{X: contactCoordinate(x, transverse), Y: contactCoordinate(x, axis)}
			bp[i] = geom.Vec2{X: contactCoordinate(y, transverse), Y: contactCoordinate(y, axis)}
		}
		return projectedSharedSimplex(a, b[:], ap, bp[:]) || projectedSharedSimplex(b, a[:], bp, ap[:])
	}
	return false
}
