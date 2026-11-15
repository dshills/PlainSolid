package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

type AffineImprintFragmentUse struct {
	Start, End                                                  int
	OriginalEdge, OriginalLocalEdge, SectionEdge, AuxiliaryEdge int         // -1 when not applicable
	Parameter                                                   [2]*big.Rat // normalized canonical original-edge units for original edges
}
type AffineImprintFragmentCycle struct {
	Uses []AffineImprintFragmentUse
	Area *big.Rat
}
type AffineImprintFragmentOptions struct {
	MaxOperations, MaxVertices, MaxCycles, MaxCoefficientBits int
	// AuxiliarySubdivision adds exact noncrossing diagonals between existing
	// source-chart nodes. It partitions the whole face, without material selection.
	AuxiliarySubdivision bool
}

// AffineImprintFaceFragments is an exact partition of one whole original chart.
// Cycles are positive in chart UV; Original reports the original physical reversal.
// The default lane rejects pinched walks and disconnected inner boundary loops;
// opt-in exact auxiliary subdivision may partition them into simple cycles.
type AffineImprintFaceFragments struct {
	source      *AffineShellPairedImprint
	owner, face int
	uv          [][2]*big.Rat
	cycles      []AffineImprintFragmentCycle
	area        *big.Rat
	work        int
}

func (r *AffineImprintFaceFragments) Original() (int, int, bool) {
	if r == nil {
		return -1, -1, false
	}
	return r.owner, r.face, r.source.sources[r.owner].faces[r.face].Reversed
}
func (r *AffineImprintFaceFragments) Vertices() [][2]*big.Rat {
	if r == nil {
		return nil
	}
	out := make([][2]*big.Rat, len(r.uv))
	for i, p := range r.uv {
		for j, q := range p {
			out[i][j] = new(big.Rat).Set(q)
		}
	}
	return out
}
func (r *AffineImprintFaceFragments) Cycles() []AffineImprintFragmentCycle {
	if r == nil {
		return nil
	}
	out := make([]AffineImprintFragmentCycle, len(r.cycles))
	for i, c := range r.cycles {
		out[i].Area = new(big.Rat).Set(c.Area)
		out[i].Uses = append([]AffineImprintFragmentUse(nil), c.Uses...)
		for j := range out[i].Uses {
			for k, p := range out[i].Uses[j].Parameter {
				if p != nil {
					out[i].Uses[j].Parameter[k] = new(big.Rat).Set(p)
				}
			}
		}
	}
	return out
}

// ExactArea returns chart UV area, not physical surface area.
func (r *AffineImprintFaceFragments) ExactArea() *big.Rat {
	if r == nil {
		return nil
	}
	return new(big.Rat).Set(r.area)
}
func (r *AffineImprintFaceFragments) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.work
}

