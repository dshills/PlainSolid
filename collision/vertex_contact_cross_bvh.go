package collision

import (
	"context"

	"plainsolid/mesh"
)

// Compare exact vertex bounds from two already validated, separately indexed
// meshes. The same exact intersection predicate and assembly visitor arbitrate
// candidate triangles as in the sweep path.
func vertexContactCrossBVH(ctx context.Context, a, b mesh.Mesh, limits Options,
	visit func(int, int, Kind, []point) error) (Report, error) {
	var report Report
	var trees [2]*distanceNode
	var prepared [2]exactTriangleMemo
	for j, m := range [2]mesh.Mesh{a, b} {
		items := make([]distanceItem, len(m.Triangles))
		for i := range items {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			it := &items[i]
			it.t = triangle(m, i)
			raw := exactTriangle(it.t)
			it.lo, it.hi = raw[0], raw[0]
			for _, p := range raw[1:] {
				for axis, q := range p {
					if q.Cmp(it.lo[axis]) < 0 {
						it.lo[axis] = q
					}
					if q.Cmp(it.hi[axis]) > 0 {
						it.hi[axis] = q
					}
				}
			}
		}
		tree, err := buildDistanceTree(ctx, items)
		if err != nil {
			return report, err
		}
		trees[j] = tree
		prepared[j] = newExactTriangleMemo(m)
	}
	stack := [][2]*distanceNode{trees}
	hits := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if report.BroadPhasePairs >= limits.MaxPairs {
			return report, ErrLimit
		}
		report.BroadPhasePairs++
		nodes := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		left, right := nodes[0], nodes[1]
		if !vertexContactExactBoxesMeet(left.box, right.box) {
			continue
		}
		if left.triangle >= 0 && right.triangle >= 0 {
			report.NarrowPhasePairs++
			i, j := left.triangle, right.triangle
			k, points := preparedTriangleIntersection(prepared[0].get(i), prepared[1].get(j))
			if k == Disjoint {
				continue
			}
			if hits >= limits.MaxContacts {
				return report, ErrLimit
			}
			hits++
			if err := visit(i, j, k, points); err != nil {
				return report, err
			}
			continue
		}
		if right.triangle >= 0 || left.triangle < 0 && left.count >= right.count {
			stack = append(stack, [2]*distanceNode{left.right, right}, [2]*distanceNode{left.left, right})
		} else {
			stack = append(stack, [2]*distanceNode{left, right.right}, [2]*distanceNode{left, right.left})
		}
	}
	report.Complete = true
	return report, nil
}

func vertexContactExactBoxesMeet(a, b distanceItem) bool {
	for i := range a.lo {
		if a.hi[i].Cmp(b.lo[i]) < 0 || b.hi[i].Cmp(a.lo[i]) < 0 {
			return false
		}
	}
	return true
}
