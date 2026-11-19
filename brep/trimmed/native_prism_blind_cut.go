//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativeBlindPrismFaceUse binds an output face to its actual original support.
// Reversed is true for tool floor/walls. Opened stock caps retain stock support;
// the new hole boundary separately comes from the tool profile.
type NativeBlindPrismFaceUse struct {
	Face, Operand, OriginalFace int
	Reversed                    bool
}
type NativePrismBlindCut struct {
	solid        brep.Solid
	owners       [2]brep.Solid
	axis, opened int
	faces        []NativeBlindPrismFaceUse
	walls        []NativePrismIntervalWallUse
	volume       *big.Rat
	work         int
}

func (s *NativePrismBlindCut) Solid() brep.Solid {
	if s == nil {
		return brep.Solid{}
	}
	return s.solid
}
func (s *NativePrismBlindCut) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismBlindCut) Faces() []NativeBlindPrismFaceUse {
	if s == nil {
		return nil
	}
	return append([]NativeBlindPrismFaceUse(nil), s.faces...)
}
func (s *NativePrismBlindCut) OpenedCap() int {
	if s == nil {
		return -1
	}
	return s.opened
}
func (s *NativePrismBlindCut) Floor() int {
	if s == nil {
		return -1
	}
	return 2
}
func (s *NativePrismBlindCut) ExactVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativePrismBlindCut) NativeOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *NativePrismBlindCut) WallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	tmp := &NativePrismIntervalCommon{uses: s.walls}
	return tmp.WallUses()
}

