package geom

import (
	"context"
	"math"
	"math/big"
)

type PerpendicularCylinderTopology uint8

const (
	PerpendicularCylinderTwoLoops PerpendicularCylinderTopology = iota
	PerpendicularCylinderTwoNodalEllipses
)

// PerpendicularCylinderNode has exact coordinates Offset[i]+Signs[i]*sqrt(
// Radicands[i]). IncidentArcs retain all germs at equal-radius singular nodes.
type PerpendicularCylinderNode struct {
	Offset         [3]*big.Rat
	Signs          [3]int
	Radicands      [3]*big.Rat
	Bounds         [3][2]*big.Rat
	IncidentArcs   []int
	SurfaceTangent bool
}
type PerpendicularCylinderArc struct {
	XSign, ZSign int
	Nodes        [2]int
}
type PerpendicularCylinderComponent struct {
	Nodes, Arcs []int
	CycleRank   int
}

// PerpendicularCylinderIntersection is an immutable source-backed result;
// exported topology/evidence is caller-owned and cannot alter curve evaluation.
// Four analytic arcs traverse increasing shared Y from -minRadius to +minRadius.
type PerpendicularCylinderIntersection struct {
	Complete        bool
	Topology        PerpendicularCylinderTopology
	Nodes           []PerpendicularCylinderNode
	Arcs            []PerpendicularCylinderArc
	Components      []PerpendicularCylinderComponent
	ExactOperations int
	origin          Vec3
	rx, rz, m       float64
}

// IntersectPerpendicularCylinders handles infinite X-axis and Z-axis circular
// supports whose centerlines intersect exactly (the represented Y origins
// must agree). Arbitrary nonparallel axes, skew axes and finite caps are not
// included. Unequal radii are everywhere transverse; equal radii have parallel
// surface normals at two nodes, but distinct curve tangent lines x=±z.
func IntersectPerpendicularCylinders(ctx context.Context, originX Vec3, radiusX float64, originZ Vec3, radiusZ float64, o SurfacePatchEvaluationOptions) (out PerpendicularCylinderIntersection, err error) {
	if ctx == nil || !originX.IsFinite() || !originZ.IsFinite() || !finite(radiusX) || !finite(radiusZ) || radiusX <= 0 || radiusZ <= 0 {
		return out, ErrInvalid
	}
	if originX.Y != originZ.Y {
		return out, ErrDomain
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
	m := math.Min(radiusX, radiusZ)
	m2 := new(big.Rat).Mul(r(m), r(m))
	dx := new(big.Rat).Sub(new(big.Rat).Mul(r(radiusZ), r(radiusZ)), m2)
	dz := new(big.Rat).Sub(new(big.Rat).Mul(r(radiusX), r(radiusX)), m2)
	for _, q := range []*big.Rat{dx, dz, m2} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	xlo, xhi, e := rationalSqrtBounds(dx, 128, b)
	if e != nil {
		return out, e
	}
	zlo, zhi, e := rationalSqrtBounds(dz, 128, b)
	if e != nil {
		return out, e
	}
	out.origin = Vec3{X: originZ.X, Y: originX.Y, Z: originX.Z}
	out.rx, out.rz, out.m = radiusX, radiusZ, m
	if radiusX == radiusZ {
		out.Topology = PerpendicularCylinderTwoNodalEllipses
	}
	ids := map[[3]int]int{}
	for _, sx := range []int{-1, 1} {
		for _, sz := range []int{-1, 1} {
			arc := PerpendicularCylinderArc{XSign: sx, ZSign: sz}
			for end, sy := range []int{-1, 1} {
				kx, kz := sx, sz
				if dx.Sign() == 0 {
					kx = 0
				}
				if dz.Sign() == 0 {
					kz = 0
				}
				key := [3]int{kx, sy, kz}
				id, ok := ids[key]
				if !ok {
					id = len(out.Nodes)
					ids[key] = id
					n := PerpendicularCylinderNode{Signs: [3]int{kx, 0, kz}, Radicands: [3]*big.Rat{new(big.Rat).Set(dx), new(big.Rat), new(big.Rat).Set(dz)}, SurfaceTangent: dx.Sign() == 0 && dz.Sign() == 0}
					for i := 0; i < 3; i++ {
						n.Offset[i] = r(coordinate(out.origin, i))
					}
					n.Offset[1].Add(n.Offset[1], new(big.Rat).Mul(big.NewRat(int64(sy), 1), r(m)))
					for i, interval := range [3][2]*big.Rat{{xlo, xhi}, {new(big.Rat), new(big.Rat)}, {zlo, zhi}} {
						lo, hi := interval[0], interval[1]
						sign := n.Signs[i]
						if sign < 0 {
							lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
						} else if sign == 0 {
							lo, hi = new(big.Rat), new(big.Rat)
						}
						n.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(n.Offset[i], lo), new(big.Rat).Add(n.Offset[i], hi)}
						for _, q := range n.Bounds[i] {
							if e := b.check(q); e != nil {
								return out, e
							}
						}
					}
					out.Nodes = append(out.Nodes, n)
				}
				arc.Nodes[end] = id
			}
			id := len(out.Arcs)
			out.Arcs = append(out.Arcs, arc)
			for _, node := range arc.Nodes {
				out.Nodes[node].IncidentArcs = append(out.Nodes[node].IncidentArcs, id)
			}
		}
	}
	seen := make([]bool, len(out.Nodes))
	for i := range out.Nodes {
		if seen[i] {
			continue
		}
		c := PerpendicularCylinderComponent{}
		queue := []int{i}
		seen[i] = true
		edges := map[int]bool{}
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			c.Nodes = append(c.Nodes, node)
			for _, arc := range out.Nodes[node].IncidentArcs {
				if !edges[arc] {
					edges[arc] = true
					c.Arcs = append(c.Arcs, arc)
				}
				for _, end := range out.Arcs[arc].Nodes {
					if !seen[end] {
						seen[end] = true
						queue = append(queue, end)
					}
				}
			}
		}
		c.CycleRank = len(c.Arcs) - len(c.Nodes) + 1
		out.Components = append(out.Components, c)
	}
	out.Complete = true
	return out, nil
}

// EvaluateArc returns numerical model point and derivative for 0<t<1.
// Parameter is shared Y=-minRadius+2*minRadius*t; endpoint derivatives may
// diverge and are deliberately excluded. Exact endpoint evidence is in Nodes.
// Arc index ordering is XSign=-1,+1 outer and ZSign=-1,+1 inner.
func (p PerpendicularCylinderIntersection) EvaluateArc(arc int, t float64) (point, tangent Vec3, err error) {
	if !p.Complete || p.m <= 0 || arc < 0 || arc > 3 || !finite(t) || t <= 0 || t >= 1 {
		return point, tangent, ErrDomain
	}
	sx, sz := -1.0, -1.0
	if arc >= 2 {
		sx = 1
	}
	if arc%2 == 1 {
		sz = 1
	}
	y := p.m * (2*t - 1)
	x2, z2 := p.rz*p.rz-y*y, p.rx*p.rx-y*y
	if x2 <= 0 || z2 <= 0 || !finite(x2) || !finite(z2) {
		return point, tangent, ErrConvergence
	}
	x, z := math.Sqrt(x2), math.Sqrt(z2)
	point = p.origin.Add(Vec3{X: sx * x, Y: y, Z: sz * z})
	tangent = Vec3{X: -sx * y * 2 * p.m / x, Y: 2 * p.m, Z: -sz * y * 2 * p.m / z}
	if !point.IsFinite() || !tangent.IsFinite() {
		return point, tangent, ErrConvergence
	}
	return
}
