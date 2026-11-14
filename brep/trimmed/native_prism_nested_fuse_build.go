package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

type NativeNestedPrismFuse struct {
	solid  brep.Solid
	source *NativeNestedPrismFuseTransition
	faces  []NativePrismFuseFaceUse
	walls  []NativePrismIntervalWallUse
	work   int
}

func (s *NativeNestedPrismFuse) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.solid
}
func (s *NativeNestedPrismFuse) Transition() *NativeNestedPrismFuseTransition {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *NativeNestedPrismFuse) Faces() []NativePrismFuseFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismFuseFaceUse(nil), s.faces...)
}
func (s *NativeNestedPrismFuse) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.walls}
	return q.WallUses()
}
func (s *NativeNestedPrismFuse) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// BuildNative sews the three certified axial layers of a strict nested Fuse.
// Exact cap triangles preserve every boundary atom, including original holes;
// all layer transitions use the same indexed vertices. Nonbinary coordinates
// reject. The closed native graph and exact oriented boundary volume must both
// agree with the original-source certificate before publication. Holed caps have
// a 96-boundary-vertex limit and all exact arithmetic is cumulatively bounded.
func (s *NativeNestedPrismFuseTransition) BuildNative(ctx context.Context, o AffineArrangementPrismOptions) (*NativeNestedPrismFuse, error) {
	return buildNestedPrismFuseMode(ctx, s, o, false)
}
func buildNestedPrismFuseMode(ctx context.Context, s *NativeNestedPrismFuseTransition, o AffineArrangementPrismOptions, tied bool) (*NativeNestedPrismFuse, error) {
	if s == nil || ctx == nil || s.union == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
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
	axis := s.Axis()
	profiles := [2]nativePrismProfile{}
	for i := range profiles {
		p, e := authenticateNativePrism(ctx, s.owners[i], axis, limit)
		if e != nil {
			return nil, e
		}
		profiles[i] = p
	}
	out := &NativeNestedPrismFuse{source: s}
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

	type atom struct {
		ring, edge int
		t          [2]*big.Rat
		point      [2][3]*big.Rat
		use        NativePrismIntervalWallUse
	}
	allAtoms := [2][]atom{}
	for owner, q := range s.proofs {
		for region, arr := range q.regions {
			v, uses := arr.Vertices(), q.BoundaryUses(region)
			for i, e := range arr.Edges() {
				if e.Owner != 0 {
					continue
				}
				u := uses[i]
				u.Use.Operand = owner
				allAtoms[owner] = append(allAtoms[owner], atom{e.Ring, e.Edge, e.Parameter, [2][3]*big.Rat{v[e.Start].Point, v[e.End].Point}, u})
			}
		}
		atoms := allAtoms[owner]
		sort.Slice(atoms, func(i, j int) bool {
			if atoms[i].ring != atoms[j].ring {
				return atoms[i].ring < atoms[j].ring
			}
			if atoms[i].edge != atoms[j].edge {
				return atoms[i].edge < atoms[j].edge
			}
			return atoms[i].t[0].Cmp(atoms[j].t[0]) < 0
		})
		atomRings := make([][]atom, 1+len(profiles[owner].trim.Holes))
		for _, a := range atoms {
			atomRings[a.ring] = append(atomRings[a.ring], a)
		}
		if owner == s.outer {
			for end, z := range []*big.Rat{s.levels[0], s.levels[3]} {
				if tied && ((end == 0 && s.levels[0].Cmp(s.levels[1]) == 0) || (end == 1 && s.levels[2].Cmp(s.levels[3]) == 0)) {
					continue
				}
				loops := [][]int{}
				for _, ring := range atomRings {
					if len(ring) < 3 {
						return nil, geom.ErrInvalid
					}
					loop := []int{}
					for i, a := range ring {
						next := ring[(i+1)%len(ring)]
						for k := 0; k < 3; k++ {
							budget.check(a.point[1][k])
							if a.point[1][k].Cmp(next.point[0][k]) != 0 {
								return nil, geom.ErrInvalid
							}
						}
						id, e := vertex(a.point[0], z)
						if e != nil {
							return nil, e
						}
						loop = append(loop, id)
					}
					loops = append(loops, loop)
				}
				normal, cap := -1, profiles[owner].lowerCap
				if end == 1 {
					normal, cap = 1, profiles[owner].upperCap
				}
				if e := triangulateAll(loops, normal, NativePrismFuseFaceUse{Operand: owner, OriginalFace: cap, Region: -1, Edge: -1, Cycle: -1}); e != nil {
					return nil, e
				}
			}
		} else {
			arr := q.Region(0)
			v, edges := arr.Vertices(), arr.Edges()
			for end, z := range s.levels[1:3] {
				loops := [][]int{}
				for _, cycle := range arr.Cycles() {
					loop := []int{}
					for _, i := range cycle.Edges {
						id, e := vertex(v[edges[i].Start].Point, z)
						if e != nil {
							return nil, e
						}
						loop = append(loop, id)
					}
					loops = append(loops, loop)
				}
				normal, cap := -1, profiles[owner].lowerCap
				if end == 1 {
					normal, cap = 1, profiles[owner].upperCap
				}
				if e := triangulateAll(loops, normal, NativePrismFuseFaceUse{Operand: owner, OriginalFace: cap, Region: owner, Edge: -1, Cycle: -1}); e != nil {
					return nil, e
				}
			}
		}
	}

	if tied {
		end, normal := 0, -1
		z := s.levels[0]
		if s.levels[0].Cmp(s.levels[1]) != 0 {
			end, normal = 1, 1
			z = s.levels[3]
		}
		// Inner-only tied cap is already published above. Split the outer cap
		// into its owner-only region and Common, preserving their internal edges.
		for region, arr := range s.proofs[s.outer].regions {
			v, edges := arr.Vertices(), arr.Edges()
			loops := [][]int{}
			for _, cycle := range arr.Cycles() {
				loop := []int{}
				for _, i := range cycle.Edges {
					id, e := vertex(v[edges[i].Start].Point, z)
					if e != nil {
						return nil, e
					}
					loop = append(loop, id)
				}
				loops = append(loops, loop)
			}
			cap := profiles[s.outer].lowerCap
			if end == 1 {
				cap = profiles[s.outer].upperCap
			}
			use := NativePrismFuseFaceUse{Operand: s.outer, OriginalFace: cap, Region: s.outer, Edge: -1, Cycle: -1}
			if region == 1 {
				use.Operand = -1
				use.OriginalFace = -1
				use.Region = 3
			}
			if e := triangulateAll(loops, normal, use); e != nil {
				return nil, e
			}
		}
	}

	wall := func(p [2][3]*big.Rat, lo, hi *big.Rat, use NativePrismIntervalWallUse, region, edge int) error {
		loop := make([]int, 4)
		for i, k := range []int{0, 1, 1, 0} {
			z := lo
			if i >= 2 {
				z = hi
			}
			id, e := vertex(p[k], z)
			if e != nil {
				return e
			}
			loop[i] = id
		}
		use.Use.Face = len(polygons)
		use.Use.Component = 0
		p0 := profiles[use.Use.Operand]
		height := budget.op('-', new(big.Rat).SetFloat64(p0.hi), new(big.Rat).SetFloat64(p0.lo))
		for i, z := range []*big.Rat{lo, hi} {
			use.HeightParameter[i] = budget.op('/', budget.op('-', z, new(big.Rat).SetFloat64(p0.lo)), height)
		}
		addFace(loop, NativePrismFuseFaceUse{Operand: use.Use.Operand, OriginalFace: use.Use.Wall, Region: region, Edge: edge, Cycle: -1})
		out.walls = append(out.walls, use)
		return budget.err
	}
	for _, span := range [][2]*big.Rat{{s.levels[0], s.levels[1]}, {s.levels[2], s.levels[3]}} {
		if tied && span[0].Cmp(span[1]) == 0 {
			continue
		}
		for _, a := range allAtoms[s.outer] {
			if e := wall(a.point, span[0], span[1], a.use, s.outer, -1); e != nil {
				return nil, e
			}
		}
	}

	v := s.union.Vertices()
	uses := s.OverlapWallUses()
	for i, e := range s.union.Edges() {
		if err := wall([2][3]*big.Rat{v[e.Start].Point, v[e.End].Point}, s.levels[1], s.levels[2], uses[i], 2, i); err != nil {
			return nil, err
		}
	}
	six := new(big.Rat)
	for _, f := range polygons {
		loop := f[0]
		a := exact[loop[0]]
		for j := 1; j+1 < len(loop); j++ {
			b, c := exact[loop[j]], exact[loop[j+1]]
			det := new(big.Rat)
			for k := 0; k < 3; k++ {
				u, w := (k+1)%3, (k+2)%3
				det = budget.op('+', det, budget.op('*', a[k], budget.op('-', budget.op('*', b[u], c[w]), budget.op('*', b[w], c[u]))))
			}
			six = budget.op('+', six, det)
		}
	}
	volume := budget.op('/', six, big.NewRat(6, 1))
	if budget.err != nil {
		return nil, budget.err
	}
	if volume.Cmp(s.volume) != 0 {
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