// CutContainedNativePrismBlind proves a finite blind recess in actual original
// coordinate-axis prism B-reps. Both profiles must be simple and hole-free; the
// tool profile lies strictly inside stock and crosses exactly one stock cap.
// Original walls/caps are authenticated, the internal tool cap becomes a reversed
// floor, and all native interfaces share exact indexed vertices/edges. Profile
// tangency, through cuts, internal closed cavities and partial shoulders reject.
func CutContainedNativePrismBlind(ctx context.Context, stock, tool brep.Solid, o NativePrismBooleanOptions) (*NativePrismBlindCut, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	limit := o.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	maximum, bits := o.Native.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if limit < 6 || limit > 16384 || maximum < 1 || maximum > 128000000 || bits < 64 || bits > 65536 || o.Native.MaxComponents < 0 || o.Native.MaxComponents > 256 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	profiles := [2]nativePrismProfile{}
	axis := -1
	topEntry := false
	var floor, entry float64
	for candidate := 0; candidate < 3; candidate++ {
		a, e := authenticateNativePrism(ctx, stock, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		b, e := authenticateNativePrism(ctx, tool, candidate, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if len(a.trim.Holes) > 0 || len(b.trim.Holes) > 0 {
			continue
		}
		up := a.lo < b.lo && b.lo < a.hi && a.hi <= b.hi
		down := b.lo <= a.lo && a.lo < b.hi && b.hi < a.hi
		if !up && !down {
			continue
		}
		bounds := a.bounds
		for j := 0; j < 2; j++ {
			if b.bounds[j][0] < bounds[j][0] {
				bounds[j][0] = b.bounds[j][0]
			}
			if b.bounds[j][1] > bounds[j][1] {
				bounds[j][1] = b.bounds[j][1]
			}
		}
		axes := [2]int{(candidate + 1) % 3, (candidate + 2) % 3}
		poles := []geom.Vec3{}
		for _, u := range bounds[0] {
			for _, v := range bounds[1] {
				xyz := [3]float64{}
				xyz[candidate] = a.lo
				xyz[axes[0]], xyz[axes[1]] = u, v
				poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
			}
		}
		surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
		if e != nil {
			return nil, e
		}
		patch := intersect.Patch{Surface: surface, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
		proof, e := intersect.SelectNoncontactingAffinePolygonRings(ctx, patch, patch, [2]intersect.AffinePolygonTrim{a.trim, b.trim}, intersect.AffineRingIntersection, o.Patches, o.Arrangement)
		if e != nil {
			return nil, e
		}
		rings := proof.Rings()
		if len(rings) != 1 || rings[0].Owner != 1 || rings[0].Ring != 0 {
			return nil, geom.ErrDomain
		}
		profiles = [2]nativePrismProfile{a, b}
		axis = candidate
		topEntry = up
		if up {
			floor, entry = b.lo, a.hi
		} else {
			floor, entry = b.hi, a.lo
		}
		break
	}
	if axis < 0 {
		return nil, geom.ErrDomain
	}
	a, b := profiles[0], profiles[1]
	n, m := len(a.trim.Outer), len(b.trim.Outer)
	if 2*(n+m) > limit {
		return nil, geom.ErrConvergence
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	axes := [2]int{(axis + 1) % 3, (axis + 2) % 3}
	vertices := []geom.Vec3{}
	addRing := func(ring [][2]*big.Rat, height float64) ([]int, error) {
		ids := make([]int, len(ring))
		for i, uv := range ring {
			xyz := [3]float64{}
			xyz[axis] = height
			budget.check(new(big.Rat).SetFloat64(height))
			for j, q := range uv {
				budget.check(q)
				x, exact := q.Float64()
				if !exact {
					return nil, geom.ErrDomain
				}
				xyz[axes[j]] = x
			}
			if budget.err != nil {
				return nil, budget.err
			}
			ids[i] = len(vertices)
			vertices = append(vertices, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		}
		return ids, nil
	}
	alo, e := addRing(a.trim.Outer, a.lo)
	if e != nil {
		return nil, e
	}
	ahi, e := addRing(a.trim.Outer, a.hi)
	if e != nil {
		return nil, e
	}
	toolLo, toolHi := entry, floor
	if topEntry {
		toolLo, toolHi = floor, entry
	}
	blo, e := addRing(b.trim.Outer, toolLo)
	if e != nil {
		return nil, e
	}
	bhi, e := addRing(b.trim.Outer, toolHi)
	if e != nil {
		return nil, e
	}
	reverse := func(ids []int) []int { out := append([]int(nil), ids...); reverseArrangementPrismIDs(out); return out }
	polygons := [][][]int{{reverse(alo)}, {append([]int(nil), ahi...)}, nil}
	opened := 0
	floorSource := b.upperCap
	if topEntry {
		opened = 1
		floorSource = b.lowerCap
		polygons[1] = append(polygons[1], reverse(bhi))
		polygons[2] = [][]int{append([]int(nil), blo...)}
	} else {
		polygons[0] = append(polygons[0], append([]int(nil), blo...))
		polygons[2] = [][]int{reverse(bhi)}
	}
	out := &NativePrismBlindCut{owners: [2]brep.Solid{stock, tool}, axis: axis, opened: opened}
	out.faces = []NativeBlindPrismFaceUse{{0, 0, a.lowerCap, false}, {1, 0, a.upperCap, false}, {2, 1, floorSource, true}}
	for owner, ring := range [][]int{alo, blo} {
		upper := ahi
		if owner == 1 {
			upper = bhi
		}
		p := profiles[owner]
		low, high := a.lo, a.hi
		if owner == 1 {
			low, high = toolLo, toolHi
		}
		for i, id := range ring {
			j := (i + 1) % len(ring)
			loop := []int{id, ring[j], upper[j], upper[i]}
			if owner == 1 {
				reverseArrangementPrismIDs(loop)
			}
			face := len(polygons)
			polygons = append(polygons, [][]int{loop})
			side := p.sides[0][i]
			out.faces = append(out.faces, NativeBlindPrismFaceUse{face, owner, side.wall, owner == 1})
			history := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: 0, Face: face, Operand: owner, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
			height := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
			for end, q := range []float64{low, high} {
				history.HeightParameter[end] = budget.op('/', budget.op('-', new(big.Rat).SetFloat64(q), new(big.Rat).SetFloat64(p.lo)), height)
				history.Use.LowerParameter[end] = big.NewRat(int64(end), 1)
				history.Use.UpperParameter[end] = big.NewRat(int64(end), 1)
				if side.lowerReverse {
					history.Use.LowerParameter[end].Sub(big.NewRat(1, 1), history.Use.LowerParameter[end])
				}
				if side.upperReverse {
					history.Use.UpperParameter[end].Sub(big.NewRat(1, 1), history.Use.UpperParameter[end])
				}
			}
			out.walls = append(out.walls, history)
		}
	}
	area := func(ring [][2]*big.Rat) *big.Rat {
		sum := new(big.Rat)
		for i, v := range ring {
			w := ring[(i+1)%len(ring)]
			sum = budget.op('+', sum, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
		}
		return budget.op('/', sum, big.NewRat(2, 1))
	}
	stockVolume := budget.op('*', area(a.trim.Outer), budget.op('-', new(big.Rat).SetFloat64(a.hi), new(big.Rat).SetFloat64(a.lo)))
	removed := budget.op('*', area(b.trim.Outer), budget.op('-', new(big.Rat).SetFloat64(toolHi), new(big.Rat).SetFloat64(toolLo)))
	out.volume = budget.op('-', stockVolume, removed)
	if budget.err != nil {
		return nil, budget.err
	}
	if out.volume.Sign() <= 0 {
		return nil, geom.ErrInvalid
	}
	out.solid, e = buildArrangementPrismFaces(vertices, polygons)
	if e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}
