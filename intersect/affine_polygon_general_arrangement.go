package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// AffineSharedBoundarySpan is a common boundary interval, independently of the
// requested Boolean. IntersectionSides marks intersection material immediately
// left/right of its canonical direction in A UV. Two false entries prove a
// purely lower-dimensional common span. Sources retain every original use;
// their Start/End are -1 because these vertices are independent of area indices.
type AffineSharedBoundarySpan struct {
	Vertices          [2]AffineArrangementVertex
	IntersectionSides [2]bool
	Sources           []AffineArrangementEdge
}

// AffineGeneralPolygonArrangement is an opt-in manifold cycle arrangement with
// coincident-source provenance and explicit lower-dimensional Common witnesses.
// Area().Edges() gives a representative use; EdgeSources gives ALL original uses.
// Point-touch branch nodes which cannot form manifold cycles remain unresolved.
type AffineGeneralPolygonArrangement struct {
	area     *AffinePolygonArrangement
	sources  [][]AffineArrangementEdge
	shared   []AffineSharedBoundarySpan
	isolated []AffineArrangementVertex
}

func (s *AffineGeneralPolygonArrangement) Complete() bool {
	return s != nil && s.area != nil && s.area.Complete()
}
func (s *AffineGeneralPolygonArrangement) ExactOperations() int {
	if s == nil || s.area == nil {
		return 0
	}
	return s.area.ExactOperations()
}
func (s *AffineGeneralPolygonArrangement) Area() *AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.area
}
func (s *AffineGeneralPolygonArrangement) EdgeSources() [][]AffineArrangementEdge {
	if s == nil {
		return nil
	}
	out := make([][]AffineArrangementEdge, len(s.sources))
	for i, v := range s.sources {
		out[i] = (&AffinePolygonArrangement{edges: v}).Edges()
	}
	return out
}
func (s *AffineGeneralPolygonArrangement) SharedBoundarySpans() []AffineSharedBoundarySpan {
	if s == nil {
		return nil
	}
	out := make([]AffineSharedBoundarySpan, len(s.shared))
	for i, v := range s.shared {
		out[i] = v
		copy := (&AffinePolygonArrangement{vertices: v.Vertices[:]}).Vertices()
		out[i].Vertices = [2]AffineArrangementVertex{copy[0], copy[1]}
		out[i].Sources = (&AffinePolygonArrangement{edges: v.Sources}).Edges()
	}
	return out
}

// IsolatedCommonPoints is populated for Intersection only; these exact common
// boundary events belong to neither an area cycle nor a common boundary span.
func (s *AffineGeneralPolygonArrangement) IsolatedCommonPoints() []AffineArrangementVertex {
	if s == nil {
		return nil
	}
	return (&AffinePolygonArrangement{vertices: s.isolated}).Vertices()
}

