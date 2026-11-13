package trimmed

import (
	"fmt"
	"math"
	"plainsolid/geom"
	"sort"
)

type depthDisk struct {
	unionDisk
	start, depth float64
}
type depthArrangement struct {
	disks  []depthDisk
	points []geom.Vec2
	arcs   []unionArc
}
type directedUnionArc struct {
	id      int
	reverse bool
}
type depthBoundary struct {
	loops           [][]directedUnionArc
	areas           []float64
	area, perimeter float64
}

func newDepthArrangement(input []depthDisk, tol float64) (depthArrangement, error) {
	fail := func(s string) (depthArrangement, error) {
		return depthArrangement{}, fmt.Errorf("trimmed: depth union: %s", s)
	}
	disks := append([]depthDisk(nil), input...)
	sort.Slice(disks, func(i, j int) bool {
		a, b := disks[i], disks[j]
		if a.center.X != b.center.X {
			return a.center.X < b.center.X
		}
		if a.center.Y != b.center.Y {
			return a.center.Y < b.center.Y
		}
		if a.radius != b.radius {
			return a.radius > b.radius
		}
		if a.start != b.start {
			return a.start < b.start
		}
		return a.depth > b.depth
	})
	// Coaxial intervals with the same footprint form one interval when
	// they meet or overlap. This also removes repeated tools without leaving
	// coincident arrangement arcs.
	merged := make([]depthDisk, 0, len(disks))
	for _, d := range disks {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if last.center == d.center && last.radius == d.radius && d.start <= last.depth {
				last.depth = max(last.depth, d.depth)
				continue
			}
		}
		merged = append(merged, d)
	}
	disks = merged
	out := depthArrangement{}
	for i, a := range disks {
		redundant := false
		for j, b := range disks {
			if i == j || a.start < b.start || a.depth > b.depth {
				continue
			}
			if a.radius <= b.radius && circleRelation(a.unionDisk, b.unionDisk, false) <= 0 {
				redundant = true
			}
		}
		if !redundant {
			out.disks = append(out.disks, a)
		}
	}
	type event struct {
		angle  float64
		vertex int
	}
	events := make([][]event, len(out.disks))
	for i, a := range out.disks {
		for j := i + 1; j < len(out.disks); j++ {
			b := out.disks[j]
			if a.start > b.depth || b.start > a.depth {
				continue
			}
			outer := circleRelation(a.unionDisk, b.unionDisk, true)
			if outer > 0 {
				continue
			}
			if outer == 0 {
				return fail("external tangency is nonmanifold")
			}
			inner := circleRelation(a.unionDisk, b.unionDisk, false)
			if a.center == b.center {
				continue
			}
			if inner < 0 {
				continue
			}
			if inner == 0 {
				return fail("internally tangent unequal-depth floor pinches")
			}
			dx, dy := b.center.X-a.center.X, b.center.Y-a.center.Y
			d := math.Hypot(dx, dy)
			scale := math.Max(d, math.Max(a.radius, b.radius))
			ra, rb, dd := a.radius/scale, b.radius/scale, d/scale
			if !finite(d) || dd == 0 || math.Min(ra+rb-dd, dd-math.Abs(ra-rb))*scale <= tol+64*0x1p-52*scale {
				return fail("unresolved near-tangent intersection")
			}
			ca := (dd + (ra-rb)*(ra+rb)/dd) / (2 * ra)
			if math.Abs(ca) >= 1 {
				return fail("unresolved intersection angle")
			}
			along, perp := a.radius*ca, a.radius*math.Sqrt((1-ca)*(1+ca))
			ux, uy := dx/d, dy/d
			for _, sign := range []float64{-1, 1} {
				p := geom.Vec2{X: a.center.X + ux*along - sign*uy*perp, Y: a.center.Y + uy*along + sign*ux*perp}
				if !finite(p.X) || !finite(p.Y) {
					return fail("intersection overflow")
				}
				for k, c := range out.disks {
					if k == i || k == j {
						continue
					}
					distance := math.Hypot(p.X-c.center.X, p.Y-c.center.Y)
					if math.Abs(distance-c.radius) <= tol+128*0x1p-52*math.Max(distance, c.radius) {
						return fail("unresolved triple boundary intersection")
					}
				}
				id := len(out.points)
				out.points = append(out.points, p)
				for _, k := range []int{i, j} {
					c := out.disks[k]
					angle := math.Atan2(p.Y-c.center.Y, p.X-c.center.X)
					if angle < 0 {
						angle += 2 * math.Pi
					}
					events[k] = append(events[k], event{angle, id})
				}
			}
		}
	}
	for i, c := range out.disks {
		es := events[i]
		sort.Slice(es, func(a, b int) bool { return es[a].angle < es[b].angle })
		if len(es) == 0 {
			id := len(out.points)
			out.points = append(out.points, geom.Vec2{X: c.center.X + c.radius, Y: c.center.Y})
			es = []event{{0, id}}
		}
		for j, a := range es {
			b := es[(j+1)%len(es)]
			end := b.angle
			if j == len(es)-1 {
				end += 2 * math.Pi
			}
			if (end-a.angle)*c.radius <= tol {
				return fail("unresolved short arrangement arc")
			}
			// Eighth-turn analytic pieces keep floor seed chords from collapsing
			// across common circle-intersection joins; all layers share these pieces.
			count := int(math.Ceil((end - a.angle) / (math.Pi / 4)))
			start := a.vertex
			for k := 0; k < count; k++ {
				from := a.angle + (end-a.angle)*float64(k)/float64(count)
				to := a.angle + (end-a.angle)*float64(k+1)/float64(count)
				last := b.vertex
				if k+1 < count {
					last = len(out.points)
					out.points = append(out.points, geom.Vec2{X: c.center.X + c.radius*math.Cos(to), Y: c.center.Y + c.radius*math.Sin(to)})
				}
				out.arcs = append(out.arcs, unionArc{i, start, last, from, to})
				start = last
			}
		}
	}
	return out, nil
}

