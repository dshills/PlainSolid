package intersect

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"sort"
)

// AffinePolygonTrim uses original rational chart coordinates. Outer must be
// positive, holes negative; all rings must be simple, disjoint and holes strictly
// contained and nonnested. Ring zero denotes Outer, then holes in authored order.
type AffinePolygonTrim struct {
	Outer [][2]*big.Rat
	Holes [][][2]*big.Rat
}
type AffineTrimSide struct{ Owner, Ring, Edge int }

// AffineLineTrimEvent retains the complete original-patch line parameter in
// [0,1], not a renormalized retained interval. Isolated distinguishes a closed
// contact with no retained interval on either side.
// AffineTrimLineLocation classifies an adjacent open parameter cell. NoSide
// means beyond the finite original contact, not an extrapolated outside claim.
type AffineTrimLineLocation uint8

const (
	AffineTrimLineNoSide AffineTrimLineLocation = iota
	AffineTrimLineOutside
	AffineTrimLineInterior
	AffineTrimLineBoundary
)

type AffineLineTrimEvent struct {
	Parameter     *big.Rat
	Vertex        geom.AffineChartIntersectionEndpoint
	Sides         []AffineTrimSide
	Isolated      bool
	Before, After AffineTrimLineLocation
}

// AffineLineTrimSpan indexes events. BoundaryOwners marks a span coincident
// with original trim edges (bit0 A, bit1 B); zero means interior to both trims.
type AffineLineTrimSpan struct {
	Start, End     int
	BoundaryOwners uint8
	Sides          []AffineTrimSide
}
type AffinePolygonLineContact struct {
	source *AffinePatchContact
	events []AffineLineTrimEvent
	spans  []AffineLineTrimSpan
	work   int
}

func (s *AffinePolygonLineContact) Complete() bool { return s != nil && s.source != nil }
func (s *AffinePolygonLineContact) OriginalContact() *AffinePatchContact {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *AffinePolygonLineContact) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *AffinePolygonLineContact) Events() []AffineLineTrimEvent {
	if s == nil {
		return nil
	}
	out := make([]AffineLineTrimEvent, len(s.events))
	for i, v := range s.events {
		out[i] = v
		out[i].Parameter = new(big.Rat).Set(v.Parameter)
		out[i].Sides = append([]AffineTrimSide(nil), v.Sides...)
		tmp := &AffinePatchContact{exact: s.source.exact, vertices: []geom.AffineChartIntersectionEndpoint{v.Vertex}}
		copy, _ := tmp.Vertices()
		out[i].Vertex = copy[0]
	}
	return out
}
func (s *AffinePolygonLineContact) Spans() []AffineLineTrimSpan {
	if s == nil {
		return nil
	}
	out := append([]AffineLineTrimSpan(nil), s.spans...)
	for i := range out {
		out[i].Sides = append([]AffineTrimSide(nil), out[i].Sides...)
	}
	return out
}

