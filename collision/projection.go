package collision

import (
	"container/heap"
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// ProjectionOptions bounds preparation and each query separately. Defaults are
// 4096 triangles and 100000 exact BVH node-bound tests. Signed additionally validates the
// embedded oriented material boundary through NewPointLocator, using Location.
type ProjectionOptions struct {
	MaxTriangles, MaxWork int
	Signed                bool
	Location              LocationOptions
}

// ProjectionResult reports the exact nearest boundary witness and winning
// original polygon/triangle. Distance is nonnegative; SignedDistance is negative
// inside, positive outside, zero on the boundary only when HasLocation is true.
// Rationals are caller-owned. Equal minima choose the lowest triangle index.
type ProjectionResult struct {
	Point                    geom.Vec3
	ExactPoint               [3]*big.Rat
	SquaredDistance          *big.Rat
	Distance, SignedDistance float64
	Face, Triangle           int
	HasLocation              bool
	Location                 LocationResult
	Work, NarrowPhase        int
}

// PointProjector is immutable and supports concurrent queries. Its zero value
// is an empty boundary. Unsigned queries require valid nondegenerate triangles;
// signed queries additionally certify embedding and material orientation.
type PointProjector struct {
	tree    *distanceNode
	items   []distanceItem
	faces   []int
	locator *PointLocator
	maxWork int
}

func NewPointProjector(ctx context.Context, s brep.Solid, o ProjectionOptions) (*PointProjector, error) {
	if ctx == nil || o.MaxTriangles < 0 || o.MaxWork < 0 {
		return nil, ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 4096
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000
	}
	if o.MaxTriangles > MaxTriangles {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	items, e := distanceItems(ctx, s, o.MaxTriangles)
	if e != nil {
		return nil, e
	}
	p := &PointProjector{items: items, faces: s.TriangleFaces(), maxWork: o.MaxWork}
	if len(items) > 0 {
		p.tree, e = buildDistanceTree(ctx, items)
		if e != nil {
			return nil, e
		}
	}
	if o.Signed {
		p.locator, e = NewPointLocator(ctx, s, o.Location)
		if e != nil {
			return nil, e
		}
	}
	return p, nil
}

// Project uses exact rational face and edge projections, and exact AABB lower
// bounds. It projects onto the actual trimmed polygon region through its
// conforming triangles, so witnesses cannot fall into holes or concavities.
func (p *PointProjector) Project(ctx context.Context, v geom.Vec3) (ProjectionResult, error) {
	empty := ProjectionResult{}
	if ctx == nil || !v.IsFinite() {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if p == nil || len(p.items) == 0 {
		return empty, ErrEmpty
	}
	x := exactTriangle([3]geom.Vec3{v, v, v})[0]
	query := distanceItem{lo: x, hi: x}
	var best closest
	winner := -1
	work, narrow := 0, 0

	queryNode := &distanceNode{box: query}
	queue := &distanceQueue{}
	prune := func(bound scalar, node *distanceNode) bool {
		if best.squared == nil {
			return false
		}
		c := bound.Cmp(best.squared)
		return c > 0 || c == 0 && node.minTriangle >= winner
	}
	push := func(node *distanceNode) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if work >= p.maxWork {
			return ErrLimit
		}
		work++
		bound := separation(query, node.box)
		if !prune(bound, node) {
			heap.Push(queue, distancePair{a: queryNode, b: node, lower: bound})
		}
		return nil
	}
	if e := push(p.tree); e != nil {
		return empty, e
	}
	for queue.Len() > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		entry := heap.Pop(queue).(distancePair)
		node := entry.b
		if prune(entry.lower, node) {
			continue
		}
		if node.triangle < 0 {
			if e := push(node.left); e != nil {
				return empty, e
			}
			if e := push(node.right); e != nil {
				return empty, e
			}
			continue
		}
		i := node.triangle
		item := p.items[i]
		narrow++
		var candidate closest
		t := exactTriangle(item.t)
		if q, ok := faceProjection(x, t); ok {
			candidate.consider(x, q)
		}
		for j := range t {
			candidate.consider(x, projection(x, t[j], t[(j+1)%3]))
		}
		if best.squared == nil || candidate.squared.Cmp(best.squared) < 0 || candidate.squared.Cmp(best.squared) == 0 && i < winner {
			best = candidate
			winner = i
		}
	}
	d, e := distanceResult(best)
	if e != nil {
		return empty, e
	}
	r := ProjectionResult{Point: d.PointB, ExactPoint: d.ExactPointB, SquaredDistance: d.SquaredDistance, Distance: d.Distance, Face: p.faces[winner], Triangle: winner, Work: work, NarrowPhase: narrow}
	if p.locator != nil {
		r.Location, e = p.locator.Locate(ctx, v)
		if e != nil {
			return empty, e
		}
		r.HasLocation = true
		r.SignedDistance = r.Distance
		if r.Location.Location == Inside {
			r.SignedDistance = -r.Distance
		}
	}
	return r, nil
}

// ProjectBoundary is the one-shot equivalent of NewPointProjector then Project.
func ProjectBoundary(ctx context.Context, s brep.Solid, p geom.Vec3, o ProjectionOptions) (ProjectionResult, error) {
	q, e := NewPointProjector(ctx, s, o)
	if e != nil {
		return ProjectionResult{}, e
	}
	return q.Project(ctx, p)
}