func (a depthArrangement) exposed(lo, hi, tol float64) ([]bool, error) {
	visible := make([]bool, len(a.arcs))
	for i, arc := range a.arcs {
		disk := a.disks[arc.disk]
		if disk.start > lo || hi > disk.depth {
			continue
		}
		covered, err := a.coveredArc(i, lo, hi, tol)
		if err != nil {
			return nil, err
		}
		visible[i] = !covered
	}
	return visible, nil
}

// coveredArc classifies the interior of an arrangement arc against the
// other disks active in one axial slab.
func (a depthArrangement) coveredArc(id int, lo, hi, tol float64) (bool, error) {
	arc := a.arcs[id]
	disk := a.disks[arc.disk]
	theta := (arc.from + arc.to) / 2
	p := geom.Vec2{X: disk.center.X + disk.radius*math.Cos(theta), Y: disk.center.Y + disk.radius*math.Sin(theta)}
	for j, other := range a.disks {
		if j == arc.disk || other.start > lo || hi > other.depth {
			continue
		}
		distance := math.Hypot(p.X-other.center.X, p.Y-other.center.Y)
		margin := tol + 128*0x1p-52*math.Max(distance, other.radius)
		if math.Abs(distance-other.radius) <= margin {
			return false, fmt.Errorf("trimmed: unresolved depth-boundary classification")
		}
		if distance < other.radius {
			return true, nil
		}
	}
	return false, nil
}

