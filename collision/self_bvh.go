package collision

import (
	"context"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// SelfIntersectionsBVHContext uses a deterministic bounding-volume hierarchy
// and the same exact predicates/incidence rules as SelfIntersections. No
// geometric tolerance is introduced. Unlike the sweep API, MaxPairs and
// BroadPhasePairs count BOTH visited node pairs and leaf triangle candidates.
// This bounds traversal overhead even when all candidates prune. MaxContacts
// retains its meaning; partial reports never set Complete. Cancellation is
// checked during input validation, tree construction and pair traversal.
// At most MaxTriangles triangles and 3*MaxTriangles vertices are accepted.
func SelfIntersectionsBVHContext(ctx context.Context, m mesh.Mesh, o Options) (report Report, err error) {
	defer func() {
		sort.Slice(report.Contacts, func(i, j int) bool {
			a, b := report.Contacts[i], report.Contacts[j]
			if a.TriangleA != b.TriangleA {
				return a.TriangleA < b.TriangleA
			}
			return a.TriangleB < b.TriangleB
		})
	}()
	if ctx == nil || o.MaxPairs < 0 || o.MaxContacts < 0 {
		return report, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return report, e
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = DefaultMaxPairs
	}
	if o.MaxContacts == 0 {
		o.MaxContacts = DefaultMaxContacts
	}
	if len(m.Triangles) > MaxTriangles || len(m.Vertices) > 3*MaxTriangles {
		return report, ErrLimit
	}
	for _, v := range m.Vertices {
		if e := ctx.Err(); e != nil {
			return report, e
		}
		if !v.IsFinite() {
			return report, ErrInvalid
		}
	}
	all := make([]item, len(m.Triangles))
	for i, ids := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return report, e
		}
		for _, id := range ids {
			if id < 0 || id >= len(m.Vertices) {
				return report, ErrInvalid
			}
		}
		tri := [3]geom.Vec3{m.Vertices[ids[0]], m.Vertices[ids[1]], m.Vertices[ids[2]]}
		if e := validateTriangle(tri); e != nil {
			return report, e
		}
		it := item{triangle: i, min: [3]float64{tri[0].X, tri[0].Y, tri[0].Z}, max: [3]float64{tri[0].X, tri[0].Y, tri[0].Z}}
		for _, p := range tri[1:] {
			for axis, v := range []float64{p.X, p.Y, p.Z} {
				it.min[axis] = math.Min(it.min[axis], v)
				it.max[axis] = math.Max(it.max[axis], v)
			}
		}
		all[i] = it
	}
	if len(all) < 2 {
		report.Complete = true
		return report, nil
	}
	type node struct {
		min, max                [3]float64
		start, end, left, right int
	}
	nodes := make([]node, 0, 2*len(all))
	center := func(it item, axis int) float64 { return it.min[axis]/2 + it.max[axis]/2 }
	var build func(int, int) (int, error)
	build = func(start, end int) (int, error) {
		if e := ctx.Err(); e != nil {
			return 0, e
		}
		n := node{min: all[start].min, max: all[start].max, start: start, end: end, left: -1, right: -1}
		var low, high [3]float64
		for k := 0; k < 3; k++ {
			low[k] = center(all[start], k)
			high[k] = low[k]
		}
		for _, it := range all[start+1 : end] {
			for k := 0; k < 3; k++ {
				n.min[k] = math.Min(n.min[k], it.min[k])
				n.max[k] = math.Max(n.max[k], it.max[k])
				c := center(it, k)
				low[k] = math.Min(low[k], c)
				high[k] = math.Max(high[k], c)
			}
		}
		id := len(nodes)
		nodes = append(nodes, n)
		if end-start > 4 {
			axis := 0
			for k := 1; k < 3; k++ {
				if high[k]-low[k] > high[axis]-low[axis] {
					axis = k
				}
			}
			sort.Slice(all[start:end], func(i, j int) bool {
				a, b := all[start+i], all[start+j]
				ca, cb := center(a, axis), center(b, axis)
				if ca != cb {
					return ca < cb
				}
				return a.triangle < b.triangle
			})
			mid := start + (end-start)/2
			left, e := build(start, mid)
			if e != nil {
				return 0, e
			}
			right, e := build(mid, end)
			if e != nil {
				return 0, e
			}
			nodes[id].left, nodes[id].right = left, right
		}
		return id, nil
	}
	root, e := build(0, len(all))
	if e != nil {
		return report, e
	}
	overlap := func(amin, amax, bmin, bmax [3]float64) bool {
		for k := 0; k < 3; k++ {
			if amax[k] < bmin[k] || bmax[k] < amin[k] {
				return false
			}
		}
		return true
	}
	charge := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if report.BroadPhasePairs >= o.MaxPairs {
			return ErrLimit
		}
		report.BroadPhasePairs++
		return nil
	}
	check := func(a, b item) error {
		if e := charge(); e != nil {
			return e
		}
		if !overlap(a.min, a.max, b.min, b.max) {
			return nil
		}
		ta, tb := a.triangle, b.triangle
		if ta > tb {
			ta, tb = tb, ta
		}
		report.NarrowPhasePairs++
		kind, points := triangleIntersection(triangle(m, ta), triangle(m, tb))
		if kind == Disjoint || expectedSharedContact(m, ta, tb, kind, points) {
			return nil
		}
		if len(report.Contacts) >= o.MaxContacts {
			return ErrLimit
		}
		report.Contacts = append(report.Contacts, Contact{TriangleA: ta, TriangleB: tb, Intersection: Intersection{Kind: kind, Points: rounded(points)}})
		return nil
	}
	stack := [][2]int{{root, root}}
	for len(stack) > 0 {
		if e := charge(); e != nil {
			return report, e
		}
		pair := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		a, b := nodes[pair[0]], nodes[pair[1]]
		if !overlap(a.min, a.max, b.min, b.max) {
			continue
		}
		if pair[0] == pair[1] {
			if a.left >= 0 {
				stack = append(stack, [2]int{a.right, a.right}, [2]int{a.left, a.right}, [2]int{a.left, a.left})
			} else {
				for i := a.start; i < a.end; i++ {
					for j := i + 1; j < a.end; j++ {
						if e := check(all[i], all[j]); e != nil {
							return report, e
						}
					}
				}
			}
			continue
		}
		if a.left < 0 && b.left < 0 {
			for i := a.start; i < a.end; i++ {
				for j := b.start; j < b.end; j++ {
					if e := check(all[i], all[j]); e != nil {
						return report, e
					}
				}
			}
			continue
		}
		if a.left >= 0 && (b.left < 0 || a.end-a.start >= b.end-b.start) {
			stack = append(stack, [2]int{a.right, pair[1]}, [2]int{a.left, pair[1]})
		} else {
			stack = append(stack, [2]int{pair[0], b.right}, [2]int{pair[0], b.left})
		}
	}
	if e := ctx.Err(); e != nil {
		return report, e
	}
	report.Complete = true
	return report, nil
}
