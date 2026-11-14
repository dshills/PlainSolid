package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// AffineArrangementVertex binds an exact model vertex to owner A's affine chart.
// A may lie outside A's retained rectangle for a union boundary owned only by B.
type AffineArrangementVertex struct {
	A     [2]*big.Rat
	Point [3]*big.Rat
}

// AffineArrangementEdge is a directed retained part of one authored trim side.
// Parameter is in the original side's [0,1] units and can decrease on reversal.
// UV retains that owner's original chart; indices refer to Vertices().
type AffineArrangementEdge struct {
	Owner, Ring, Edge, Start, End int
	Parameter                     [2]*big.Rat
	UV                            [2][2]*big.Rat
}

// AffineArrangementCycle lists directed edge indices. SignedAreaA is positive
// for an exterior and negative for a hole in the common owner A orientation.
type AffineArrangementCycle struct {
	Edges       []int
	SignedAreaA *big.Rat
}
type AffinePolygonArrangement struct {
	source   *AffinePatchContact
	vertices []AffineArrangementVertex
	edges    []AffineArrangementEdge
	cycles   []AffineArrangementCycle
	work     int
}

func (s *AffinePolygonArrangement) Complete() bool { return s != nil && s.source != nil }
func (s *AffinePolygonArrangement) OriginalContact() *AffinePatchContact {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *AffinePolygonArrangement) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *AffinePolygonArrangement) Vertices() []AffineArrangementVertex {
	if s == nil {
		return nil
	}
	out := make([]AffineArrangementVertex, len(s.vertices))
	for i, v := range s.vertices {
		for j := 0; j < 2; j++ {
			out[i].A[j] = new(big.Rat).Set(v.A[j])
		}
		for j := 0; j < 3; j++ {
			out[i].Point[j] = new(big.Rat).Set(v.Point[j])
		}
	}
	return out
}
func (s *AffinePolygonArrangement) Edges() []AffineArrangementEdge {
	if s == nil {
		return nil
	}
	out := append([]AffineArrangementEdge(nil), s.edges...)
	for i, e := range out {
		for j := 0; j < 2; j++ {
			out[i].Parameter[j] = new(big.Rat).Set(e.Parameter[j])
			for k := 0; k < 2; k++ {
				out[i].UV[j][k] = new(big.Rat).Set(e.UV[j][k])
			}
		}
	}
	return out
}
func (s *AffinePolygonArrangement) Cycles() []AffineArrangementCycle {
	if s == nil {
		return nil
	}
	out := make([]AffineArrangementCycle, len(s.cycles))
	for i, c := range s.cycles {
		out[i] = AffineArrangementCycle{append([]int(nil), c.Edges...), new(big.Rat).Set(c.SignedAreaA)}
	}
	return out
}

type affineArrangementSide struct {
	owner, ring, edge int
	uv, xy            [2][2]*big.Rat
	xyz               [2][3]*big.Rat
	cuts              []*big.Rat
}

