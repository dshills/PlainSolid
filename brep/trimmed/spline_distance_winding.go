package trimmed

import (
	"context"
	"errors"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type splineWindingNode struct {
	lo, hi float64
	box    geom.UVBox
	start  geom.Vec2
	end    geom.Vec2
	valid  bool
	left   *splineWindingNode
	right  *splineWindingNode
}

type splineWindingUse struct {
	use     Use
	curve   PCurve
	enclose trimIntervalEnclosure
	reverse bool
	root    *splineWindingNode
}

// A distance call classifies many nearby boxes against the same trim. Reuse
// the exact interval enclosures of its dyadic subdivision tree. A leaf is
// accepted only when its enclosure is disjoint from the current query.
type splineWindingCache struct {
	faces [][][]splineWindingUse
	left  int
}

func newSplineWindingCache(faces int) *splineWindingCache {
	return &splineWindingCache{faces: make([][][]splineWindingUse, faces), left: 16384}
}

func (c *splineWindingCache) init(face int, f Face) {
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		l := make([]splineWindingUse, len(loop))
		for i, u := range loop {
			x := &l[i]
			x.use = u
			if trim, rev, ok := trimTube(u.Curve); ok {
				x.curve, x.reverse = trim, rev
				x.enclose = func(ctx context.Context, a, b float64) (geom.UVBox, bool, error) {
					return trim.EncloseUV(ctx, a, b, geom.SurfaceTubeIntervalOptions{MaxIterations: 4})
				}
			} else if trim, rev, ok := trimCertifiedSpline(u.Curve); ok {
				x.curve, x.reverse = trim, rev
				x.enclose = func(ctx context.Context, a, b float64) (geom.UVBox, bool, error) {
					return trim.EncloseSplineUV(ctx, a, b, geom.SurfaceArcIntervalOptions{MaxIterations: 4})
				}
			}
		}
		c.faces[face] = append(c.faces[face], l)
	}
}

func (c *splineWindingCache) classify(ctx context.Context, face int, f Face, q geom.UVBox) (parametric.Location, bool, error) {
	if c.faces[face] == nil {
		c.init(face, f)
	}
	// SplineUV has its own exact Bézier trim subdivision path.
	for _, loop := range c.faces[face] {
		for _, x := range loop {
			if x.use.Curve != nil && x.enclose == nil {
				return classifyTrimBox(ctx, f, q, 96)
			}
		}
	}
	p := geom.Vec2{X: q.Min.X/2 + q.Max.X/2, Y: q.Min.Y/2 + q.Max.Y/2}
	limit := 192
	sum := 0
	for _, loop := range c.faces[face] {
		for i := range loop {
			if e := ctx.Err(); e != nil {
				return parametric.Outside, false, e
			}
			x := &loop[i]
			u := x.use
			if x.enclose == nil {
				if splineDistanceSegmentMeetsBox(u.UVStart, u.UVEnd, q) {
					return parametric.Outside, false, nil
				}
				sum += splineDistanceWindingEdge(p, u.UVStart, u.UVEnd)
				continue
			}
			if x.root == nil {
				n, e := c.build(ctx, x, 0, 1)
				if e != nil || n == nil {
					return parametric.Outside, false, e
				}
				a, b := n.start, n.end
				if x.reverse {
					a, b = b, a
				}
				if a != u.UVStart || b != u.UVEnd {
					return parametric.Outside, false, geom.ErrConvergence
				}
				x.root = n
			}
			w, ok, e := c.winding(ctx, x, x.root, q, p, &limit)
			if e != nil || !ok {
				return parametric.Outside, false, e
			}
			if x.reverse {
				w = -w
			}
			sum += w
		}
	}
	if sum == 0 {
		return parametric.Outside, true, nil
	}
	if sum == 1 {
		return parametric.Inside, true, nil
	}
	return parametric.Outside, false, geom.ErrConvergence
}

