package brep

import (
	"fmt"
	"math"
	"plainsolid/geom"
	"sort"
)

// MaxHoledFaceVertices bounds the total vertices of a face with inner wires.
// Tessellation builds a maximal constrained planar graph without Steiner points;
// its worst-case work is cubic in the number of boundary vertices.
const MaxHoledFaceVertices = 256
const faceTolerance = 1e-12

type segment2 struct{ a, b int }

func edgeKey(a, b int) segment2 {
	if a > b {
		a, b = b, a
	}
	return segment2{a, b}
}
func cross2(a, b, c geom.Vec2) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }
func sign2(a, b, c geom.Vec2) int {
	x, y := (b.X-a.X)*(c.Y-a.Y), (b.Y-a.Y)*(c.X-a.X)
	d := x - y
	if math.Abs(d) > 8e-16*(math.Abs(x)+math.Abs(y)) {
		if d > 0 {
			return 1
		}
		return -1
	}
	sign, _ := geom.Orient2D(a, b, c)
	return sign
}
func onSegment2(p, a, b geom.Vec2) bool {
	return sign2(a, b, p) == 0 && p.X >= math.Min(a.X, b.X) && p.X <= math.Max(a.X, b.X) && p.Y >= math.Min(a.Y, b.Y) && p.Y <= math.Max(a.Y, b.Y)
}
func intersects2(a, b, c, d geom.Vec2) bool {
	s1, s2, s3, s4 := sign2(a, b, c), sign2(a, b, d), sign2(c, d, a), sign2(c, d, b)
	if s1*s2 < 0 && s3*s4 < 0 {
		return true
	}
	return s1 == 0 && onSegment2(c, a, b) || s2 == 0 && onSegment2(d, a, b) || s3 == 0 && onSegment2(a, c, d) || s4 == 0 && onSegment2(b, c, d)
}

// location2 returns 1 inside, 0 outside, -1 on a boundary.
func location2(p geom.Vec2, ring []int, points []geom.Vec2) int {
	winding := 0
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		u, v := points[a], points[b]
		if onSegment2(p, u, v) {
			return -1
		}
		if u.Y <= p.Y {
			if v.Y > p.Y && sign2(u, v, p) > 0 {
				winding++
			}
		} else if v.Y <= p.Y && sign2(u, v, p) < 0 {
			winding--
		}
	}
	if winding != 0 {
		return 1
	}
	return 0
}
func area2(ring []int, points []geom.Vec2) float64 {
	sum := 0.0
	origin := points[ring[0]]
	for i, a := range ring {
		b := ring[(i+1)%len(ring)]
		sum += cross2(origin, points[a], points[b])
	}
	return sum
}
func reverseIDs(ids []int) {
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
}