// ArrangeTransverseAffinePolygons constructs complete Boolean boundary cycles
// for coplanar polygon/hole trims with only proper transverse cross-owner
// boundary intersections (or no intersections). All vertex touches, endpoint-on-
// edge contacts and coincident spans reject atomically. This is a planar read-only
// arrangement, not native B-rep sewing or a positive-volume Boolean.
// MaxEdges bounds each owner's authored edges and hence the finite arrangement
// size. Exact arithmetic and trim validation share the MaxOperations budget.
func ArrangeTransverseAffinePolygons(ctx context.Context, a, c Patch, trims [2]AffinePolygonTrim, operation AffineRingOperation, p geom.SurfacePatchOptions, o AffinePolygonContactOptions) (*AffinePolygonArrangement, error) {
	if ctx == nil || operation > AffineRingDifference {
		return nil, geom.ErrInvalid
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 64
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxEdges < 3 || o.MaxEdges > 256 || o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	b := &affineTrimBudget{ctx: ctx, max: o.MaxOperations, bits: o.MaxCoefficientBits}
	rings, e := validateAffinePolygonTrims(ctx, [2]Patch{a, c}, trims, o, b)
	if e != nil {
		return nil, e
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	source, e := IntersectAffinePatches(ctx, a, c, p, geom.AffineChartIntersectionOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
	if e != nil {
		return nil, e
	}
	b.work += source.work
	if !source.coplanar {
		return nil, geom.ErrDomain
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	chart, e := geom.NewAffineSurfaceChart(ctx, source.exact[0], geom.AffineSurfaceChartOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits, MaxCells: 4096})
	if e != nil {
		return nil, e
	}
	b.work += chart.ExactOperations()
	projected := [2][][][2]*big.Rat{}
	sides := []affineArrangementSide{}
	for owner, loops := range rings {
		for ri, ring := range loops {
			xy := make([][2]*big.Rat, len(ring))
			xyz := make([][3]*big.Rat, len(ring))
			for i, uv := range ring {
				if b.work >= b.max {
					return nil, geom.ErrConvergence
				}
				jet, e := source.exact[owner].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
				b.work += jet.ExactOperations
				if e != nil {
					return nil, e
				}
				xyz[i] = jet.Point
				if owner == 0 {
					xy[i] = uv
				} else {
					if b.work >= b.max {
						return nil, geom.ErrConvergence
					}
					mapped, e := chart.InvertExactContext(ctx, jet.Point, geom.AffineChartInverseOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
					b.work += mapped.ExactOperations
					if e != nil {
						return nil, e
					}
					if !mapped.OnPlane {
						return nil, geom.ErrDomain
					}
					xy[i] = [2]*big.Rat{mapped.U, mapped.V}
				}
			}
			projected[owner] = append(projected[owner], xy)
			for i := range ring {
				j := (i + 1) % len(ring)
				sides = append(sides, affineArrangementSide{owner: owner, ring: ri, edge: i, uv: [2][2]*big.Rat{ring[i], ring[j]}, xy: [2][2]*big.Rat{xy[i], xy[j]}, xyz: [2][3]*big.Rat{xyz[i], xyz[j]}, cuts: []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}})
			}
		}
	}
	lerp := func(x, y, t *big.Rat) *big.Rat { return b.op('+', x, b.op('*', t, b.op('-', y, x))) }
	for i := range sides {
		for j := i + 1; j < len(sides); j++ {
			x, y := &sides[i], &sides[j]
			if x.owner == y.owner {
				continue
			}
			s0, s1 := b.side(x.xy[0], x.xy[1], y.xy[0]), b.side(x.xy[0], x.xy[1], y.xy[1])
			t0, t1 := b.side(y.xy[0], y.xy[1], x.xy[0]), b.side(y.xy[0], y.xy[1], x.xy[1])
			if b.err != nil {
				return nil, b.err
			}
			if s0.Sign()*s1.Sign() > 0 || t0.Sign()*t1.Sign() > 0 {
				continue
			}
			apart := false
			for axis := 0; axis < 2; axis++ {
				lo, hi := x.xy[0][axis], x.xy[1][axis]
				if lo.Cmp(hi) > 0 {
					lo, hi = hi, lo
				}
				clo, chi := y.xy[0][axis], y.xy[1][axis]
				if clo.Cmp(chi) > 0 {
					clo, chi = chi, clo
				}
				if hi.Cmp(clo) < 0 || chi.Cmp(lo) < 0 {
					apart = true
				}
			}
			if apart {
				continue
			}
			if s0.Sign()*s1.Sign() != -1 || t0.Sign()*t1.Sign() != -1 {
				return nil, ErrUnresolved
			}
			tx := b.op('/', t0, b.op('-', t0, t1))
			ty := b.op('/', s0, b.op('-', s0, s1))
			for axis := 0; axis < 3; axis++ {
				px := lerp(x.xyz[0][axis], x.xyz[1][axis], tx)
				py := lerp(y.xyz[0][axis], y.xyz[1][axis], ty)
				if b.err != nil {
					return nil, b.err
				}
				if px.Cmp(py) != 0 {
					return nil, geom.ErrInvalid
				}
			}
			x.cuts = append(x.cuts, tx)
			y.cuts = append(y.cuts, ty)
		}
	}
	inside := func(q [2]*big.Rat, loops [][][2]*big.Rat) (bool, error) {
		wind := 0
		for _, ring := range loops {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				side := b.side(v, w, q).Sign()
				if b.err != nil {
					return false, b.err
				}
				if side == 0 {
					on := true
					for axis := 0; axis < 2; axis++ {
						lo, hi := v[axis], w[axis]
						if lo.Cmp(hi) > 0 {
							lo, hi = hi, lo
						}
						if q[axis].Cmp(lo) < 0 || q[axis].Cmp(hi) > 0 {
							on = false
						}
					}
					if on {
						return false, ErrUnresolved
					}
				}
				if v[1].Cmp(q[1]) <= 0 && w[1].Cmp(q[1]) > 0 && side > 0 {
					wind++
				}
				if v[1].Cmp(q[1]) > 0 && w[1].Cmp(q[1]) <= 0 && side < 0 {
					wind--
				}
			}
		}
		return wind != 0, nil
	}
	out := &AffinePolygonArrangement{source: source}
	nodes := map[string]int{}
	node := func(side affineArrangementSide, t *big.Rat) (int, error) {
		v := AffineArrangementVertex{}
		for axis := 0; axis < 2; axis++ {
			v.A[axis] = lerp(side.xy[0][axis], side.xy[1][axis], t)
		}
		for axis := 0; axis < 3; axis++ {
			v.Point[axis] = lerp(side.xyz[0][axis], side.xyz[1][axis], t)
		}
		if b.err != nil {
			return 0, b.err
		}
		key := v.A[0].RatString() + "," + v.A[1].RatString()
		if i, ok := nodes[key]; ok {
			for axis := 0; axis < 3; axis++ {
				if v.Point[axis].Cmp(out.vertices[i].Point[axis]) != 0 {
					return 0, geom.ErrInvalid
				}
			}
			return i, nil
		}
		i := len(out.vertices)
		nodes[key] = i
		out.vertices = append(out.vertices, v)
		return i, nil
	}
	for _, side := range sides {
		sort.Slice(side.cuts, func(i, j int) bool { return side.cuts[i].Cmp(side.cuts[j]) < 0 })
		for i := 0; i+1 < len(side.cuts); i++ {
			lo, hi := side.cuts[i], side.cuts[i+1]
			if lo.Cmp(hi) == 0 {
				return nil, ErrUnresolved
			}
			mid := b.op('/', b.op('+', lo, hi), big.NewRat(2, 1))
			q := [2]*big.Rat{lerp(side.xy[0][0], side.xy[1][0], mid), lerp(side.xy[0][1], side.xy[1][1], mid)}
			in, e := inside(q, projected[1-side.owner])
			if e != nil {
				return nil, e
			}
			keep := in
			direction := 1
			if side.owner == 1 {
				direction = source.orientation
			}
			switch operation {
			case AffineRingUnion:
				keep = !in
			case AffineRingDifference:
				if side.owner == 0 {
					keep = !in
				} else {
					direction = -direction
				}
			}
			if !keep {
				continue
			}
			if direction < 0 {
				lo, hi = hi, lo
			}
			start, e := node(side, lo)
			if e != nil {
				return nil, e
			}
			end, e := node(side, hi)
			if e != nil {
				return nil, e
			}
			if start == end {
				return nil, geom.ErrInvalid
			}
			edge := AffineArrangementEdge{Owner: side.owner, Ring: side.ring, Edge: side.edge, Start: start, End: end, Parameter: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
			for axis := 0; axis < 2; axis++ {
				edge.UV[0][axis] = lerp(side.uv[0][axis], side.uv[1][axis], lo)
				edge.UV[1][axis] = lerp(side.uv[0][axis], side.uv[1][axis], hi)
			}
			out.edges = append(out.edges, edge)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	incoming, outgoing := make([]int, len(out.vertices)), make([]int, len(out.vertices))
	for i := range incoming {
		incoming[i] = -1
		outgoing[i] = -1
	}
	for i, e := range out.edges {
		if incoming[e.End] != -1 || outgoing[e.Start] != -1 {
			return nil, ErrUnresolved
		}
		incoming[e.End] = i
		outgoing[e.Start] = i
	}
	for i := range incoming {
		if incoming[i] < 0 || outgoing[i] < 0 {
			return nil, ErrUnresolved
		}
	}
	visited := make([]bool, len(out.edges))
	for seed := range out.edges {
		if visited[seed] {
			continue
		}
		cycle := AffineArrangementCycle{SignedAreaA: new(big.Rat)}
		current := seed
		for {
			if visited[current] {
				if current != seed {
					return nil, ErrUnresolved
				}
				break
			}
			visited[current] = true
			cycle.Edges = append(cycle.Edges, current)
			edge := out.edges[current]
			v, w := out.vertices[edge.Start].A, out.vertices[edge.End].A
			cycle.SignedAreaA = b.op('+', cycle.SignedAreaA, b.op('-', b.op('*', v[0], w[1]), b.op('*', v[1], w[0])))
			current = outgoing[edge.End]
		}
		cycle.SignedAreaA = b.op('/', cycle.SignedAreaA, big.NewRat(2, 1))
		if b.err != nil {
			return nil, b.err
		}
		if len(cycle.Edges) < 3 || cycle.SignedAreaA.Sign() == 0 {
			return nil, ErrUnresolved
		}
		out.cycles = append(out.cycles, cycle)
	}
	out.work = b.work
	return out, nil
}