func (c *splineWindingCache) build(ctx context.Context, x *splineWindingUse, a, b float64) (*splineWindingNode, error) {
	if c.left == 0 {
		return nil, nil
	}
	c.left--
	n := &splineWindingNode{lo: a, hi: b}
	box, valid, e := x.enclose(ctx, a, b)
	if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, intersect.ErrUnresolved) {
		return nil, e
	}
	n.start, e = x.curve.EvalUV(a)
	if e != nil {
		return nil, e
	}
	n.end, e = x.curve.EvalUV(b)
	if e != nil {
		return nil, e
	}
	box.Min.X, box.Min.Y = min(box.Min.X, n.start.X, n.end.X), min(box.Min.Y, n.start.Y, n.end.Y)
	box.Max.X, box.Max.Y = max(box.Max.X, n.start.X, n.end.X), max(box.Max.Y, n.start.Y, n.end.Y)
	n.box, n.valid = box, valid
	return n, nil
}

func (c *splineWindingCache) winding(ctx context.Context, x *splineWindingUse, n *splineWindingNode,
	q geom.UVBox, p geom.Vec2, limit *int) (int, bool, error) {
	if e := ctx.Err(); e != nil {
		return 0, false, e
	}
	if *limit == 0 || n == nil {
		return 0, false, nil
	}
	*limit = *limit - 1
	if n.valid && !uvBoxesOverlap(n.box, q) {
		return splineDistanceWindingEdge(p, n.start, n.end), true, nil
	}
	// Once a trim point is enclosed entirely by the query, winding for the
	// whole box is necessarily unresolved. Avoid refining that trim to the
	// precision limit just to rediscover the boundary crossing.
	if n.valid && n.box.Min.X >= q.Min.X && n.box.Min.Y >= q.Min.Y && n.box.Max.X <= q.Max.X && n.box.Max.Y <= q.Max.Y {
		return 0, false, nil
	}
	m := n.lo/2 + n.hi/2
	if m <= n.lo || m >= n.hi {
		return 0, false, nil
	}
	var e error
	if n.left == nil {
		n.left, e = c.build(ctx, x, n.lo, m)
		if e != nil {
			return 0, false, e
		}
	}
	if n.right == nil {
		n.right, e = c.build(ctx, x, m, n.hi)
		if e != nil {
			return 0, false, e
		}
	}
	if n.left != nil && n.left.valid && n.right != nil && n.right.valid {
		contains := func(b geom.UVBox) bool {
			return b.Min.X >= q.Min.X && b.Min.Y >= q.Min.Y && b.Max.X <= q.Max.X && b.Max.Y <= q.Max.Y
		}
		if contains(n.left.box) || contains(n.right.box) {
			return 0, false, nil
		}
	}
	a, ok, e := c.winding(ctx, x, n.left, q, p, limit)
	if e != nil || !ok {
		return 0, false, e
	}
	b, ok, e := c.winding(ctx, x, n.right, q, p, limit)
	return a + b, ok, e
}

func splineDistanceWindingEdge(p, a, b geom.Vec2) int {
	if a.Y <= p.Y && p.Y < b.Y {
		if s, _ := geom.Orient2D(a, b, p); s > 0 {
			return 1
		}
	}
	if b.Y <= p.Y && p.Y < a.Y {
		if s, _ := geom.Orient2D(a, b, p); s < 0 {
			return -1
		}
	}
	return 0
}

func splineDistanceSegmentMeetsBox(a, b geom.Vec2, q geom.UVBox) bool {
	if max(a.X, b.X) < q.Min.X || min(a.X, b.X) > q.Max.X || max(a.Y, b.Y) < q.Min.Y || min(a.Y, b.Y) > q.Max.Y {
		return false
	}
	pos, neg := false, false
	for _, p := range [4]geom.Vec2{q.Min, {X: q.Min.X, Y: q.Max.Y}, q.Max, {X: q.Max.X, Y: q.Min.Y}} {
		s, _ := geom.Orient2D(a, b, p)
		if s == 0 {
			return true
		}
		pos, neg = pos || s > 0, neg || s < 0
	}
	return pos && neg
}