// ArrangeGeneralAffinePolygons accepts exact endpoint and collinear overlap
// events in addition to transverse crossings. Symbolic rational infinitesimal
// winding proves both sides of every fully split segment; no finite offset or
// tolerance is used. Area cycles describe the regularized planar Boolean;
// closed-set Common spans/points are separate and never promoted to area. It
// requires manifold directed cycles for the selected area.
// Existing ArrangeTransverseAffinePolygons remains strict and unchanged.
func ArrangeGeneralAffinePolygons(ctx context.Context, a, c Patch, trims [2]AffinePolygonTrim, operation AffineRingOperation, p geom.SurfacePatchOptions, o AffinePolygonContactOptions) (*AffineGeneralPolygonArrangement, error) {
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
	vertex := func(side affineArrangementSide, t *big.Rat) AffineArrangementVertex {
		v := AffineArrangementVertex{}
		for axis := 0; axis < 2; axis++ {
			v.A[axis] = lerp(side.xy[0][axis], side.xy[1][axis], t)
		}
		for axis := 0; axis < 3; axis++ {
			v.Point[axis] = lerp(side.xyz[0][axis], side.xyz[1][axis], t)
		}
		return v
	}
	key := func(v AffineArrangementVertex) string { return v.A[0].RatString() + "," + v.A[1].RatString() }
	events := []AffineArrangementVertex{}
	eventSeen := map[string]bool{}
	record := func(x, y *affineArrangementSide, tx, ty *big.Rat) error {
		vx, vy := vertex(*x, tx), vertex(*y, ty)
		if b.err != nil {
			return b.err
		}
		for axis := 0; axis < 3; axis++ {
			if vx.Point[axis].Cmp(vy.Point[axis]) != 0 {
				return geom.ErrInvalid
			}
		}
		x.cuts = append(x.cuts, tx)
		y.cuts = append(y.cuts, ty)
		k := key(vx)
		if !eventSeen[k] {
			eventSeen[k] = true
			events = append(events, vx)
		}
		return nil
	}
	between := func(x, y, q [2]*big.Rat) bool {
		for axis := 0; axis < 2; axis++ {
			lo, hi := x[axis], y[axis]
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if q[axis].Cmp(lo) < 0 || q[axis].Cmp(hi) > 0 {
				return false
			}
		}
		return true
	}
	parameter := func(side affineArrangementSide, q [2]*big.Rat) *big.Rat {
		axis := 0
		if side.xy[0][0].Cmp(side.xy[1][0]) == 0 {
			axis = 1
		}
		return b.op('/', b.op('-', q[axis], side.xy[0][axis]), b.op('-', side.xy[1][axis], side.xy[0][axis]))
	}
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
			if s0.Sign()*s1.Sign() == -1 && t0.Sign()*t1.Sign() == -1 {
				if e := record(x, y, b.op('/', t0, b.op('-', t0, t1)), b.op('/', s0, b.op('-', s0, s1))); e != nil {
					return nil, e
				}
				continue
			}
			for k, s := range []*big.Rat{s0, s1} {
				if s.Sign() == 0 && between(x.xy[0], x.xy[1], y.xy[k]) {
					if e := record(x, y, parameter(*x, y.xy[k]), big.NewRat(int64(k), 1)); e != nil {
						return nil, e
					}
				}
			}
			for k, t := range []*big.Rat{t0, t1} {
				if t.Sign() == 0 && between(y.xy[0], y.xy[1], x.xy[k]) {
					if e := record(x, y, big.NewRat(int64(k), 1), parameter(*y, x.xy[k])); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	// Every affine sign is evaluated at q+epsilon*n with epsilon positive and
	// infinitesimal: first nonzero coefficient decides, including ray-vertex ties.
	sign := func(c, d *big.Rat) int {
		if c.Sign() != 0 {
			return c.Sign()
		}
		return d.Sign()
	}
	inside := func(q, n [2]*big.Rat, loops [][][2]*big.Rat) bool {
		wind := 0
		for _, ring := range loops {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				s0 := b.side(v, w, q)
				dx, dy := b.op('-', w[0], v[0]), b.op('-', w[1], v[1])
				s1 := b.op('-', b.op('*', dx, n[1]), b.op('*', dy, n[0]))
				side := sign(s0, s1)
				ny := b.op('-', new(big.Rat), n[1])
				vy, wy := sign(b.op('-', v[1], q[1]), ny), sign(b.op('-', w[1], q[1]), ny)
				if vy <= 0 && wy > 0 && side > 0 {
					wind++
				}
				if vy > 0 && wy <= 0 && side < 0 {
					wind--
				}
			}
		}
		return wind != 0
	}
	boundary := func(q [2]*big.Rat, loops [][][2]*big.Rat) bool {
		for _, ring := range loops {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				if b.side(v, w, q).Sign() == 0 && between(v, w, q) {
					return true
				}
			}
		}
		return false
	}
	material := func(a, c bool) bool {
		switch operation {
		case AffineRingIntersection:
			return a && c
		case AffineRingUnion:
			return a || c
		default:
			return a && !c
		}
	}
	out := &AffineGeneralPolygonArrangement{area: &AffinePolygonArrangement{source: source}}
	area := out.area
	nodes := map[string]int{}
	edgeMap := map[string]int{}
	sharedMap := map[string]int{}
	node := func(v AffineArrangementVertex) (int, error) {
		k := key(v)
		if i, ok := nodes[k]; ok {
			for axis := 0; axis < 3; axis++ {
				if v.Point[axis].Cmp(area.vertices[i].Point[axis]) != 0 {
					return 0, geom.ErrInvalid
				}
			}
			return i, nil
		}
		i := len(area.vertices)
		nodes[k] = i
		area.vertices = append(area.vertices, v)
		return i, nil
	}
	provenance := func(side affineArrangementSide, lo, hi *big.Rat) AffineArrangementEdge {
		e := AffineArrangementEdge{Owner: side.owner, Ring: side.ring, Edge: side.edge, Start: -1, End: -1, Parameter: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
		for axis := 0; axis < 2; axis++ {
			e.UV[0][axis] = lerp(side.uv[0][axis], side.uv[1][axis], lo)
			e.UV[1][axis] = lerp(side.uv[0][axis], side.uv[1][axis], hi)
		}
		return e
	}
	for _, side := range sides {
		sort.Slice(side.cuts, func(i, j int) bool { return side.cuts[i].Cmp(side.cuts[j]) < 0 })
		cuts := side.cuts[:0]
		for _, t := range side.cuts {
			if len(cuts) == 0 || t.Cmp(cuts[len(cuts)-1]) != 0 {
				cuts = append(cuts, t)
			}
		}
		for i := 0; i+1 < len(cuts); i++ {
			lo, hi := cuts[i], cuts[i+1]
			mid := b.op('/', b.op('+', lo, hi), big.NewRat(2, 1))
			q := [2]*big.Rat{lerp(side.xy[0][0], side.xy[1][0], mid), lerp(side.xy[0][1], side.xy[1][1], mid)}
			n := [2]*big.Rat{b.op('-', side.xy[0][1], side.xy[1][1]), b.op('-', side.xy[1][0], side.xy[0][0])}
			neg := [2]*big.Rat{b.op('-', new(big.Rat), n[0]), b.op('-', new(big.Rat), n[1])}
			al, cl := inside(q, n, projected[0]), inside(q, n, projected[1])
			ar, cr := inside(q, neg, projected[0]), inside(q, neg, projected[1])
			left, right := material(al, cl), material(ar, cr)
			v, w := vertex(side, lo), vertex(side, hi)
			if b.err != nil {
				return nil, b.err
			}
			if boundary(q, projected[1-side.owner]) {
				vl, wh := v, w
				tl, th := lo, hi
				flags := [2]bool{al && cl, ar && cr}
				if key(vl) > key(wh) {
					vl, wh = wh, vl
					tl, th = th, tl
					flags[0], flags[1] = flags[1], flags[0]
				}
				k := key(vl) + ";" + key(wh)
				use := provenance(side, tl, th)
				if si, ok := sharedMap[k]; ok {
					if out.shared[si].IntersectionSides != flags {
						return nil, geom.ErrInvalid
					}
					out.shared[si].Sources = append(out.shared[si].Sources, use)
				} else {
					sharedMap[k] = len(out.shared)
					out.shared = append(out.shared, AffineSharedBoundarySpan{[2]AffineArrangementVertex{vl, wh}, flags, []AffineArrangementEdge{use}})
				}
			}
			if left == right {
				continue
			}
			if !left {
				lo, hi = hi, lo
				v, w = w, v
			}
			start, e := node(v)
			if e != nil {
				return nil, e
			}
			end, e := node(w)
			if e != nil {
				return nil, e
			}
			if start == end {
				return nil, geom.ErrInvalid
			}
			use := provenance(side, lo, hi)
			use.Start, use.End = start, end
			k := key(v) + ";" + key(w)
			reverse := key(w) + ";" + key(v)
			if _, bad := edgeMap[reverse]; bad {
				return nil, geom.ErrInvalid
			}
			if ei, ok := edgeMap[k]; ok {
				out.sources[ei] = append(out.sources[ei], use)
			} else {
				edgeMap[k] = len(area.edges)
				area.edges = append(area.edges, use)
				out.sources = append(out.sources, []AffineArrangementEdge{use})
			}
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	incoming, outgoing := make([]int, len(area.vertices)), make([]int, len(area.vertices))
	for i := range incoming {
		incoming[i] = -1
		outgoing[i] = -1
	}
	for i, e := range area.edges {
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
	visited := make([]bool, len(area.edges))
	for seed := range area.edges {
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
			edge := area.edges[current]
			v, w := area.vertices[edge.Start].A, area.vertices[edge.End].A
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
		area.cycles = append(area.cycles, cycle)
	}
	if operation == AffineRingIntersection {
		covered := map[string]bool{}
		for _, v := range area.vertices {
			covered[key(v)] = true
		}
		for _, s := range out.shared {
			covered[key(s.Vertices[0])] = true
			covered[key(s.Vertices[1])] = true
		}
		for _, v := range events {
			if !covered[key(v)] {
				out.isolated = append(out.isolated, v)
			}
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	area.work = b.work
	return out, nil
}
