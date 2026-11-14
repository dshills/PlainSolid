package brep

import (
	"context"
	"errors"
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
	return p.X >= math.Min(a.X, b.X) && p.X <= math.Max(a.X, b.X) && p.Y >= math.Min(a.Y, b.Y) && p.Y <= math.Max(a.Y, b.Y) && sign2(a, b, p) == 0
}
func intersects2(a, b, c, d geom.Vec2) bool {
	if math.Max(a.X, b.X) < math.Min(c.X, d.X) || math.Max(c.X, d.X) < math.Min(a.X, b.X) || math.Max(a.Y, b.Y) < math.Min(c.Y, d.Y) || math.Max(c.Y, d.Y) < math.Min(a.Y, b.Y) {
		return false
	}
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

var ErrTriangulationLimit = errors.New("brep: planar triangulation work limit exceeded")

// PlanarTriangulationOptions bounds vertices and conservative point/edge visits.
// Existing solid constructors retain their 256-vertex domain.
type PlanarTriangulationOptions struct{ MaxVertices, MaxOperations int }

// TriangulatePlanarRings validates planar simple loops, preserves all input
// vertices, and returns indices into outer followed by each hole. Inner loops
// must oppose the outer winding. No Steiner vertices or approximations occur.
func TriangulatePlanarRings(ctx context.Context, outer []geom.Vec3, holes [][]geom.Vec3, o PlanarTriangulationOptions) ([][3]int, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxVertices < 3 || o.MaxVertices > 2048 || o.MaxOperations < 1 || o.MaxOperations > 128000000 {
		return nil, geom.ErrInvalid
	}
	return triangulateRingsWork(ctx, outer, holes, o.MaxVertices, o.MaxOperations)
}
func triangulateRings(outer []geom.Vec3, holes [][]geom.Vec3) ([][3]int, error) {
	return triangulateRingsWork(context.Background(), outer, holes, MaxHoledFaceVertices, 0)
}
func triangulateRingsWork(ctx context.Context, outer []geom.Vec3, holes [][]geom.Vec3, maximum, budget int) ([][3]int, error) {
	visits := 0
	step := func(n int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if budget > 0 && n > budget-visits {
			return ErrTriangulationLimit
		}
		visits += n
		return nil
	}
	if e := step(0); e != nil {
		return nil, e
	}

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
	if count > maximum {
		return nil, ErrTriangulationLimit
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
		if e := step(i); e != nil {
			return nil, e
		}
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
		if e := step(len(boundary) - i); e != nil {
			return nil, e
		}
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
	type ringBox struct{ lo, hi geom.Vec2 }
	boxes := make([]ringBox, len(rings))
	for i, ring := range rings {
		boxes[i] = ringBox{points[ring[0]], points[ring[0]]}
		for _, id := range ring {
			p := points[id]
			boxes[i].lo.X = min(boxes[i].lo.X, p.X)
			boxes[i].lo.Y = min(boxes[i].lo.Y, p.Y)
			boxes[i].hi.X = max(boxes[i].hi.X, p.X)
			boxes[i].hi.Y = max(boxes[i].hi.Y, p.Y)
		}
	}
	inside := func(p geom.Vec2) (bool, error) {
		for i, ring := range rings {
			if e := step(1); e != nil {
				return false, e
			}
			box := boxes[i]
			if p.X < box.lo.X || p.X > box.hi.X || p.Y < box.lo.Y || p.Y > box.hi.Y {
				if i == 0 {
					return false, nil
				}
				continue
			}
			if e := step(len(ring)); e != nil {
				return false, e
			}
			location := location2(p, ring, points)
			if i == 0 && location != 1 || i > 0 && location != 0 {
				return false, nil
			}
		}
		return true, nil
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
	grid := newPlanarEdgeGrid(points)
	for i, edge := range edges {
		if e := step(grid.add(points[edge.a], points[edge.b], i)); e != nil {
			return nil, e
		}
	}
	type candidate struct {
		edge     segment2
		distance float64
	}
	// A diagonal whose endpoints belong to one strictly convex hole lies
	// inside that excluded region. Prove convexity once instead of repeatedly
	// testing all of its chords against the entire constrained graph.
	owner := make([]int, len(points))
	convexHole := make([]bool, len(rings))
	for ri, ring := range rings {
		if ri == 0 {
			continue
		}
		convexHole[ri] = true
		for i, id := range ring {
			owner[id] = ri
			if sign2(points[id], points[ring[(i+1)%len(ring)]], points[ring[(i+2)%len(ring)]]) >= 0 {
				convexHole[ri] = false
			}
		}
	}
	// A candidate must leave both endpoints into the local material wedge.
	// Ring orientation puts material on the left, including around holes.
	// Strict wrong-side tests preserve boundary-collinear candidates for the
	// subsequent full intersection proof.
	previous, next := make([]int, len(points)), make([]int, len(points))
	for _, ring := range rings {
		for i, id := range ring {
			previous[id] = ring[(i+len(ring)-1)%len(ring)]
			next[id] = ring[(i+1)%len(ring)]
		}
	}
	enters := func(a, b int) bool {
		p, q := points[a], points[b]
		leftIncoming := sign2(points[previous[a]], p, q)
		leftOutgoing := sign2(p, points[next[a]], q)
		turn := sign2(points[previous[a]], p, points[next[a]])
		if turn > 0 {
			return leftIncoming >= 0 && leftOutgoing >= 0
		}
		return leftIncoming >= 0 || leftOutgoing >= 0
	}
	var candidates []candidate
	for a := range points {
		if e := step(len(points) - a); e != nil {
			return nil, e
		}
		for b := a + 1; b < len(points); b++ {
			key := segment2{a, b}
			if owner[a] > 0 && owner[a] == owner[b] && convexHole[owner[a]] {
				continue
			}
			if present[key] {
				continue
			}
			if !enters(a, b) || !enters(b, a) {
				continue
			}
			p, q := points[a], points[b]
			candidates = append(candidates, candidate{key, (p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y)})
		}
	}
	// Certify completed triangular faces once. A later diagonal entering one
	// of their vertex wedges must cross its opposite edge: the triangle has
	// no other boundary vertex where that diagonal could terminate.
	neighborsDone := make([][]int, len(points))
	for _, edge := range edges {
		neighborsDone[edge.a] = append(neighborsDone[edge.a], edge.b)
		neighborsDone[edge.b] = append(neighborsDone[edge.b], edge.a)
	}
	sealed := make([][][2]int, len(points))
	seal := func(a, b int) error {
		if e := step(len(neighborsDone[a])); e != nil {
			return e
		}
		for _, c := range neighborsDone[a] {
			if c == b || !present[edgeKey(b, c)] {
				continue
			}
			x, y, z := a, b, c
			turn := sign2(points[x], points[y], points[z])
			if turn == 0 {
				continue
			}
			if turn < 0 {
				y, z = z, y
			}
			if e := step(len(points)); e != nil {
				return e
			}
			empty := true
			loX, hiX := min(points[x].X, points[y].X, points[z].X), max(points[x].X, points[y].X, points[z].X)
			loY, hiY := min(points[x].Y, points[y].Y, points[z].Y), max(points[x].Y, points[y].Y, points[z].Y)
			for id, p := range points {
				if id == x || id == y || id == z || p.X < loX || p.X > hiX || p.Y < loY || p.Y > hiY {
					continue
				}
				if sign2(points[x], points[y], p) >= 0 && sign2(points[y], points[z], p) >= 0 && sign2(points[z], points[x], p) >= 0 {
					empty = false
					break
				}
			}
			if empty {
				sealed[x] = append(sealed[x], [2]int{y, z})
				sealed[y] = append(sealed[y], [2]int{z, x})
				sealed[z] = append(sealed[z], [2]int{x, y})
			}
		}
		return nil
	}
	blocked := func(a, b int) bool {
		for _, w := range sealed[a] {
			if sign2(points[a], points[w[0]], points[b]) >= 0 && sign2(points[a], points[b], points[w[1]]) >= 0 {
				return true
			}
		}
		return false
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].distance < candidates[j].distance })
	for _, candidate := range candidates {
		// A connected constrained triangulation has exactly this many edges.
		// Further noncrossing diagonals cannot be added once it is complete.
		if len(edges) == 2*len(points)+3*len(holes)-3 {
			break
		}
		if e := step(1); e != nil {
			return nil, e
		}
		a, b := candidate.edge.a, candidate.edge.b
		if e := step(len(sealed[a]) + len(sealed[b])); e != nil {
			return nil, e
		}
		if blocked(a, b) || blocked(b, a) {
			continue
		}
		p, q := points[a], points[b]
		contained, e := inside(geom.Vec2{X: (p.X + q.X) / 2, Y: (p.Y + q.Y) / 2})
		if e != nil {
			return nil, e
		}
		if !contained {
			continue
		}
		valid := true
		nearby, work := grid.query(p, q)
		if e := step(work); e != nil {
			return nil, e
		}
		for _, id := range nearby {
			edge := edges[id]
			// Every boundary vertex is incident to an indexed edge. Test both
			// endpoints before skipping shared candidate endpoints, preserving the
			// original rejection of a diagonal through any other model vertex.
			for _, v := range []int{edge.a, edge.b} {
				if v != a && v != b && onSegment2(points[v], p, q) {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
			if edge.a == a || edge.a == b || edge.b == a || edge.b == b {
				continue
			}
			if intersects2(p, q, points[edge.a], points[edge.b]) {
				valid = false
				break
			}
		}
		if valid {
			if e := step(grid.add(p, q, len(edges))); e != nil {
				return nil, e
			}
			edges = append(edges, candidate.edge)
			present[candidate.edge] = true
			if e := seal(a, b); e != nil {
				return nil, e
			}
			neighborsDone[a] = append(neighborsDone[a], b)
			neighborsDone[b] = append(neighborsDone[b], a)
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
		if e := step(1); e != nil {
			return nil, e
		}
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
