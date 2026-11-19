package boolean

import (
	"math"

	"plainsolid/geom"
)

// node is a point of the arrangement (AR1). It stands for every input
// vertex and computed point merged into it: its position is their mean,
// and its radius is ε plus the largest distance from one of them to that
// mean (T2).
type node struct {
	pos     geom.Vec3
	radius  float64
	members []geom.Vec3
	verts   [2][]int // the input vertices of A and of B merged into it
	into    int      // the node this one was merged into, or -1
}

// find follows merges to the node that now stands for n.
func (a *arrangement) find(n int) int {
	for a.nodes[n].into >= 0 {
		n = a.nodes[n].into
	}
	return n
}

// at is the node that now stands for n.
func (a *arrangement) at(n int) *node { return &a.nodes[a.find(n)] }

// add makes a node of radius ε for one vertex or one computed point.
func (a *arrangement) add(p geom.Vec3) (int, error) {
	if a.live >= a.limits.MaxNodes {
		return -1, fail(ErrBudget, "arrangement", "more than MaxNodes %d nodes", a.limits.MaxNodes)
	}
	a.live++
	a.nodes = append(a.nodes, node{pos: p, radius: a.t.eps, members: []geom.Vec3{p}, into: -1})
	return len(a.nodes) - 1, nil
}

// coincident reports whether two nodes are one point under T3: their
// distance is at most the sum of their radii.
func (a *arrangement) coincident(x, y int) bool {
	p, q := a.at(x), a.at(y)
	return p.pos.Distance(q.pos) <= p.radius+q.radius
}

// covers reports whether a computed point p, which has radius ε until it
// is a node, is coincident with node n (T3).
func (a *arrangement) covers(n int, p geom.Vec3) bool {
	return a.at(n).pos.Distance(p) <= a.at(n).radius+a.t.eps
}

// merge makes two nodes one and returns it. A radius above MaxGrowth × ε
// is a chain of near coincidences and returns ErrTolerance (T4).
func (a *arrangement) merge(x, y int) (int, error) {
	x, y = a.find(x), a.find(y)
	if x == y {
		return x, nil
	}
	if y < x {
		x, y = y, x
	}
	keep, gone := &a.nodes[x], &a.nodes[y]
	members := append(append([]geom.Vec3(nil), keep.members...), gone.members...)
	var mean geom.Vec3
	for _, p := range members {
		mean = mean.Add(p.Scale(1 / float64(len(members))))
	}
	radius := a.t.eps
	for _, p := range members {
		radius = math.Max(radius, a.t.eps+p.Distance(mean))
	}
	if radius > a.t.margin {
		return -1, fail(ErrTolerance, "arrangement", "merging nodes at %v needs radius %g, above %g (T4)", mean, radius, a.t.margin)
	}
	keep.pos, keep.radius, keep.members = mean, radius, members
	for i := range keep.verts {
		keep.verts[i] = append(keep.verts[i], gone.verts[i]...)
	}
	gone.into = x
	a.live--
	return x, nil
}
