package trimmed

import (
	"fmt"
	"math"
	"math/big"
	"plainsolid/geom"
	"sort"
)

type unionDisk struct {
	center geom.Vec2
	radius float64
	frame  geom.Frame
}
type unionArc struct {
	disk, start, end int
	from, to         float64
}
type unionProfile struct {
	disks           []unionDisk
	points          []geom.Vec2
	loops           [][]unionArc
	areas           []float64
	area, perimeter float64
}

// circleRelation compares the exact represented squared center distance with
// the squared sum/difference of represented radii, before constructing angles.
func circleRelation(a, b unionDisk, sum bool) int {
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	dx := new(big.Rat).Sub(rat(a.center.X), rat(b.center.X))
	dy := new(big.Rat).Sub(rat(a.center.Y), rat(b.center.Y))
	d2 := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
	r := new(big.Rat).Sub(rat(a.radius), rat(b.radius))
	if sum {
		r.Add(rat(a.radius), rat(b.radius))
	}
	return d2.Cmp(new(big.Rat).Mul(r, r))
}
func diskUnionProfile(disks []unionDisk, tol float64) (unionProfile, error) {
	fail := func(reason string) (unionProfile, error) {
		return unionProfile{}, fmt.Errorf("trimmed: circular union: %s", reason)
	}
	disks = append([]unionDisk(nil), disks...)
	sort.Slice(disks, func(i, j int) bool {
		a, b := disks[i], disks[j]
		if a.center.X != b.center.X {
			return a.center.X < b.center.X
		}
		if a.center.Y != b.center.Y {
			return a.center.Y < b.center.Y
		}
		return a.radius > b.radius
	})
	var active []unionDisk
	for i, a := range disks {
		contained := false
		for j, b := range disks {
			if i == j {
				continue
			}
			if a.center == b.center && a.radius == b.radius {
				if j < i {
					contained = true
				}
				continue
			}
			if a.radius <= b.radius && circleRelation(a, b, false) <= 0 {
				contained = true
			}
		}
		if !contained {
			active = append(active, a)
		}
	}
	out := unionProfile{disks: active}
	type event struct {
		angle  float64
		vertex int
	}
	events := make([][]event, len(active))
	for i, a := range active {
		for j := i + 1; j < len(active); j++ {
			b := active[j]
			relation := circleRelation(a, b, true)
			if relation > 0 {
				continue
			}
			if relation == 0 {
				return fail("externally tangent union is nonmanifold")
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
			along := a.radius * ca
			perp := a.radius * math.Sqrt((1-ca)*(1+ca))
			ux, uy := dx/d, dy/d
			for _, sign := range []float64{-1, 1} {
				p := geom.Vec2{X: a.center.X + ux*along - sign*uy*perp, Y: a.center.Y + uy*along + sign*ux*perp}
				if !finite(p.X) || !finite(p.Y) {
					return fail("intersection coordinate overflow")
				}
				for k, c := range active {
					if k == i || k == j {
						continue
					}
					distance := math.Hypot(p.X-c.center.X, p.Y-c.center.Y)
					if math.Abs(distance-c.radius) <= tol+128*0x1p-52*math.Max(distance, c.radius) {
						return fail("unresolved triple intersection")
					}
				}
				id := len(out.points)
				out.points = append(out.points, p)
				for _, k := range []int{i, j} {
					c := active[k]
					angle := math.Atan2(p.Y-c.center.Y, p.X-c.center.X)
					if angle < 0 {
						angle += 2 * math.Pi
					}
					events[k] = append(events[k], event{angle, id})
				}
			}
		}
	}
	var arcs []unionArc
	for i, c := range active {
		es := events[i]
		sort.Slice(es, func(a, b int) bool { return es[a].angle < es[b].angle })
		if len(es) == 0 {
			p := geom.Vec2{X: c.center.X + c.radius, Y: c.center.Y}
			id := len(out.points)
			out.points = append(out.points, p)
			arcs = append(arcs, unionArc{i, id, id, 0, 2 * math.Pi})
			continue
		}
		for j, a := range es {
			b := es[(j+1)%len(es)]
			end := b.angle
			if j == len(es)-1 {
				end += 2 * math.Pi
			}
			if (end-a.angle)*c.radius <= tol {
				return fail("unresolved short boundary arc")
			}
			middle := (a.angle + end) / 2
			p := geom.Vec2{X: c.center.X + c.radius*math.Cos(middle), Y: c.center.Y + c.radius*math.Sin(middle)}
			exposed := true
			for k, other := range active {
				if k == i {
					continue
				}
				d := math.Hypot(p.X-other.center.X, p.Y-other.center.Y)
				margin := tol + 128*0x1p-52*math.Max(d, other.radius)
				if math.Abs(d-other.radius) <= margin {
					return fail("unresolved boundary classification")
				}
				if d < other.radius {
					exposed = false
					break
				}
			}
			if exposed {
				arcs = append(arcs, unionArc{i, a.vertex, b.vertex, a.angle, end})
			}
		}
	}
	// Exposed directed arcs must form disjoint simple cycles. Inner cycles would
	// leave isolated material islands in a through cut; those are not supported.
	next := map[int]int{}
	incoming := map[int]int{}
	for i, a := range arcs {
		if _, ok := next[a.start]; ok {
			return fail("branched union boundary")
		}
		next[a.start] = i
		incoming[a.end]++
	}
	for p := range next {
		if incoming[p] != 1 {
			return fail("open union boundary")
		}
	}
	visited := make([]bool, len(arcs))
	for first := range arcs {
		if visited[first] {
			continue
		}
		var loop []unionArc
		area, perimeter := 0., 0.
		at := first
		for steps := 0; ; steps++ {
			if steps > len(arcs) {
				return fail("union boundary traversal limit")
			}
			if visited[at] {
				if at != first {
					return fail("intersecting union cycles")
				}
				break
			}
			visited[at] = true
			a := arcs[at]
			c := active[a.disk]
			area += .5 * (c.radius*c.radius*(a.to-a.from) + c.radius*c.center.X*(math.Sin(a.to)-math.Sin(a.from)) + c.radius*c.center.Y*(math.Cos(a.from)-math.Cos(a.to)))
			perimeter += c.radius * (a.to - a.from)
			// Quarter arcs keep the ordinary compact cylinder-wall mesher eligible.
			count := int(math.Ceil((a.to - a.from) / (math.Pi / 2)))
			start := a.start
			for k := 0; k < count; k++ {
				from := a.from + (a.to-a.from)*float64(k)/float64(count)
				to := a.from + (a.to-a.from)*float64(k+1)/float64(count)
				end := a.end
				if k+1 < count {
					end = len(out.points)
					out.points = append(out.points, geom.Vec2{X: c.center.X + c.radius*math.Cos(to), Y: c.center.Y + c.radius*math.Sin(to)})
				}
				loop = append(loop, unionArc{a.disk, start, end, from, to})
				start = end
			}
			n, ok := next[a.end]
			if !ok {
				return fail("open union boundary")
			}
			at = n
		}
		if !finite(area) || !finite(perimeter) || area <= tol*tol {
			return fail("internal material island or unresolved union area")
		}
		out.loops = append(out.loops, loop)
		out.areas = append(out.areas, area)
		out.area += area
		out.perimeter += perimeter
	}
	if len(out.loops) == 0 || !finite(out.area) || !finite(out.perimeter) {
		return fail("empty or unbounded union")
	}
	return out, nil
}

func unionDotSign(a, b geom.Vec3) int {
	x, y := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}
	sum := new(big.Rat)
	for i := range x {
		sum.Add(sum, new(big.Rat).Mul(new(big.Rat).SetFloat64(x[i]), new(big.Rat).SetFloat64(y[i])))
	}
	return sum.Sign()
}
