package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// NativePrismTransitionFaceUse retains the original supporting face of every
// triangle/quad. Region is 0 for retained entry cap or cut-span walls, 1 for the
// tool floor, and -1 for the opposite stock cap or uncut-span walls. Edge indexes
// the transition arrangement for cut-span walls; it is -1 for cap triangles.
// Cycle identifies a cap region cycle, or is -1 for walls/opposite stock cap.
type NativePrismTransitionFaceUse struct {
	Face, Operand, OriginalFace, Region, Edge, Cycle int
	Reversed                                         bool
}
type NativeCrossingPrismCut struct {
	solid  brep.Solid
	source *NativePrismCutTransition
	faces  []NativePrismTransitionFaceUse
	walls  []NativePrismIntervalWallUse
	work   int
}

func (s *NativeCrossingPrismCut) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.solid
}
func (s *NativeCrossingPrismCut) Transition() *NativePrismCutTransition {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *NativeCrossingPrismCut) Faces() []NativePrismTransitionFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativePrismTransitionFaceUse(nil), s.faces...)
}
func (s *NativeCrossingPrismCut) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.walls}
	return q.WallUses()
}
func (s *NativeCrossingPrismCut) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}

// BuildNative consumes the authenticated exact imprint. This first domain has
// one or more simple positive cycles in each region (no holes). Exact cap triangulation
// preserves every crossing atom; wall quads split at the internal tool cap.
// All shared vertices are losslessly represented; unrepresentable coordinates
// reject rather than changing the original source geometry. The final closed
// graph and its exact signed boundary volume are independently rechecked.
func (s *NativePrismCutTransition) BuildNative(ctx context.Context, o AffineArrangementPrismOptions) (*NativeCrossingPrismCut, error) {
	if s == nil || ctx == nil || s.regions[0] == nil || s.regions[1] == nil {
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
	for _, a := range s.regions {
		c := a.Cycles()
		if len(c) == 0 {
			return nil, geom.ErrDomain
		}
		for _, cycle := range c {
			if cycle.SignedAreaA.Sign() <= 0 {
				return nil, geom.ErrDomain
			}
		}
	}
	profiles := [2]nativePrismProfile{}
	for i := range profiles {
		p, e := authenticateNativePrism(ctx, s.owners[i], s.axis, limit)
		if e != nil {
			return nil, e
		}
		profiles[i] = p
	}
	// Original stock edge intervals, already proved gap-free by the imprint.
	type atom struct {
		edge  int
		t     [2]*big.Rat
		point [2][3]*big.Rat
		use   NativePrismIntervalWallUse
	}
	atoms := []atom{}
	for region, a := range s.regions {
		v := a.Vertices()
		uses := s.BoundaryUses(region)
		for i, e := range a.Edges() {
			if e.Owner == 0 {
				atoms = append(atoms, atom{e.Edge, e.Parameter, [2][3]*big.Rat{v[e.Start].Point, v[e.End].Point}, uses[i]})
			}
		}
	}
	sort.Slice(atoms, func(i, j int) bool {
		if atoms[i].edge != atoms[j].edge {
			return atoms[i].edge < atoms[j].edge
		}
		return atoms[i].t[0].Cmp(atoms[j].t[0]) < 0
	})
	if len(atoms) < 3 {
		return nil, geom.ErrInvalid
	}
	for i, a := range atoms {
		b := atoms[(i+1)%len(atoms)]
		for k := 0; k < 3; k++ {
			budget.check(a.point[1][k])
			if a.point[1][k].Cmp(b.point[0][k]) != 0 {
				return nil, geom.ErrInvalid
			}
		}
	}
	out := &NativeCrossingPrismCut{source: s}
	var vertices []geom.Vec3
	var exact [][3]*big.Rat
	var polygons [][][]int
	ids := map[[3]string]int{}
	vertex := func(p [3]*big.Rat, z *big.Rat) (int, error) {
		p[s.axis] = z
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
	xy := [2]int{(s.axis + 1) % 3, (s.axis + 2) % 3}
	orient := func(a, b, c int) *big.Rat {
		return budget.op('-', budget.op('*', budget.op('-', exact[b][xy[0]], exact[a][xy[0]]), budget.op('-', exact[c][xy[1]], exact[a][xy[1]])), budget.op('*', budget.op('-', exact[b][xy[1]], exact[a][xy[1]]), budget.op('-', exact[c][xy[0]], exact[a][xy[0]])))
	}
	addFace := func(loop []int, use NativePrismTransitionFaceUse) {
		use.Face = len(polygons)
		polygons = append(polygons, [][]int{loop})
		out.faces = append(out.faces, use)
	}
	triangulate := func(loop []int, normal int, use NativePrismTransitionFaceUse) error {
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
	opposite := new(big.Rat).SetFloat64(profiles[0].lo)
	oppositeCap := profiles[0].lowerCap
	if s.normal < 0 {
		opposite.SetFloat64(profiles[0].hi)
		oppositeCap = profiles[0].upperCap
	}
	stockLoop := []int{}
	for _, a := range atoms {
		id, e := vertex(a.point[0], opposite)
		if e != nil {
			return nil, e
		}
		stockLoop = append(stockLoop, id)
	}
	if e := triangulate(stockLoop, -s.normal, NativePrismTransitionFaceUse{Operand: 0, OriginalFace: oppositeCap, Region: -1, Edge: -1, Cycle: -1}); e != nil {
		return nil, e
	}
	for region, arr := range s.regions {
		z := s.entry
		owner := s.entryOwner
		if region == 1 {
			z = s.floor
			owner = s.floorOwner
		}
		v, e := arr.Vertices(), arr.Edges()
		for ci, cycle := range arr.Cycles() {
			loop := []int{}
			for _, i := range cycle.Edges {
				id, err := vertex(v[e[i].Start].Point, z)
				if err != nil {
					return nil, err
				}
				loop = append(loop, id)
			}
			if err := triangulate(loop, s.normal, NativePrismTransitionFaceUse{Operand: owner.Operand, OriginalFace: owner.Face, Region: region, Edge: -1, Cycle: ci, Reversed: region == 1}); err != nil {
				return nil, err
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
		original := profiles[use.Use.Operand]
		height := budget.op('-', new(big.Rat).SetFloat64(original.hi), new(big.Rat).SetFloat64(original.lo))
		for i, z := range []*big.Rat{lo, hi} {
			use.HeightParameter[i] = budget.op('/', budget.op('-', z, new(big.Rat).SetFloat64(original.lo)), height)
		}
		addFace(loop, NativePrismTransitionFaceUse{Operand: use.Use.Operand, OriginalFace: use.Use.Wall, Region: region, Edge: edge, Cycle: -1, Reversed: use.Use.Operand == 1})
		out.walls = append(out.walls, use)
		return budget.err
	}
	uncutLo, uncutHi := opposite, s.floor
	cutLo, cutHi := s.floor, s.entry
	if s.normal < 0 {
		uncutLo, uncutHi = s.floor, opposite
		cutLo, cutHi = s.entry, s.floor
	}
	for _, a := range atoms {
		if e := wall(a.point, uncutLo, uncutHi, a.use, -1, -1); e != nil {
			return nil, e
		}
	}
	arr := s.regions[0]
	v := arr.Vertices()
	uses := s.BoundaryUses(0)
	for i, e := range arr.Edges() {
		if err := wall([2][3]*big.Rat{v[e.Start].Point, v[e.End].Point}, cutLo, cutHi, uses[i], 0, i); err != nil {
			return nil, err
		}
	}
	// Exact oriented boundary integration is independent of the axial volume
	// formula. This also checks triangle/quad publication orientation.
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
	var err error
	out.solid, err = buildArrangementPrismFaces(vertices, polygons)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	out.work = budget.work
	return out, nil
}
