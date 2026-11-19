package collision

import (
	"math/big"
	"plainsolid/geom"
	"runtime"
)

type scalar = *big.Rat
type point [3]scalar

func number(x float64) scalar       { return new(big.Rat).SetFloat64(x) }
func add(a, b scalar) scalar        { return new(big.Rat).Add(a, b) }
func sub(a, b scalar) scalar        { return new(big.Rat).Sub(a, b) }
func mul(a, b scalar) scalar        { return new(big.Rat).Mul(a, b) }
func div(a, b scalar) scalar        { return new(big.Rat).Quo(a, b) }
func exact(p geom.Vec3) point       { return point{number(p.X), number(p.Y), number(p.Z)} }
func plus(a, b point) point         { return point{add(a[0], b[0]), add(a[1], b[1]), add(a[2], b[2])} }
func minus(a, b point) point        { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func scale(a point, s scalar) point { return point{mul(a[0], s), mul(a[1], s), mul(a[2], s)} }
func dot(a, b point) scalar         { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func same(a, b point) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 && a[2].Cmp(b[2]) == 0 }
func nonzeroAxis(p point) int {
	for i, v := range p {
		if v.Sign() != 0 {
			return i
		}
	}
	return -1
}
func interpolate(a, b point, t scalar) point { return plus(a, scale(minus(b, a), t)) }
func orient(a, b, c point, drop int) scalar {
	u, v := (drop+1)%3, (drop+2)%3
	r := sub(mul(sub(b[u], a[u]), sub(c[v], a[v])), mul(sub(b[v], a[v]), sub(c[u], a[u])))
	runtime.KeepAlive(&a) // Stack-map defect, js/wasm by-value parameter form (see docs/STATUS.md): keeps a in the stack map. Remove once Go is fixed.
	runtime.KeepAlive(&b)
	runtime.KeepAlive(&c)
	return r
}
func triangleNormal(t [3]point) point { return cross(minus(t[1], t[0]), minus(t[2], t[0])) }
func unique(points []point) []point {
	result := make([]point, 0, len(points))
	for _, p := range points {
		found := false
		for _, q := range result {
			if same(p, q) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, p)
		}
	}
	return result
}
func exactTriangle(t [3]geom.Vec3) [3]point { return [3]point{exact(t[0]), exact(t[1]), exact(t[2])} }
func strictlyInside(p point, t [3]point, n point) bool {
	axis := nonzeroAxis(n)
	sign := orient(t[0], t[1], t[2], axis).Sign()
	for i, a := range t {
		if orient(a, t[(i+1)%3], p, axis).Sign()*sign <= 0 {
			return false
		}
	}
	return true
}
func sliceTriangle(t [3]point, n point, d scalar) []point {
	var result []point
	var distances [3]scalar
	for i, p := range t {
		distances[i] = sub(dot(n, p), d)
		if distances[i].Sign() == 0 {
			result = append(result, p)
		}
	}
	for i, a := range t {
		j := (i + 1) % 3
		if distances[i].Sign()*distances[j].Sign() < 0 {
			ratio := div(distances[i], sub(distances[i], distances[j]))
			result = append(result, interpolate(a, t[j], ratio))
		}
	}
	return unique(result)
}
func extrema(points []point, axis int) (point, point) {
	low, high := points[0], points[0]
	for _, p := range points[1:] {
		if p[axis].Cmp(low[axis]) < 0 {
			low = p
		}
		if p[axis].Cmp(high[axis]) > 0 {
			high = p
		}
	}
	return low, high
}

func triangleIntersection(a, b [3]geom.Vec3) (Kind, []point) {
	return preparedTriangleIntersection(prepareExactTriangle(a), prepareExactTriangle(b))
}

func preparedTriangleIntersection(a, b preparedExactTriangle) (Kind, []point) {
	ta, tb := a.vertices, b.vertices
	na, nb := a.normal, b.normal
	da, db := a.offset, b.offset
	line := cross(na, nb)
	axis := nonzeroAxis(line)
	if axis < 0 {
		if sub(dot(na, tb[0]), da).Sign() != 0 {
			return Disjoint, nil
		}
		return coplanar(ta, tb, nonzeroAxis(na))
	}
	sa, sb := sliceTriangle(ta, nb, db), sliceTriangle(tb, na, da)
	if len(sa) == 0 || len(sb) == 0 {
		return Disjoint, nil
	}
	alo, ahi := extrema(sa, axis)
	blo, bhi := extrema(sb, axis)
	low, high := alo, ahi
	if blo[axis].Cmp(low[axis]) > 0 {
		low = blo
	}
	if bhi[axis].Cmp(high[axis]) < 0 {
		high = bhi
	}
	comparison := low[axis].Cmp(high[axis])
	if comparison > 0 {
		return Disjoint, nil
	}
	if comparison == 0 {
		return Touching, []point{low}
	}
	midpoint := scale(plus(low, high), big.NewRat(1, 2))
	kind := Touching
	if strictlyInside(midpoint, ta, na) && strictlyInside(midpoint, tb, nb) {
		kind = Crossing
	}
	return kind, []point{low, high}
}
func coplanar(a, b [3]point, axis int) (Kind, []point) {
	polygon := []point{a[0], a[1], a[2]}
	sign := orient(b[0], b[1], b[2], axis).Sign()
	for i, start := range b {
		end := b[(i+1)%3]
		var next []point
		if len(polygon) == 0 {
			return Disjoint, nil
		}
		previous := polygon[len(polygon)-1]
		pd := orient(start, end, previous, axis)
		for _, current := range polygon {
			cd := orient(start, end, current, axis)
			pin, cin := pd.Sign()*sign >= 0, cd.Sign()*sign >= 0
			if pin != cin {
				ratio := div(pd, sub(pd, cd))
				next = append(next, interpolate(previous, current, ratio))
			}
			if cin {
				next = append(next, current)
			}
			previous, pd = current, cd
		}
		polygon = unique(next)
	}
	if len(polygon) == 0 {
		return Disjoint, nil
	}
	if len(polygon) == 1 {
		return Touching, polygon
	}
	area := new(big.Rat)
	for i := 1; i+1 < len(polygon); i++ {
		area.Add(area, orient(polygon[0], polygon[i], polygon[i+1], axis))
	}
	if area.Sign() != 0 {
		return CoplanarOverlap, polygon
	}
	direction := minus(polygon[1], polygon[0])
	low, high := extrema(polygon, nonzeroAxis(direction))
	if same(low, high) {
		return Touching, []point{low}
	}
	return Touching, []point{low, high}
}
func rounded(points []point) []geom.Vec3 {
	result := make([]geom.Vec3, len(points))
	for i, p := range points {
		x, _ := p[0].Float64()
		y, _ := p[1].Float64()
		z, _ := p[2].Float64()
		result[i] = geom.Vec3{X: x, Y: y, Z: z}
	}
	return result
}
func onSegment(p, a, b point) bool {
	if nonzeroAxis(cross(minus(p, a), minus(b, a))) >= 0 {
		return false
	}
	for i := 0; i < 3; i++ {
		lo, hi := a[i], b[i]
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if p[i].Cmp(lo) < 0 || p[i].Cmp(hi) > 0 {
			return false
		}
	}
	return true
}
