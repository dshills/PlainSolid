package geom

import (
	"context"
	"math"
	"math/big"
	"strconv"
)

// PerpendicularCylinderCapMask uses XMin,XMax,ZMin,ZMax bits in that order.
type PerpendicularCylinderCapMask uint8

const (
	PerpendicularCapXMin PerpendicularCylinderCapMask = 1 << iota
	PerpendicularCapXMax
	PerpendicularCapZMin
	PerpendicularCapZMax
)

type FinitePerpendicularCylinderNode struct {
	PerpendicularCylinderNode
	Caps, CapTangencies PerpendicularCylinderCapMask
}

// SquaredY is ordered increasing; YSign chooses the shared-coordinate half.
// Nodes follow increasing squaredY, so negative-half arcs traverse decreasing Y.
type FinitePerpendicularCylinderArc struct {
	XSign, YSign, ZSign int
	SquaredY            [2]*big.Rat
	Nodes               [2]int
}
type FinitePerpendicularCylinderComponent struct {
	Nodes, Arcs                          []int
	CycleRank                            int
	IsolatedPoint, HasBoundary, Singular bool
}
type FinitePerpendicularCylinderIntersection struct {
	Complete        bool
	Nodes           []FinitePerpendicularCylinderNode
	Arcs            []FinitePerpendicularCylinderArc
	Components      []FinitePerpendicularCylinderComponent
	ExactOperations int
}

// ClipPerpendicularCylinderIntersection clips the complete support graph by
// closed MODEL-coordinate X and Z intervals (not distances from either supplied
// axis origin). Both windows must have positive length. Each branch's x² and
// z² constraints reduce to rational bounds on y²; all exact algebraic endpoint
// incidences, cap owners, tangent point remnants and equal-radius nodes remain.
func ClipPerpendicularCylinderIntersection(ctx context.Context, originX Vec3, radiusX float64, originZ Vec3, radiusZ float64, xRange, zRange [2]*big.Rat, o SurfacePatchEvaluationOptions) (out FinitePerpendicularCylinderIntersection, err error) {
	for _, w := range [][2]*big.Rat{xRange, zRange} {
		if w[0] == nil || w[1] == nil || w[0].Cmp(w[1]) >= 0 {
			return out, ErrInvalid
		}
	}
	support, e := IntersectPerpendicularCylinders(ctx, originX, radiusX, originZ, radiusZ, o)
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: support.ExactOperations}
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
	rx2, rz2 := new(big.Rat).Mul(r(radiusX), r(radiusX)), new(big.Rat).Mul(r(radiusZ), r(radiusZ))
	m := math.Min(radiusX, radiusZ)
	m2 := new(big.Rat).Mul(r(m), r(m))
	crossing := Vec3{X: originZ.X, Y: originX.Y, Z: originX.Z}
	caps := [4]*big.Rat{new(big.Rat).Sub(xRange[0], r(crossing.X)), new(big.Rat).Sub(xRange[1], r(crossing.X)), new(big.Rat).Sub(zRange[0], r(crossing.Z)), new(big.Rat).Sub(zRange[1], r(crossing.Z))}
	for _, q := range caps {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	ids := map[string]int{}
	node := func(q *big.Rat, sx, sy, sz int) (int, error) {
		rad := [3]*big.Rat{new(big.Rat).Sub(rz2, q), new(big.Rat).Set(q), new(big.Rat).Sub(rx2, q)}
		signs := [3]int{sx, sy, sz}
		key := ""
		for i := 0; i < 3; i++ {
			if rad[i].Sign() == 0 {
				signs[i] = 0
			}
			key += rad[i].RatString() + ":" + strconv.Itoa(signs[i]) + "/"
		}
		if id, ok := ids[key]; ok {
			return id, nil
		}
		n := FinitePerpendicularCylinderNode{}
		n.Signs = signs
		n.Radicands = rad
		n.SurfaceTangent = rad[0].Sign() == 0 && rad[2].Sign() == 0
		for i := 0; i < 3; i++ {
			n.Offset[i] = r(coordinate(crossing, i))
			lo, hi, e := rationalSqrtBounds(rad[i], 128, b)
			if e != nil {
				return 0, e
			}
			if signs[i] < 0 {
				lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
			}
			n.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(n.Offset[i], lo), new(big.Rat).Add(n.Offset[i], hi)}
			for _, v := range n.Bounds[i] {
				if e := b.check(v); e != nil {
					return 0, e
				}
			}
		}
		for i, cap := range caps {
			axis := 0
			if i >= 2 {
				axis = 2
			}
			if cap.Sign() == signs[axis] && new(big.Rat).Mul(cap, cap).Cmp(rad[axis]) == 0 {
				mask := PerpendicularCylinderCapMask(1 << i)
				n.Caps |= mask
				if q.Sign() == 0 {
					n.CapTangencies |= mask
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
			lo, hi := new(big.Rat), new(big.Rat).Set(m2)
			valid := true
			for i, sign := range []int{sx, sz} {
				low, high := new(big.Rat).Set(caps[2*i]), new(big.Rat).Set(caps[2*i+1])
				if sign < 0 {
					low, high = new(big.Rat).Neg(high), new(big.Rat).Neg(low)
				}
				if high.Sign() < 0 {
					valid = false
					break
				}
				if low.Sign() < 0 {
					low.SetInt64(0)
				}
				R2 := rz2
				if i == 1 {
					R2 = rx2
				}
				a, c := new(big.Rat).Sub(R2, new(big.Rat).Mul(high, high)), new(big.Rat).Sub(R2, new(big.Rat).Mul(low, low))
				if a.Cmp(lo) > 0 {
					lo = a
				}
				if c.Cmp(hi) < 0 {
					hi = c
				}
			}
			if !valid || lo.Cmp(hi) > 0 {
				continue
			}
			for _, q := range []*big.Rat{lo, hi} {
				if e := b.check(q); e != nil {
					return out, e
				}
			}
			for _, sy := range []int{-1, 1} {
				a, e := node(lo, sx, sy, sz)
				if e != nil {
					return out, e
				}
				c, e := node(hi, sx, sy, sz)
				if e != nil {
					return out, e
				}
				if lo.Cmp(hi) == 0 {
					continue
				}
				id := len(out.Arcs)
				out.Arcs = append(out.Arcs, FinitePerpendicularCylinderArc{sx, sy, sz, [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, [2]int{a, c}})
				out.Nodes[a].IncidentArcs = append(out.Nodes[a].IncidentArcs, id)
				out.Nodes[c].IncidentArcs = append(out.Nodes[c].IncidentArcs, id)
			}
		}
	}
	seen := make([]bool, len(out.Nodes))
	for i := range out.Nodes {
		if seen[i] {
			continue
		}
		comp := FinitePerpendicularCylinderComponent{}
		queue := []int{i}
		seen[i] = true
		edges := map[int]bool{}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			comp.Nodes = append(comp.Nodes, n)
			degree := len(out.Nodes[n].IncidentArcs)
			if degree == 1 {
				comp.HasBoundary = true
			}
			if degree > 2 {
				comp.Singular = true
			}
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
		comp.IsolatedPoint = len(comp.Arcs) == 0
		comp.CycleRank = len(comp.Arcs) - len(comp.Nodes) + 1
		out.Components = append(out.Components, comp)
	}
	out.Complete = true
	return out, nil
}
