package geom

import (
	"context"
	"math"
	"math/big"
	"strconv"
)

type SkewOrthogonalCylinderTopology uint8

const (
	SkewCylinderEmpty SkewOrthogonalCylinderTopology = iota
	SkewCylinderIsolatedPoint
	SkewCylinderOneLoop
	SkewCylinderTwoLoops
	SkewCylinderOneNode
	SkewCylinderTwoNodes
)

// SkewOrthogonalCylinderIntersection retains exact projection overlap and
// signed-root nodes. Arcs use the same XSign/ZSign and node conventions as
// PerpendicularCylinderArc, but shared Y is relative to the X-axis centerline.
type SkewOrthogonalCylinderIntersection struct {
	Complete        bool
	Topology        SkewOrthogonalCylinderTopology
	RelativeY       [2]*big.Rat
	AxisOffset      *big.Rat
	Nodes           []PerpendicularCylinderNode
	Arcs            []PerpendicularCylinderArc
	Components      []PerpendicularCylinderComponent
	ExactOperations int
	origin          Vec3
	rx, rz          float64
	delta, lo, hi   *big.Rat
}

// IntersectSkewOrthogonalCylinders classifies complete infinite X/Z cylinder
// support intersections for arbitrary represented centerline Y displacement.
// Exact interval overlap, not a tolerance, decides disjointness and isolated
// tangency. A four-germ endpoint has parallel surface normals and distinct
// curve tangent lines; it is not a regular embedded curve.
func IntersectSkewOrthogonalCylinders(ctx context.Context, originX Vec3, radiusX float64, originZ Vec3, radiusZ float64, o SurfacePatchEvaluationOptions) (out SkewOrthogonalCylinderIntersection, err error) {
	if ctx == nil || !originX.IsFinite() || !originZ.IsFinite() || !finite(radiusX) || !finite(radiusZ) || radiusX <= 0 || radiusZ <= 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Nodes = nil
			out.Arcs = nil
			out.Components = nil
		}
	}()
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	delta := new(big.Rat).Sub(r(originZ.Y), r(originX.Y))
	lo, hi := new(big.Rat).Neg(r(radiusX)), r(radiusX)
	otherLo, otherHi := new(big.Rat).Sub(delta, r(radiusZ)), new(big.Rat).Add(delta, r(radiusZ))
	if otherLo.Cmp(lo) > 0 {
		lo = otherLo
	}
	if otherHi.Cmp(hi) < 0 {
		hi = otherHi
	}
	for _, q := range []*big.Rat{delta, lo, hi} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.AxisOffset = new(big.Rat).Set(delta)
	out.RelativeY = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
	if lo.Cmp(hi) > 0 {
		out.Complete = true
		return out, nil
	}
	out.origin = Vec3{X: originZ.X, Y: originX.Y, Z: originX.Z}
	out.rx, out.rz = radiusX, radiusZ
	out.delta, out.lo, out.hi = new(big.Rat).Set(delta), new(big.Rat).Set(lo), new(big.Rat).Set(hi)
	rx2, rz2 := new(big.Rat).Mul(r(radiusX), r(radiusX)), new(big.Rat).Mul(r(radiusZ), r(radiusZ))
	ids := map[string]int{}
	node := func(y *big.Rat, sx, sz int) (int, error) {
		yz := new(big.Rat).Sub(y, delta)
		dx, dz := new(big.Rat).Sub(rz2, new(big.Rat).Mul(yz, yz)), new(big.Rat).Sub(rx2, new(big.Rat).Mul(y, y))
		if dx.Sign() == 0 {
			sx = 0
		}
		if dz.Sign() == 0 {
			sz = 0
		}
		key := y.RatString() + ":" + strconv.Itoa(sx) + ":" + strconv.Itoa(sz)
		if id, ok := ids[key]; ok {
			return id, nil
		}
		n := PerpendicularCylinderNode{Signs: [3]int{sx, 0, sz}, Radicands: [3]*big.Rat{dx, new(big.Rat), dz}, Offset: [3]*big.Rat{r(originZ.X), new(big.Rat).Add(r(originX.Y), y), r(originX.Z)}, SurfaceTangent: dx.Sign() == 0 && dz.Sign() == 0}
		for i := 0; i < 3; i++ {
			l, h, e := rationalSqrtBounds(n.Radicands[i], 128, b)
			if e != nil {
				return 0, e
			}
			if n.Signs[i] < 0 {
				l, h = new(big.Rat).Neg(h), new(big.Rat).Neg(l)
			}
			n.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(n.Offset[i], l), new(big.Rat).Add(n.Offset[i], h)}
			for _, q := range n.Bounds[i] {
				if e := b.check(q); e != nil {
					return 0, e
				}
			}
		}
		id := len(out.Nodes)
		ids[key] = id
		out.Nodes = append(out.Nodes, n)
		return id, nil
	}
	for _, sx := range []int{-1, 1} {
		for _, sz := range []int{-1, 1} {
			a, e := node(lo, sx, sz)
			if e != nil {
				return out, e
			}
			c, e := node(hi, sx, sz)
			if e != nil {
				return out, e
			}
			if lo.Cmp(hi) == 0 {
				continue
			}
			id := len(out.Arcs)
			out.Arcs = append(out.Arcs, PerpendicularCylinderArc{sx, sz, [2]int{a, c}})
			out.Nodes[a].IncidentArcs = append(out.Nodes[a].IncidentArcs, id)
			out.Nodes[c].IncidentArcs = append(out.Nodes[c].IncidentArcs, id)
		}
	}
	seen := make([]bool, len(out.Nodes))
	for i := range out.Nodes {
		if seen[i] {
			continue
		}
		comp := PerpendicularCylinderComponent{}
		queue := []int{i}
		seen[i] = true
		edges := map[int]bool{}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			comp.Nodes = append(comp.Nodes, n)
			for _, arc := range out.Nodes[n].IncidentArcs {
				if !edges[arc] {
					edges[arc] = true
					comp.Arcs = append(comp.Arcs, arc)
				}
				for _, end := range out.Arcs[arc].Nodes {
					if !seen[end] {
						seen[end] = true
						queue = append(queue, end)
					}
				}
			}
		}
		comp.CycleRank = len(comp.Arcs) - len(comp.Nodes) + 1
		out.Components = append(out.Components, comp)
	}
	if len(out.Arcs) == 0 {
		out.Topology = SkewCylinderIsolatedPoint
	} else {
		nodes := 0
		for _, n := range out.Nodes {
			if n.SurfaceTangent {
				nodes++
			}
		}
		switch {
		case nodes == 2:
			out.Topology = SkewCylinderTwoNodes
		case nodes == 1:
			out.Topology = SkewCylinderOneNode
		case len(out.Components) == 2:
			out.Topology = SkewCylinderTwoLoops
		default:
			out.Topology = SkewCylinderOneLoop
		}
	}
	out.Complete = true
	return out, nil
}

