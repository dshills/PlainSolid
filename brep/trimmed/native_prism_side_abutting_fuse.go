package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// NativeSideAbuttingPrismFuse removes only authenticated opposite original wall
// spans over a positive axial overlap. Caps retain their original source owners.
type NativeSideAbuttingPrismFuse struct {
	solid  brep.Solid
	source *NativePrismIntervalCommon
	faces  []NativePrismFuseFaceUse
	walls  []NativePrismIntervalWallUse
	volume *big.Rat
	work   int
}

func (s *NativeSideAbuttingPrismFuse) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.solid
}
func (s *NativeSideAbuttingPrismFuse) CommonProof() *NativePrismIntervalCommon {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *NativeSideAbuttingPrismFuse) Faces() []NativePrismFuseFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismFuseFaceUse(nil), s.faces...)
}
func (s *NativeSideAbuttingPrismFuse) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.walls}
	return q.WallUses()
}
func (s *NativeSideAbuttingPrismFuse) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativeSideAbuttingPrismFuse) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// FuseSideAbuttingNativePrisms authenticates cardinal prisms with zero footprint
// Common area and positive shared side area. Isolated extra point contacts,
// overlapping shared spans and nonrepresentable native coordinates reject.
func FuseSideAbuttingNativePrisms(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativeSideAbuttingPrismFuse, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	limit := o.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	axis := -1
	for k := 0; k < 3; k++ {
		x, e := authenticateNativePrism(ctx, a, k, limit)
		if e != nil {
			continue
		}
		y, e := authenticateNativePrism(ctx, b, k, limit)
		if e != nil {
			continue
		}
		if x.lo < y.hi && y.lo < x.hi {
			axis = k
			break
		}
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}
	common, e := commonNativePrismIntervalsAxis(ctx, a, b, o, axis)
	if e != nil {
		return nil, e
	}
	return buildSideAbuttingNativePrisms(ctx, common, o.Native)
}
func buildSideAbuttingNativePrisms(ctx context.Context, common *NativePrismIntervalCommon, o AffineArrangementPrismOptions) (*NativeSideAbuttingPrismFuse, error) {
	if ctx == nil || common == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if common.kind != NativePrismCommonContact || common.lo.Cmp(common.hi) >= 0 || len(common.profile.Area().Cycles()) != 0 || len(common.profile.SharedBoundarySpans()) == 0 || len(common.profile.IsolatedCommonPoints()) != 0 {
		return nil, geom.ErrDomain
	}
	maximum, bits, limit := o.MaxOperations, o.MaxCoefficientBits, o.MaxVertices
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if limit == 0 {
		limit = 4096
	}
	if maximum < 1 || maximum > 128000000 || bits < 64 || bits > 65536 || limit < 6 || limit > 16384 || o.MaxComponents < 0 || o.MaxComponents > 256 {
		return nil, geom.ErrInvalid
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	axis := common.axis
	profiles := [2]nativePrismProfile{}
	for i := range profiles {
		p, e := authenticateNativePrism(ctx, common.owners[i], axis, limit)
		if e != nil {
			return nil, e
		}
		profiles[i] = p
	}
	out := &NativeSideAbuttingPrismFuse{source: common}
	var vertices []geom.Vec3
	var exact [][3]*big.Rat
	var polygons [][][]int
	ids := map[[3]string]int{}
	vertex := func(p [3]*big.Rat, z *big.Rat) (int, error) {
		p[axis] = z
		key := [3]string{}
		xyz := [3]float64{}
		for k, q := range p {
			budget.check(q)
			key[k] = q.RatString()
			var ok bool
			xyz[k], ok = q.Float64()
			if !ok {
				return 0, geom.ErrDomain
			}
		}
		if budget.err != nil {
			return 0, budget.err
		}
		if id, ok := ids[key]; ok {
			return id, nil
		}
		if len(vertices) >= limit {
			return 0, geom.ErrConvergence
		}
		id := len(vertices)
		ids[key] = id
		vertices = append(vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		var q [3]*big.Rat
		for k := range q {
			q[k] = new(big.Rat).Set(p[k])
		}
		exact = append(exact, q)
		return id, nil
	}
	xy := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	orient := func(a, b, c int) *big.Rat {
		return budget.op('-', budget.op('*', budget.op('-', exact[b][xy[0]], exact[a][xy[0]]), budget.op('-', exact[c][xy[1]], exact[a][xy[1]])), budget.op('*', budget.op('-', exact[b][xy[1]], exact[a][xy[1]]), budget.op('-', exact[c][xy[0]], exact[a][xy[0]])))
	}
	addFace := func(loop []int, use NativePrismFuseFaceUse) {
		use.Face = len(polygons)
		polygons = append(polygons, [][]int{loop})
		out.faces = append(out.faces, use)
	}
	triangulate := func(loop []int, normal int, use NativePrismFuseFaceUse) error {
		rest := append([]int(nil), loop...)
		sum := new(big.Rat)
		want := new(big.Rat)
		for i, a := range loop {
			b := loop[(i+1)%len(loop)]
			want = budget.op('+', want, budget.op('-', budget.op('*', exact[a][xy[0]], exact[b][xy[1]]), budget.op('*', exact[a][xy[1]], exact[b][xy[0]])))
		}
		emit := func(a, b, c int) {
			sum = budget.op('+', sum, orient(a, b, c))
			tri := []int{a, b, c}
			if normal < 0 {
				tri[1], tri[2] = tri[2], tri[1]
			}
			addFace(tri, use)
		}
		for len(rest) > 3 {
			found := false
			for i, b := range rest {
				a, c := rest[(i+len(rest)-1)%len(rest)], rest[(i+1)%len(rest)]
				if orient(a, b, c).Sign() <= 0 {
					continue
				}
				clear := true
				for _, p := range rest {
					if p == a || p == b || p == c {
						continue
					}
					if orient(a, b, p).Sign() >= 0 && orient(b, c, p).Sign() >= 0 && orient(c, a, p).Sign() >= 0 {
						clear = false
						break
					}
				}
				if budget.err != nil {
					return budget.err
				}
				if !clear {
					continue
				}
				emit(a, b, c)
				rest = append(rest[:i], rest[i+1:]...)
				found = true
				break
			}
			if !found {
				return geom.ErrDomain
			}
		}
		if orient(rest[0], rest[1], rest[2]).Sign() <= 0 {
			return geom.ErrDomain
		}
		emit(rest[0], rest[1], rest[2])
		if budget.err != nil {
			return budget.err
		}
		if want.Sign() <= 0 || sum.Cmp(want) != 0 {
			return geom.ErrInvalid
		}
		return nil
	}
	triangulateAll := func(loops [][]int, normal int, use NativePrismFuseFaceUse) error {
		holed := false
		for _, loop := range loops {
			area := new(big.Rat)
			for i, a := range loop {
				c := loop[(i+1)%len(loop)]
				area = budget.op('+', area, budget.op('-', budget.op('*', exact[a][xy[0]], exact[c][xy[1]]), budget.op('*', exact[a][xy[1]], exact[c][xy[0]])))
			}
			if area.Sign() < 0 {
				holed = true
			}
			if area.Sign() == 0 {
				return geom.ErrDomain
			}
		}
		if budget.err != nil {
			return budget.err
		}
		if !holed {
			for ci, loop := range loops {
				u := use
				if u.Region >= 0 {
					u.Cycle = ci
				}
				if e := triangulate(loop, normal, u); e != nil {
					return e
				}
			}
			return nil
		}
		triangles, e := nativePrismHoledCapTriangles(loops, exact, xy, budget)
		if e != nil {
			return e
		}
		for _, tri := range triangles {
			loop := []int{tri.ids[0], tri.ids[1], tri.ids[2]}
			if normal < 0 {
				loop[1], loop[2] = loop[2], loop[1]
			}
			u := use
			if u.Region >= 0 {
				u.Cycle = tri.cycle
			}
			addFace(loop, u)
		}
		return nil
	}

	type key struct{ owner, ring, edge int }
	shared := map[key][][2]*big.Rat{}
	for _, span := range common.profile.SharedBoundarySpans() {
		if span.IntersectionSides[0] || span.IntersectionSides[1] || len(span.Sources) != 2 {
			return nil, geom.ErrDomain
		}
		u, v := span.Sources[0], span.Sources[1]
		if u.Owner == v.Owner || u.Parameter[1].Cmp(u.Parameter[0])*v.Parameter[1].Cmp(v.Parameter[0]) != -1 {
			return nil, geom.ErrDomain
		}
		for _, e := range span.Sources {
			p := e.Parameter
			if p[0].Cmp(p[1]) > 0 {
				p[0], p[1] = p[1], p[0]
			}
			k := key{e.Owner, e.Ring, e.Edge}
			shared[k] = append(shared[k], p)
		}
	}
	out.volume = new(big.Rat)
	for owner, profile := range profiles {
		rings := append([][][2]*big.Rat{profile.trim.Outer}, profile.trim.Holes...)
		type atom struct {
			p      [2][3]*big.Rat
			use    NativePrismIntervalWallUse
			shared bool
		}
		atoms := []atom{}
		capRings := [][]atom{}
		area := new(big.Rat)
		zlo, zhi := new(big.Rat).SetFloat64(profile.lo), new(big.Rat).SetFloat64(profile.hi)
		for ring, pts := range rings {
			ringAtoms := []atom{}
			for edge, p := range pts {
				q := pts[(edge+1)%len(pts)]
				area = budget.op('+', area, budget.op('-', budget.op('*', p[0], q[1]), budget.op('*', p[1], q[0])))
				spans := shared[key{owner, ring, edge}]
				cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 1)}
				for _, t := range spans {
					cuts = append(cuts, t[0], t[1])
				}
				sort.Slice(cuts, func(i, j int) bool { return cuts[i].Cmp(cuts[j]) < 0 })
				unique := cuts[:0]
				for _, t := range cuts {
					budget.check(t)
					if len(unique) == 0 || unique[len(unique)-1].Cmp(t) != 0 {
						unique = append(unique, t)
					}
				}
				for i := 0; i+1 < len(unique); i++ {
					ts := [2]*big.Rat{unique[i], unique[i+1]}
					n := 0
					for _, t := range spans {
						if t[0].Cmp(ts[0]) <= 0 && ts[1].Cmp(t[1]) <= 0 {
							n++
						}
					}
					if n > 1 {
						return nil, geom.ErrInvalid
					}
					side := profile.sides[ring][edge]
					x := atom{shared: n == 1, use: NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: 0, Face: -1, Operand: owner, Wall: side.wall, LowerCap: profile.lowerCap, UpperCap: profile.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}}
					for end, t := range ts {
						x.p[end][axis] = new(big.Rat).Set(zlo)
						for k := 0; k < 2; k++ {
							x.p[end][xy[k]] = budget.op('+', p[k], budget.op('*', t, budget.op('-', q[k], p[k])))
						}
						x.use.Use.LowerParameter[end] = new(big.Rat).Set(t)
						x.use.Use.UpperParameter[end] = new(big.Rat).Set(t)
						if side.lowerReverse {
							x.use.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), t)
						}
						if side.upperReverse {
							x.use.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), t)
						}
					}
					atoms = append(atoms, x)
					ringAtoms = append(ringAtoms, x)
				}
			}
			capRings = append(capRings, ringAtoms)
		}
		height := budget.op('-', zhi, zlo)
		out.volume = budget.op('+', out.volume, budget.op('*', budget.op('/', area, big.NewRat(2, 1)), height))
		for end, z := range []*big.Rat{zlo, zhi} {
			loops := [][]int{}
			for _, ring := range capRings {
				loop := []int{}
				for _, a := range ring {
					id, e := vertex(a.p[0], z)
					if e != nil {
						return nil, e
					}
					loop = append(loop, id)
				}
				loops = append(loops, loop)
			}
			cap, normal := profile.lowerCap, -1
			if end == 1 {
				cap, normal = profile.upperCap, 1
			}
			if e := triangulateAll(loops, normal, NativePrismFuseFaceUse{Operand: owner, OriginalFace: cap, Region: -1, Edge: -1, Cycle: -1}); e != nil {
				return nil, e
			}
		}
		heights := []*big.Rat{zlo, zhi}
		for _, z := range []*big.Rat{common.lo, common.hi} {
			if zlo.Cmp(z) < 0 && z.Cmp(zhi) < 0 {
				heights = append(heights, z)
			}
		}
		sort.Slice(heights, func(i, j int) bool { return heights[i].Cmp(heights[j]) < 0 })
		for _, a := range atoms {
			for i := 0; i+1 < len(heights); i++ {
				lo, hi := heights[i], heights[i+1]
				if a.shared && lo.Cmp(common.lo) >= 0 && hi.Cmp(common.hi) <= 0 {
					continue
				}
				ids := []int{}
				for j, k := range []int{0, 1, 1, 0} {
					z := lo
					if j >= 2 {
						z = hi
					}
					id, e := vertex(a.p[k], z)
					if e != nil {
						return nil, e
					}
					ids = append(ids, id)
				}
				u := a.use
				u.Use.Face = len(polygons)
				u.HeightParameter = [2]*big.Rat{budget.op('/', budget.op('-', lo, zlo), height), budget.op('/', budget.op('-', hi, zlo), height)}
				addFace(ids, NativePrismFuseFaceUse{Operand: owner, OriginalFace: u.Use.Wall, Region: owner, Edge: -1, Cycle: -1})
				out.walls = append(out.walls, u)
			}
		}
	}
	six := new(big.Rat)
	for _, f := range polygons {
		loop := f[0]
		a := exact[loop[0]]
		for j := 1; j+1 < len(loop); j++ {
			b, c := exact[loop[j]], exact[loop[j+1]]
			for k := 0; k < 3; k++ {
				u, v := (k+1)%3, (k+2)%3
				six = budget.op('+', six, budget.op('*', a[k], budget.op('-', budget.op('*', b[u], c[v]), budget.op('*', b[v], c[u]))))
			}
		}
	}
	volume := budget.op('/', six, big.NewRat(6, 1))
	if budget.err != nil {
		return nil, budget.err
	}
	if volume.Cmp(out.volume) != 0 {
		return nil, geom.ErrInvalid
	}
	var e error
	out.solid, e = buildArrangementPrismFaces(vertices, polygons)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}