// IntersectPolygonAffinePatches proves a transverse closed contact on authored
// straight polygon trims, including holes and nonconvex outer rings. Coplanar
// contacts reject. Exact subdivision includes every trim intersection and every
// coincident edge endpoint before any interval is classified. MaxEdges bounds
// the total edges per owner; MaxOperations also charges topology validator work.
func IntersectPolygonAffinePatches(ctx context.Context, a, c Patch, trims [2]AffinePolygonTrim, p geom.SurfacePatchOptions, o AffinePolygonContactOptions) (*AffinePolygonLineContact, error) {
	if ctx == nil {
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
	owners := [2]Patch{a, c}
	rings := [2][][][2]*big.Rat{}
	for owner, trim := range trims {
		if !owners[owner].valid() {
			return nil, geom.ErrInvalid
		}
		if len(trim.Holes) > o.MaxEdges/3 {
			return nil, geom.ErrInvalid
		}
		rings[owner] = append([][][2]*big.Rat{trim.Outer}, trim.Holes...)
		total := 0
		proof := []trimvalidate.Loop{}
		for _, ring := range rings[owner] {
			total += len(ring)
			if len(ring) < 3 || total > o.MaxEdges {
				return nil, geom.ErrInvalid
			}
			loop := trimvalidate.Loop{}
			for _, v := range ring {
				for axis, q := range v {
					b.check(q)
					if b.err != nil {
						return nil, b.err
					}
					lo, hi := owners[owner].Min.X, owners[owner].Max.X
					if axis == 1 {
						lo, hi = owners[owner].Min.Y, owners[owner].Max.Y
					}
					if q.Cmp(new(big.Rat).SetFloat64(lo)) < 0 || q.Cmp(new(big.Rat).SetFloat64(hi)) > 0 {
						return nil, geom.ErrDomain
					}
				}
			}
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				h := [3][3]*big.Rat{}
				for axis := 0; axis < 2; axis++ {
					h[0][axis] = v[axis]
					h[1][axis] = b.op('/', b.op('+', v[axis], w[axis]), big.NewRat(2, 1))
					h[2][axis] = w[axis]
				}
				for j := 0; j < 3; j++ {
					h[j][2] = big.NewRat(1, 1)
				}
				if b.err != nil {
					return nil, b.err
				}
				edge, e := trimvalidate.ExactQuadratic(h)
				if e != nil {
					return nil, e
				}
				loop = append(loop, edge)
			}
			proof = append(proof, loop)
		}
		if b.work >= b.max {
			return nil, geom.ErrConvergence
		}
		_, report, e := trimvalidate.CertifyLoops(ctx, proof[0], proof[1:], trimvalidate.Options{MaxEdges: o.MaxEdges, MaxWork: b.max - b.work, MaxBits: b.bits})
		b.work += report.Work
		if e != nil {
			return nil, e
		}
		if report.Status == trimvalidate.Unresolved {
			return nil, ErrUnresolved
		}
		if report.Status != trimvalidate.Certified {
			return nil, geom.ErrDomain
		}
	}
	if b.work >= b.max {
		return nil, geom.ErrConvergence
	}
	source, e := IntersectAffinePatches(ctx, a, c, p, geom.AffineChartIntersectionOptions{MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
	if e != nil {
		return nil, e
	}
	b.work += source.work
	if source.coplanar {
		return nil, geom.ErrDomain
	}
	out := &AffinePolygonLineContact{source: source}
	if source.kind == AffinePatchContactEmpty {
		out.work = b.work
		return out, nil
	}
	vertices, _ := source.Vertices()
	first, last := vertices[0], vertices[len(vertices)-1]
	at := func(t *big.Rat) geom.AffineChartIntersectionEndpoint {
		q := geom.AffineChartIntersectionEndpoint{}
		lerp := func(x, y *big.Rat) *big.Rat { return b.op('+', x, b.op('*', t, b.op('-', y, x))) }
		for j := 0; j < 2; j++ {
			q.A[j] = lerp(first.A[j], last.A[j])
			q.B[j] = lerp(first.B[j], last.B[j])
		}
		for j := 0; j < 3; j++ {
			q.Point[j] = lerp(first.Point[j], last.Point[j])
		}
		if t.Sign() == 0 {
			q.BoundaryA = first.BoundaryA
			q.BoundaryB = first.BoundaryB
		} else if t.Cmp(big.NewRat(1, 1)) == 0 {
			q.BoundaryA = last.BoundaryA
			q.BoundaryB = last.BoundaryB
		} else {
			q.BoundaryA = first.BoundaryA & last.BoundaryA
			q.BoundaryB = first.BoundaryB & last.BoundaryB
		}
		return q
	}
	onSegment := func(v, w, q [2]*big.Rat) bool {
		for axis := 0; axis < 2; axis++ {
			x, y := v[axis], w[axis]
			if x.Cmp(y) > 0 {
				x, y = y, x
			}
			if q[axis].Cmp(x) < 0 || q[axis].Cmp(y) > 0 {
				return false
			}
		}
		return true
	}
	sites := []*big.Rat{big.NewRat(0, 1)}
	if source.kind == AffinePatchContactSegment {
		sites = append(sites, big.NewRat(1, 1))
	}
	for owner, loops := range rings {
		start, end := affineContactUV(first, owner), affineContactUV(last, owner)
		axis := 0
		if start[0].Cmp(end[0]) == 0 {
			axis = 1
		}
		for _, ring := range loops {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				s0, s1 := b.side(v, w, start), b.side(v, w, end)
				if b.err != nil {
					return nil, b.err
				}
				if s0.Cmp(s1) != 0 {
					t := b.op('/', s0, b.op('-', s0, s1))
					if t.Sign() >= 0 && t.Cmp(big.NewRat(1, 1)) <= 0 {
						point := affineContactUV(at(t), owner)
						if onSegment(v, w, point) {
							sites = append(sites, t)
						}
					}
				} else if s0.Sign() == 0 && source.kind == AffinePatchContactSegment {
					for _, endpoint := range [][2]*big.Rat{v, w} {
						t := b.op('/', b.op('-', endpoint[axis], start[axis]), b.op('-', end[axis], start[axis]))
						if t.Sign() >= 0 && t.Cmp(big.NewRat(1, 1)) <= 0 {
							sites = append(sites, t)
						}
					}
				}
			}
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Cmp(sites[j]) < 0 })
	unique := sites[:0]
	for _, t := range sites {
		if len(unique) == 0 || t.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, t)
		}
	}
	sites = unique
	// All straight-boundary events are now present. Winding is constant in each
	// remaining open interval; boundary intervals have their own exact ownership.
	locate := func(q geom.AffineChartIntersectionEndpoint) (bool, uint8, []AffineTrimSide) {
		retained := true
		mask := uint8(0)
		sides := []AffineTrimSide{}
		for owner, loops := range rings {
			uv := affineContactUV(q, owner)
			wind := 0
			boundary := false
			for ri, ring := range loops {
				for i, v := range ring {
					w := ring[(i+1)%len(ring)]
					side := b.side(v, w, uv).Sign()
					if side == 0 && onSegment(v, w, uv) {
						boundary = true
						sides = append(sides, AffineTrimSide{owner, ri, i})
					}
					if v[1].Cmp(uv[1]) <= 0 && w[1].Cmp(uv[1]) > 0 && side > 0 {
						wind++
					}
					if v[1].Cmp(uv[1]) > 0 && w[1].Cmp(uv[1]) <= 0 && side < 0 {
						wind--
					}
				}
			}
			if boundary {
				mask |= 1 << owner
			} else if wind == 0 {
				retained = false
			}
		}
		return retained, mask, sides
	}
	type cell struct {
		keep  bool
		mask  uint8
		sides []AffineTrimSide
	}
	cells := make([]cell, len(sites)-1)
	for i := range cells {
		t := b.op('/', b.op('+', sites[i], sites[i+1]), big.NewRat(2, 1))
		keep, mask, sides := locate(at(t))
		cells[i] = cell{keep, mask, sides}
	}
	if b.err != nil {
		return nil, b.err
	}
	indices := make([]int, len(sites))
	for i := range indices {
		indices[i] = -1
	}
	for i, t := range sites {
		v := at(t)
		keep, _, sides := locate(v)
		if keep {
			indices[i] = len(out.events)
			isolated := (i == 0 || !cells[i-1].keep) && (i == len(cells) || !cells[i].keep)
			location := func(index int) AffineTrimLineLocation {
				if index < 0 || index >= len(cells) {
					return AffineTrimLineNoSide
				}
				if !cells[index].keep {
					return AffineTrimLineOutside
				}
				if cells[index].mask != 0 {
					return AffineTrimLineBoundary
				}
				return AffineTrimLineInterior
			}
			out.events = append(out.events, AffineLineTrimEvent{Parameter: new(big.Rat).Set(t), Vertex: v, Sides: sides, Isolated: isolated, Before: location(i - 1), After: location(i)})
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	for i, cell := range cells {
		if cell.keep {
			if indices[i] < 0 || indices[i+1] < 0 {
				return nil, geom.ErrInvalid
			}
			out.spans = append(out.spans, AffineLineTrimSpan{indices[i], indices[i+1], cell.mask, cell.sides})
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	out.work = b.work
	return out, nil
}