// EvaluateArc is an analytic numerical representative in increasing relative
// Y, excluding endpoint parameter singularities. Exact endpoints are Nodes.
func (p SkewOrthogonalCylinderIntersection) EvaluateArc(arc int, t float64) (point, tangent Vec3, err error) {
	if !p.Complete || p.lo == nil || p.lo.Cmp(p.hi) >= 0 || arc < 0 || arc > 3 || !finite(t) || t <= 0 || t >= 1 {
		return point, tangent, ErrDomain
	}
	lo, _ := p.lo.Float64()
	hi, _ := p.hi.Float64()
	delta, _ := p.delta.Float64()
	y := lo + (hi-lo)*t
	v := y - delta
	x2, z2 := p.rz*p.rz-v*v, p.rx*p.rx-y*y
	if x2 <= 0 || z2 <= 0 || !finite(x2) || !finite(z2) {
		return point, tangent, ErrConvergence
	}
	sx, sz := -1.0, -1.0
	if arc >= 2 {
		sx = 1
	}
	if arc%2 == 1 {
		sz = 1
	}
	x, z := math.Sqrt(x2), math.Sqrt(z2)
	point = p.origin.Add(Vec3{X: sx * x, Y: y, Z: sz * z})
	tangent = Vec3{X: -sx * v * (hi - lo) / x, Y: hi - lo, Z: -sz * y * (hi - lo) / z}
	if !point.IsFinite() || !tangent.IsFinite() {
		return point, tangent, ErrConvergence
	}
	return
}