// FragmentFace partitions a complete authenticated rectangular source face.
// Without AuxiliarySubdivision it rejects pinched cycles and disconnected inner
// loops. Unsupported atom intersections always reject atomically. Cycles do not
// select either shell's retained material.
func (s *AffineShellPairedImprint) FragmentFace(ctx context.Context, owner, face int, o AffineImprintFragmentOptions) (*AffineImprintFaceFragments, error) {
	if ctx == nil || s == nil || owner < 0 || owner > 1 || face < 0 || face >= len(s.sources[owner].faces) {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 256
	}
	if o.MaxCycles == 0 {
		o.MaxCycles = 128
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxVertices < 4 || o.MaxVertices > 4096 || o.MaxCycles < 1 || o.MaxCycles > 1024 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 {
		return nil, geom.ErrInvalid
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: o.MaxCoefficientBits}
	r := &AffineImprintFaceFragments{source: s, owner: owner, face: face}
	ids := map[[2]string]int{}
	vertex := func(p [2]*big.Rat) (int, error) {
		for _, v := range p {
			b.check(v)
		}
		if b.err != nil {
			return 0, b.err
		}
		key := [2]string{p[0].RatString(), p[1].RatString()}
		if i, ok := ids[key]; ok {
			return i, nil
		}
		if len(ids) >= o.MaxVertices {
			return 0, geom.ErrConvergence
		}
		i := len(ids)
		ids[key] = i
		r.uv = append(r.uv, [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])})
		return i, nil
	}
	type atom struct {
		a, c int
		use  AffineImprintFragmentUse
	}
	atoms := []atom{}
	for i, e := range s.edges {
		if e.Faces[owner] != face {
			continue
		}
		a, err := vertex(e.UV[owner][0])
		if err != nil {
			return nil, err
		}
		c, err := vertex(e.UV[owner][1])
		if err != nil {
			return nil, err
		}
		atoms = append(atoms, atom{a, c, AffineImprintFragmentUse{OriginalEdge: -1, OriginalLocalEdge: -1, SectionEdge: i, AuxiliaryEdge: -1}})
	}
	original := s.sources[owner].faces[face]
	for local, u := range original.Outer {
		p := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
		q := [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
		if _, e := vertex(p); e != nil {
			return nil, e
		}
		if _, e := vertex(q); e != nil {
			return nil, e
		}
		axis := 0
		if p[0].Cmp(q[0]) == 0 {
			axis = 1
		}
		fixed := 1 - axis
		if p[fixed].Cmp(q[fixed]) != 0 || p[axis].Cmp(q[axis]) == 0 {
			return nil, geom.ErrDomain
		}
		type at struct {
			id int
			t  *big.Rat
		}
		points := []at{}
		for i, v := range r.uv {
			if v[fixed].Cmp(p[fixed]) != 0 {
				continue
			}
			t := b.op('/', b.op('-', v[axis], p[axis]), b.op('-', q[axis], p[axis]))
			if t.Sign() >= 0 && t.Cmp(big.NewRat(1, 1)) <= 0 {
				points = append(points, at{i, t})
			}
		}
		sort.Slice(points, func(i, j int) bool { return points[i].t.Cmp(points[j].t) < 0 })
		if len(points) < 2 || points[0].t.Sign() != 0 || points[len(points)-1].t.Cmp(big.NewRat(1, 1)) != 0 {
			return nil, geom.ErrInvalid
		}
		for i := 0; i+1 < len(points); i++ {
			ts := [2]*big.Rat{points[i].t, points[i+1].t}
			if u.Reversed {
				for k := range ts {
					ts[k] = b.op('-', big.NewRat(1, 1), ts[k])
				}
			}
			atoms = append(atoms, atom{points[i].id, points[i+1].id, AffineImprintFragmentUse{OriginalEdge: u.Edge, OriginalLocalEdge: local, SectionEdge: -1, AuxiliaryEdge: -1, Parameter: ts}})
		}
	}
	// Every edge belongs to the same independently embedded source chart. Check
	// straight atom intersections again: only common graph endpoints are permitted.
	cross := func(a, c, d int) *big.Rat {
		p, q, t := r.uv[a], r.uv[c], r.uv[d]
		return b.op('-', b.op('*', b.op('-', q[0], p[0]), b.op('-', t[1], p[1])), b.op('*', b.op('-', q[1], p[1]), b.op('-', t[0], p[0])))
	}
	on := func(a, c, p int) bool {
		if cross(a, c, p).Sign() != 0 {
			return false
		}
		for k := 0; k < 2; k++ {
			x, y := r.uv[a][k], r.uv[c][k]
			if x.Cmp(y) > 0 {
				x, y = y, x
			}
			if r.uv[p][k].Cmp(x) < 0 || r.uv[p][k].Cmp(y) > 0 {
				return false
			}
		}
		return true
	}
	for i, x := range atoms {
		if x.a == x.c {
			return nil, geom.ErrInvalid
		}
		for _, y := range atoms[:i] {
			if (x.a == y.a && x.c == y.c) || (x.a == y.c && x.c == y.a) {
				return nil, geom.ErrDomain
			}
			if cross(x.a, x.c, y.a).Sign()*cross(x.a, x.c, y.c).Sign() < 0 && cross(y.a, y.c, x.a).Sign()*cross(y.a, y.c, x.c).Sign() < 0 {
				return nil, geom.ErrDomain
			}
			for _, v := range []int{x.a, x.c} {
				if v != y.a && v != y.c && on(y.a, y.c, v) {
					return nil, geom.ErrDomain
				}
			}
			for _, v := range []int{y.a, y.c} {
				if v != x.a && v != x.c && on(x.a, x.c, v) {
					return nil, geom.ErrDomain
				}
			}
		}
	}
	// The authenticated original affine chart is injective on this convex
	// rectangle. Thus every segment between its exact nodes stays on the original
	// face. A maximal noncrossing augmentation removes pinched face walks without
	// moving a node or assuming which side belongs to either Boolean operand.
	if o.AuxiliarySubdivision {
		domain := s.charts[owner][face].patch.Domain()
		lo := [2]*big.Rat{ratInset(domain.UMin), ratInset(domain.VMin)}
		hi := [2]*big.Rat{ratInset(domain.UMax), ratInset(domain.VMax)}
		for _, p := range r.uv {
			for k, v := range p {
				if v.Cmp(lo[k]) < 0 || v.Cmp(hi[k]) > 0 {
					return nil, geom.ErrDomain
				}
			}
		}
		auxiliary := 0
		for a := 0; a < len(r.uv); a++ {
			for c := a + 1; c < len(r.uv); c++ {
				if b.err != nil {
					return nil, b.err
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				valid := true
				for _, e := range atoms {
					if (a == e.a && c == e.c) || (a == e.c && c == e.a) {
						valid = false
						break
					}
					if cross(a, c, e.a).Sign()*cross(a, c, e.c).Sign() < 0 && cross(e.a, e.c, a).Sign()*cross(e.a, e.c, c).Sign() < 0 {
						valid = false
						break
					}
					for _, v := range []int{e.a, e.c} {
						if v != a && v != c && on(a, c, v) {
							valid = false
							break
						}
					}
					for _, v := range []int{a, c} {
						if v != e.a && v != e.c && on(e.a, e.c, v) {
							valid = false
							break
						}
					}
					if !valid {
						break
					}
				}
				if valid {
					atoms = append(atoms, atom{a, c, AffineImprintFragmentUse{OriginalEdge: -1, OriginalLocalEdge: -1, SectionEdge: -1, AuxiliaryEdge: auxiliary}})
					auxiliary++
				}
			}
		}
	}
	type dart struct {
		from, to, atom int
		rev            bool
	}
	darts := []dart{}
	outgoing := make([][]int, len(r.uv))
	for i, e := range atoms {
		j := len(darts)
		darts = append(darts, dart{e.a, e.c, i, false}, dart{e.c, e.a, i, true})
		outgoing[e.a] = append(outgoing[e.a], j)
		outgoing[e.c] = append(outgoing[e.c], j+1)
	}
	for v, list := range outgoing {
		sort.Slice(list, func(i, j int) bool {
			a, c := r.uv[darts[list[i]].to], r.uv[darts[list[j]].to]
			p := r.uv[v]
			ax, ay := b.op('-', a[0], p[0]), b.op('-', a[1], p[1])
			cx, cy := b.op('-', c[0], p[0]), b.op('-', c[1], p[1])
			half := func(x, y *big.Rat) bool { return y.Sign() > 0 || (y.Sign() == 0 && x.Sign() >= 0) }
			ha, hc := half(ax, ay), half(cx, cy)
			if ha != hc {
				return ha
			}
			return b.op('-', b.op('*', ax, cy), b.op('*', ay, cx)).Sign() > 0
		})
		outgoing[v] = list
	}
	seen := make([]bool, len(darts))
	count := make([]int, len(atoms))
	orientation := make([]int, len(atoms))
	negative := 0
	sum := new(big.Rat)
	for start := range darts {
		if seen[start] {
			continue
		}
		cycle := AffineImprintFragmentCycle{Area: new(big.Rat)}
		visited := map[int]bool{}
		d := start
		for steps := 0; ; steps++ {
			if steps > len(darts) || seen[d] {
				return nil, geom.ErrDomain
			}
			seen[d] = true
			x := darts[d]
			if visited[x.from] {
				return nil, geom.ErrDomain
			}
			visited[x.from] = true
			p, q := r.uv[x.from], r.uv[x.to]
			cycle.Area = b.op('+', cycle.Area, b.op('-', b.op('*', p[0], q[1]), b.op('*', p[1], q[0])))
			use := atoms[x.atom].use
			use.Start, use.End = x.from, x.to
			if x.rev {
				use.Parameter[0], use.Parameter[1] = use.Parameter[1], use.Parameter[0]
			}
			cycle.Uses = append(cycle.Uses, use)
			list := outgoing[x.to]
			at := -1
			for i, k := range list {
				if k == d^1 {
					at = i
					break
				}
			}
			if at < 0 {
				return nil, geom.ErrInvalid
			}
			d = list[(at+len(list)-1)%len(list)]
			if d == start {
				break
			}
		}
		cycle.Area = b.op('/', cycle.Area, big.NewRat(2, 1))
		if cycle.Area.Sign() < 0 {
			negative++
			continue
		}
		if cycle.Area.Sign() == 0 || len(cycle.Uses) < 3 {
			return nil, geom.ErrDomain
		}
		if len(r.cycles) >= o.MaxCycles {
			return nil, geom.ErrConvergence
		}
		for _, u := range cycle.Uses {
			for i, a := range atoms {
				if (a.a == u.Start && a.c == u.End) || (a.c == u.Start && a.a == u.End) {
					count[i]++
					if a.a == u.Start {
						orientation[i]++
					} else {
						orientation[i]--
					}
					break
				}
			}
		}
		sum = b.op('+', sum, cycle.Area)
		r.cycles = append(r.cycles, cycle)
	}
	if negative != 1 {
		return nil, geom.ErrDomain
	}
	domain := s.charts[owner][face].patch.Domain()
	area := b.op('*', b.op('-', ratInset(domain.UMax), ratInset(domain.UMin)), b.op('-', ratInset(domain.VMax), ratInset(domain.VMin)))
	if sum.Cmp(area) != 0 {
		return nil, geom.ErrInvalid
	}
	for i, a := range atoms {
		if a.use.SectionEdge >= 0 || a.use.AuxiliaryEdge >= 0 {
			if count[i] != 2 || orientation[i] != 0 {
				return nil, geom.ErrInvalid
			}
		} else if count[i] != 1 {
			return nil, geom.ErrInvalid
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r.area = area
	r.work = b.work
	return r, nil
}

// NativeFaces returns independent native source-surface faces, not a sewn Solid.
// All chart/model vertices must convert losslessly; no approximate endpoints or
// alternative support replaces the original. Continuous model/use identity is
// by the identical original surface and identical affine UV path on each edge.
func (r *AffineImprintFaceFragments) NativeFaces(ctx context.Context, o geom.SurfacePatchEvaluationOptions) ([]SewFace, error) {
	if ctx == nil || r == nil {
		return nil, geom.ErrInvalid
	}
	max := o.MaxOperations
	if max == 0 {
		max = 8000000
	}
	if max < 1 || max > 128000000 {
		return nil, geom.ErrInvalid
	}
	work := 0
	uv := make([]geom.Vec2, len(r.uv))
	points := make([]geom.Vec3, len(r.uv))
	patch := r.source.charts[r.owner][r.face].patch
	for i, p := range r.uv {
		var exact bool
		uv[i].X, exact = p[0].Float64()
		if !exact {
			return nil, geom.ErrDomain
		}
		uv[i].Y, exact = p[1].Float64()
		if !exact {
			return nil, geom.ErrDomain
		}
		o.MaxOperations = max - work
		if o.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		q, e := patch.EvaluateRational(ctx, p[0], p[1], o)
		if e != nil {
			return nil, e
		}
		work += q.ExactOperations
		xyz := [3]float64{}
		for k, v := range q.Point {
			xyz[k], exact = v.Float64()
			if !exact {
				return nil, geom.ErrDomain
			}
		}
		points[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	source := r.source.sources[r.owner].faces[r.face]
	out := []SewFace{}
	for _, c := range r.cycles {
		f := SewFace{Face: Face{Surface: source.Surface, Reversed: source.Reversed}}
		for i, u := range c.Uses {
			f.Vertices = append(f.Vertices, points[u.Start])
			a, b := uv[u.Start], uv[u.End]
			f.Edges = append(f.Edges, parametric.Edge{Start: i, End: (i + 1) % len(c.Uses), Curve: surfaceTrimCurve{surface: source.Surface, a: a, b: b}, Range: parametric.Interval{Max: 1}})
			f.Face.Outer = append(f.Face.Outer, Use{Edge: i, UVStart: a, UVEnd: b})
		}
		out = append(out, f)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