func triangulateRings(outer []geom.Vec3, holes [][]geom.Vec3) ([][3]int, error) {
	if len(outer) < 3 {
		return nil, fmt.Errorf("outer wire needs at least three vertices")
	}
	count := len(outer)
	for _, hole := range holes {
		if len(hole) < 3 {
			return nil, fmt.Errorf("inner wire needs at least three vertices")
		}
		count += len(hole)
	}
	if count > MaxHoledFaceVertices {
		return nil, fmt.Errorf("holed face exceeds %d vertex work limit", MaxHoledFaceVertices)
	}
	origin := outer[0]
	scale := 0.0
	for _, ring := range append([][]geom.Vec3{outer}, holes...) {
		for _, p := range ring {
			if !p.IsFinite() {
				return nil, fmt.Errorf("nonfinite face vertex")
			}
			d := p.Sub(origin)
			scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
		}
	}
	if scale == 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
		return nil, fmt.Errorf("degenerate face scale")
	}
	normal := geom.Vec3{}
	for i, p := range outer {
		normal = normal.Add(p.Sub(origin).Scale(1 / scale).Cross(outer[(i+1)%len(outer)].Sub(origin).Scale(1 / scale)))
	}
	normal, e := normal.Unit()
	if e != nil {
		return nil, e
	}
	axis := 0
	if math.Abs(normal.Y) > math.Abs(normal.X) {
		axis = 1
	}
	if math.Abs(normal.Z) > math.Abs([]float64{normal.X, normal.Y}[axis]) {
		axis = 2
	}
	// Predicate coordinates must preserve the represented input geometry.
	// Subtracting an origin and dividing by an arbitrary scale can turn exact
	// collinearity into a tiny nonzero determinant, producing degenerate model
	// triangles. A power-of-two scaling without translation is exact here.
	maxCoordinate := 0.0
	for _, ring := range append([][]geom.Vec3{outer}, holes...) {
		for _, p := range ring {
			components := [3]float64{p.X, p.Y, p.Z}
			for i, value := range components {
				if i != axis {
					maxCoordinate = math.Max(maxCoordinate, math.Abs(value))
				}
			}
		}
	}
	_, exponent := math.Frexp(maxCoordinate)
	projectScale := func(v float64) float64 { return math.Ldexp(v, -exponent) }
	projectedScale := projectScale(scale)
	resolution := faceTolerance * projectedScale
	areaResolution := faceTolerance * projectedScale * projectedScale
	if (math.IsInf(projectedScale, 0) || math.IsNaN(projectedScale)) || resolution == 0 || areaResolution == 0 {
		return nil, fmt.Errorf("face projection scale is unresolved")
	}
	points := make([]geom.Vec2, 0, count)
	rings := make([][]int, 0, 1+len(holes))
	for _, ring := range append([][]geom.Vec3{outer}, holes...) {
		ids := make([]int, len(ring))
		for i, p := range ring {
			q := p.Sub(origin).Scale(1 / scale)
			if !q.IsFinite() || math.Abs(q.Dot(normal)) > faceTolerance {
				return nil, fmt.Errorf("inner and outer wires must be coplanar")
			}
			var projected geom.Vec2
			switch axis {
			case 0:
				projected = geom.Vec2{X: projectScale(p.Y), Y: projectScale(p.Z)}
			case 1:
				projected = geom.Vec2{X: projectScale(p.X), Y: projectScale(p.Z)}
			case 2:
				projected = geom.Vec2{X: projectScale(p.X), Y: projectScale(p.Y)}
			}
			// Reject an exponent spread that loses represented coordinates rather
			// than silently changing predicate geometry through underflow.
			components := [3]float64{p.X, p.Y, p.Z}
			for component, value := range components {
				if component != axis && math.Ldexp(projectScale(value), exponent) != value {
					return nil, fmt.Errorf("face coordinate exponent spread exceeds exact projection range")
				}
			}
			ids[i] = len(points)
			points = append(points, projected)
		}
		rings = append(rings, ids)
	}
	for i, p := range points {
		for j := 0; j < i; j++ {
			q := points[j]
			if math.Hypot(p.X-q.X, p.Y-q.Y) <= resolution {
				return nil, fmt.Errorf("face wires share coincident or unresolved vertices")
			}
		}
	}
	outerArea := area2(rings[0], points)
	if math.Abs(outerArea) <= areaResolution {
		return nil, fmt.Errorf("degenerate outer wire")
	}
	flip := outerArea < 0
	var boundary []segment2
	for r, ids := range rings {
		area := area2(ids, points)
		if math.Abs(area) <= areaResolution {
			return nil, fmt.Errorf("degenerate wire")
		}
		if r > 0 && (area > 0) == (outerArea > 0) {
			return nil, fmt.Errorf("inner wires must oppose outer winding")
		}
		for i, a := range ids {
			b, c := ids[(i+1)%len(ids)], ids[(i+2)%len(ids)]
			if math.Abs(cross2(points[a], points[b], points[c])) <= areaResolution {
				return nil, fmt.Errorf("collinear or unresolved consecutive wire vertices")
			}
			boundary = append(boundary, segment2{a, b})
		}
		if flip {
			reverseIDs(ids)
		}
	}
	for i, a := range boundary {
		for _, b := range boundary[i+1:] {
			if a.a == b.a || a.a == b.b || a.b == b.a || a.b == b.b {
				continue
			}
			if intersects2(points[a.a], points[a.b], points[b.a], points[b.b]) {
				return nil, fmt.Errorf("face wires intersect or touch")
			}
		}
	}
	for i := 1; i < len(rings); i++ {
		if location2(points[rings[i][0]], rings[0], points) != 1 {
			return nil, fmt.Errorf("hole is not strictly contained in outer wire")
		}
		for j := 1; j < i; j++ {
			if location2(points[rings[i][0]], rings[j], points) != 0 || location2(points[rings[j][0]], rings[i], points) != 0 {
				return nil, fmt.Errorf("holes overlap or nest")
			}
		}
	}
	inside := func(p geom.Vec2) bool {
		if location2(p, rings[0], points) != 1 {
			return false
		}
		for _, ring := range rings[1:] {
			if location2(p, ring, points) != 0 {
				return false
			}
		}
		return true
	}
	// Boundary edges constrain the graph. Both directions record whether the
	// domain is on the left, allowing hole interiors to be excluded without an
	// unreliable centroid test on concave loops.
	var edges []segment2
	present := map[segment2]bool{}
	domainLeft := map[segment2]bool{}
	for _, ring := range rings {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			key := edgeKey(a, b)
			edges = append(edges, key)
			present[key] = true
			domainLeft[segment2{a, b}] = true
			domainLeft[segment2{b, a}] = false
		}
	}
	type candidate struct {
		edge     segment2
		distance float64
	}
	var candidates []candidate
	for a := range points {
		for b := a + 1; b < len(points); b++ {
			key := segment2{a, b}
			if present[key] {
				continue
			}
			p, q := points[a], points[b]
			candidates = append(candidates, candidate{key, (p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y)})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].distance < candidates[j].distance })
	for _, candidate := range candidates {
		a, b := candidate.edge.a, candidate.edge.b
		p, q := points[a], points[b]
		if !inside(geom.Vec2{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2}) {
			continue
		}
		valid := true
		for i, r := range points {
			if i != a && i != b && onSegment2(r, p, q) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		for _, edge := range edges {
			if edge.a == a || edge.a == b || edge.b == a || edge.b == b {
				continue
			}
			if intersects2(p, q, points[edge.a], points[edge.b]) {
				valid = false
				break
			}
		}
		if valid {
			edges = append(edges, candidate.edge)
			present[candidate.edge] = true
		}
	}
	neighbors := make([][]int, len(points))
	for _, edge := range edges {
		neighbors[edge.a] = append(neighbors[edge.a], edge.b)
		neighbors[edge.b] = append(neighbors[edge.b], edge.a)
	}
	for i := range neighbors {
		origin := points[i]
		half := func(p geom.Vec2) int {
			y, x := p.Y-origin.Y, p.X-origin.X
			if y > 0 || y == 0 && x >= 0 {
				return 0
			}
			return 1
		}
		sort.SliceStable(neighbors[i], func(a, b int) bool {
			p, q := points[neighbors[i][a]], points[neighbors[i][b]]
			ha, hb := half(p), half(q)
			if ha != hb {
				return ha < hb
			}
			return sign2(origin, p, q) > 0
		})
	}
	visited := map[segment2]bool{}
	var triangles [][3]int
	for _, edge := range edges {
		for _, start := range []segment2{edge, {edge.b, edge.a}} {
			if visited[start] {
				continue
			}
			current := start
			var cycle []int
			excluded := false
			for steps := 0; ; steps++ {
				if steps > 2*len(edges) {
					return nil, fmt.Errorf("constrained face walk failed")
				}
				if visited[current] {
					if current != start {
						return nil, fmt.Errorf("constrained graph is not manifold")
					}
					break
				}
				visited[current] = true
				cycle = append(cycle, current.a)
				if left, exists := domainLeft[current]; exists && !left {
					excluded = true
				}
				order := neighbors[current.b]
				index := -1
				for i, v := range order {
					if v == current.a {
						index = i
						break
					}
				}
				if index < 0 {
					return nil, fmt.Errorf("constrained graph lacks reverse edge")
				}
				current = segment2{current.b, order[(index+len(order)-1)%len(order)]}
			}
			if excluded {
				continue
			}
			if len(cycle) != 3 {
				return nil, fmt.Errorf("constrained graph did not triangulate a face")
			}
			if sign2(points[cycle[0]], points[cycle[1]], points[cycle[2]]) <= 0 {
				return nil, fmt.Errorf("invalid constrained triangle orientation")
			}
			t := [3]int{cycle[0], cycle[1], cycle[2]}
			if flip {
				t[1], t[2] = t[2], t[1]
			}
			triangles = append(triangles, t)
		}
	}
	if len(triangles) != count+2*len(holes)-2 {
		return nil, fmt.Errorf("constrained triangle count mismatch")
	}
	expected := 0.0
	for _, ring := range rings {
		expected += area2(ring, points)
	}
	actual := 0.0
	for _, t := range triangles {
		actual += math.Abs(cross2(points[t[0]], points[t[1]], points[t[2]]))
	}
	if math.Abs(actual-expected) > 1e-10*math.Max(projectedScale*projectedScale, expected) {
		return nil, fmt.Errorf("constrained triangulation area mismatch")
	}
	return triangles, nil
}
