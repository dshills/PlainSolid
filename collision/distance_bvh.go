package collision

import (
	"container/heap"
	"context"
	"fmt"
	"sort"
)

type distanceNode struct {
	box                              distanceItem
	left, right                      *distanceNode
	triangle, minTriangle, count, id int
}

func buildDistanceTree(ctx context.Context, items []distanceItem) (*distanceNode, error) {
	ids := make([]int, len(items))
	for i := range ids {
		ids[i] = i
	}
	nextID := 0
	var build func([]int) (*distanceNode, error)
	build = func(ids []int) (*distanceNode, error) {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		n := &distanceNode{triangle: -1, minTriangle: ids[0], count: len(ids), id: nextID}
		nextID++
		n.box.lo, n.box.hi = items[ids[0]].lo, items[ids[0]].hi
		for _, i := range ids {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if i < n.minTriangle {
				n.minTriangle = i
			}
			for k := 0; k < 3; k++ {
				if items[i].lo[k].Cmp(n.box.lo[k]) < 0 {
					n.box.lo[k] = items[i].lo[k]
				}
				if items[i].hi[k].Cmp(n.box.hi[k]) > 0 {
					n.box.hi[k] = items[i].hi[k]
				}
			}
		}
		if len(ids) == 1 {
			n.triangle = ids[0]
			return n, nil
		}
		axis := 0
		extent := sub(n.box.hi[0], n.box.lo[0])
		for k := 1; k < 3; k++ {
			d := sub(n.box.hi[k], n.box.lo[k])
			if d.Cmp(extent) > 0 {
				axis, extent = k, d
			}
		}
		// Sorting exact doubled box centers avoids overflow or rounding-dependent
		// partitions. The original index resolves every equal-center tie.
		centers := make(map[int]scalar, len(ids))
		for _, i := range ids {
			centers[i] = add(items[i].lo[axis], items[i].hi[axis])
		}
		sort.Slice(ids, func(i, j int) bool {
			cmp := centers[ids[i]].Cmp(centers[ids[j]])
			if cmp != 0 {
				return cmp < 0
			}
			return ids[i] < ids[j]
		})
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		middle := len(ids) / 2
		var e error
		n.left, e = build(ids[:middle])
		if e != nil {
			return nil, e
		}
		n.right, e = build(ids[middle:])
		if e != nil {
			return nil, e
		}
		return n, nil
	}
	return build(ids)
}

type distancePair struct {
	a, b  *distanceNode
	lower scalar
}
type distanceQueue []distancePair

func (q distanceQueue) Len() int { return len(q) }
func (q distanceQueue) Less(i, j int) bool {
	a, b := q[i], q[j]
	if c := a.lower.Cmp(b.lower); c != 0 {
		return c < 0
	}
	if a.a.minTriangle != b.a.minTriangle {
		return a.a.minTriangle < b.a.minTriangle
	}
	if a.b.minTriangle != b.b.minTriangle {
		return a.b.minTriangle < b.b.minTriangle
	}
	if a.a.id != b.a.id {
		return a.a.id < b.a.id
	}
	return a.b.id < b.b.id
}
func (q distanceQueue) Swap(i, j int)     { q[i], q[j] = q[j], q[i] }
func (q *distanceQueue) Push(x any)       { *q = append(*q, x.(distancePair)) }
func (q *distanceQueue) Pop() any         { a := *q; x := a[len(a)-1]; *q = a[:len(a)-1]; return x }
func earlierPair(ai, bi, aj, bj int) bool { return ai < aj || ai == aj && bi < bj }
func searchDistanceTrees(ctx context.Context, aa, bb []distanceItem, limit int) (closest, int, int, int, int, error) {
	a, e := buildDistanceTree(ctx, aa)
	if e != nil {
		return closest{}, 0, 0, 0, 0, e
	}
	b, e := buildDistanceTree(ctx, bb)
	if e != nil {
		return closest{}, 0, 0, 0, 0, e
	}
	var best closest
	ai, bi, pairs, narrow := 0, 0, 0, 0
	queue := &distanceQueue{}
	prune := func(p distancePair) bool {
		if best.squared == nil {
			return false
		}
		comparison := p.lower.Cmp(best.squared)
		return comparison > 0 || comparison == 0 && !earlierPair(p.a.minTriangle, p.b.minTriangle, ai, bi)
	}
	push := func(a, b *distanceNode) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if pairs >= limit {
			return fmt.Errorf("%w: distance BVH pair budget", ErrLimit)
		}
		pairs++
		p := distancePair{a, b, separation(a.box, b.box)}
		if !prune(p) {
			heap.Push(queue, p)
		}
		return nil
	}
	if e = push(a, b); e != nil {
		return closest{}, 0, 0, pairs, narrow, e
	}
	for queue.Len() > 0 {
		if e = ctx.Err(); e != nil {
			return closest{}, 0, 0, pairs, narrow, e
		}
		p := heap.Pop(queue).(distancePair)
		if prune(p) {
			continue
		}
		if p.a.triangle >= 0 && p.b.triangle >= 0 {
			i, j := p.a.triangle, p.b.triangle
			narrow++
			candidate := closestTriangles(aa[i].t, bb[j].t)
			cmp := -1
			if best.squared != nil {
				cmp = candidate.squared.Cmp(best.squared)
			}
			if cmp < 0 || cmp == 0 && earlierPair(i, j, ai, bi) {
				best, ai, bi = candidate, i, j
			}
			continue
		}
		if p.b.triangle >= 0 || p.a.triangle < 0 && p.a.count >= p.b.count {
			if e = push(p.a.left, p.b); e != nil {
				return closest{}, 0, 0, pairs, narrow, e
			}
			if e = push(p.a.right, p.b); e != nil {
				return closest{}, 0, 0, pairs, narrow, e
			}
		} else {
			if e = push(p.a, p.b.left); e != nil {
				return closest{}, 0, 0, pairs, narrow, e
			}
			if e = push(p.a, p.b.right); e != nil {
				return closest{}, 0, 0, pairs, narrow, e
			}
		}
	}
	return best, ai, bi, pairs, narrow, nil
}