func (a depthArrangement) boundary(arcs []directedUnionArc, allowHoles bool, tol float64) (depthBoundary, error) {
	fail := func(s string) (depthBoundary, error) {
		return depthBoundary{}, fmt.Errorf("trimmed: depth boundary: %s", s)
	}
	ends := func(d directedUnionArc) (int, int) {
		arc := a.arcs[d.id]
		if d.reverse {
			return arc.end, arc.start
		}
		return arc.start, arc.end
	}
	next := map[int]int{}
	incoming := map[int]int{}
	for i, d := range arcs {
		x, y := ends(d)
		if _, ok := next[x]; ok {
			return fail("branched or tangent boundary")
		}
		next[x] = i
		incoming[y]++
	}
	for p := range next {
		if incoming[p] != 1 {
			return fail("open boundary")
		}
	}
	out := depthBoundary{}
	seen := make([]bool, len(arcs))
	for first := range arcs {
		if seen[first] {
			continue
		}
		at := first
		anchor := a.disks[a.arcs[arcs[first].id].disk].center
		var loop []directedUnionArc
		area, perimeter := 0., 0.
		for steps := 0; ; steps++ {
			if steps > len(arcs) {
				return fail("traversal limit")
			}
			if seen[at] {
				if at != first {
					return fail("intersecting cycles")
				}
				break
			}
			seen[at] = true
			d := arcs[at]
			arc := a.arcs[d.id]
			c := a.disks[arc.disk]
			from, to := arc.from, arc.to
			if d.reverse {
				from, to = to, from
			}
			area += .5 * (c.radius*c.radius*(to-from) + c.radius*(c.center.X-anchor.X)*(math.Sin(to)-math.Sin(from)) + c.radius*(c.center.Y-anchor.Y)*(math.Cos(from)-math.Cos(to)))
			perimeter += c.radius * math.Abs(to-from)
			loop = append(loop, d)
			_, end := ends(d)
			n, ok := next[end]
			if !ok {
				return fail("open traversal")
			}
			at = n
		}
		if !finite(area) || math.Abs(area) <= tol*tol || !finite(perimeter) {
			return fail("unresolved area")
		}
		if !allowHoles && area < 0 {
			return fail("material-island footprint")
		}
		out.loops = append(out.loops, loop)
		out.areas = append(out.areas, area)
		out.area += area
		out.perimeter += perimeter
	}
	return out, nil
}
func (a depthArrangement) insideLoop(point geom.Vec2, loop []directedUnionArc, tolerance float64) (bool, error) {
	winding := 0
	for _, d := range loop {
		arc := a.arcs[d.id]
		c := a.disks[arc.disk]
		margin := tolerance + 128*0x1p-52*math.Max(c.radius, math.Hypot(point.X-c.center.X, point.Y-c.center.Y))
		start, end := a.points[arc.start], a.points[arc.end]
		// Avoid rays passing through a represented join. The caller can choose
		// another point on the inner loop; ambiguous nesting is never guessed.
		if math.Abs(point.Y-start.Y) <= margin || math.Abs(point.Y-end.Y) <= margin {
			return false, fmt.Errorf("trimmed: unresolved floor nesting ray")
		}
		theta := math.Atan2(point.Y-c.center.Y, point.X-c.center.X)
		near := math.Min(math.Hypot(point.X-start.X, point.Y-start.Y), math.Hypot(point.X-end.X, point.Y-end.Y))
		for k := -2; k <= 3; k++ {
			angle := theta + float64(k)*2*math.Pi
			if angle >= arc.from && angle <= arc.to {
				near = math.Min(near, math.Abs(math.Hypot(point.X-c.center.X, point.Y-c.center.Y)-c.radius))
			}
		}
		if near <= margin {
			return false, fmt.Errorf("trimmed: unresolved floor nesting clearance")
		}
		y := (point.Y - c.center.Y) / c.radius
		if y <= -1 || y >= 1 {
			continue
		}
		angle := math.Asin(y)
		for _, root := range []float64{angle, math.Pi - angle} {
			for k := -2; k <= 3; k++ {
				theta := root + float64(k)*2*math.Pi
				if theta < arc.from || theta >= arc.to {
					continue
				}
				x := c.center.X + c.radius*math.Cos(theta)
				if x == point.X {
					return false, fmt.Errorf("trimmed: unresolved floor nesting")
				}
				if x < point.X {
					continue
				}
				sign := 1
				if math.Cos(theta) < 0 {
					sign = -1
				}
				if d.reverse {
					sign = -sign
				}
				winding += sign
			}
		}
	}
	return winding != 0, nil
}
