package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
)

type nativeNestedPrismProfile struct {
	regions [2]*intersect.AffinePolygonArrangement
	uses    [2][]NativePrismIntervalWallUse
}

func (s *nativeNestedPrismProfile) Region(i int) *intersect.AffinePolygonArrangement {
	return s.regions[i]
}
func (s *nativeNestedPrismProfile) BoundaryUses(i int) []NativePrismIntervalWallUse {
	q := NativePrismIntervalCommon{uses: s.uses[i]}
	return q.WallUses()
}

// NativeNestedPrismFuseTransition is an immutable proof of a three-layer Fuse
// whose outer original interval strictly contains the other. Both shoulders are
// the inner owner's profile minus the outer owner's profile, at its original caps.
type NativeNestedPrismFuseTransition struct {
	owners                [2]brep.Solid
	proofs                [2]*nativeNestedPrismProfile
	union                 *intersect.AffinePolygonArrangement
	uses                  []NativePrismIntervalWallUse
	levels                [4]*big.Rat
	outer, axis           int
	volume                *big.Rat
	work, arrangementWork int
}

func (s *NativeNestedPrismFuseTransition) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativeNestedPrismFuseTransition) OuterOwner() int {
	if s == nil {
		return -1
	}
	return s.outer
}
func (s *NativeNestedPrismFuseTransition) Axis() int {
	if s == nil {
		return -1
	}
	return s.axis
}
func (s *NativeNestedPrismFuseTransition) Levels() [4]*big.Rat {
	var r [4]*big.Rat
	if s != nil {
		for i, q := range s.levels {
			r[i] = new(big.Rat).Set(q)
		}
	}
	return r
}
func (s *NativeNestedPrismFuseTransition) UnionProfile() *intersect.AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.union
}
func (s *NativeNestedPrismFuseTransition) ShoulderProfile() *intersect.AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.proofs[1-s.outer].Region(0)
}
func (s *NativeNestedPrismFuseTransition) OverlapWallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.uses}
	return q.WallUses()
}
func (s *NativeNestedPrismFuseTransition) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativeNestedPrismFuseTransition) Operations() (int, int) {
	if s == nil {
		return 0, 0
	}
	return s.work, s.arrangementWork
}

// ImprintNestedCrossingNativePrismFuse authenticates strict nested axial spans
// and genuinely transverse profiles. Original rings may have holes. Complete
// owner-only and Common partitions account for all source boundaries, with
// exactly reversed paired internal edges. Contact/equal cap cases reject.
func ImprintNestedCrossingNativePrismFuse(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativeNestedPrismFuseTransition, error) {
	return imprintNestedCrossingNativePrismProfiles(ctx, a, b, o, -1)
}

func imprintNestedCrossingNativePrismProfiles(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions, requiredOuter int) (*NativeNestedPrismFuseTransition, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	maximum, arrMax, bits, limit := o.Native.MaxOperations, o.Arrangement.MaxOperations, o.Native.MaxCoefficientBits, o.Native.MaxVertices
	if maximum == 0 {
		maximum = 8000000
	}
	if arrMax == 0 {
		arrMax = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if limit == 0 {
		limit = 4096
	}
	if maximum < 1 || maximum > 128000000 || arrMax < 1 || arrMax > 128000000 || bits < 64 || bits > 65536 || limit < 6 || limit > 16384 || o.Native.MaxComponents < 0 || o.Native.MaxComponents > 256 {
		return nil, geom.ErrInvalid
	}
	out := &NativeNestedPrismFuseTransition{owners: [2]brep.Solid{a, b}, axis: -1}
	profiles := [2]nativePrismProfile{}
	for axis := 0; axis < 3; axis++ {
		x, e := authenticateNativePrism(ctx, a, axis, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		y, e := authenticateNativePrism(ctx, b, axis, limit)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		outer := -1
		if x.lo < y.lo && y.hi < x.hi {
			outer = 0
		} else if y.lo < x.lo && x.hi < y.hi {
			outer = 1
		}
		if outer >= 0 && (requiredOuter < 0 || outer == requiredOuter) {
			profiles = [2]nativePrismProfile{x, y}
			out.axis, out.outer = axis, outer
			break
		}
	}
	if out.axis < 0 {
		return nil, geom.ErrDomain
	}
	outer, inner := profiles[out.outer], profiles[1-out.outer]
	for i, z := range []float64{outer.lo, inner.lo, inner.hi, outer.hi} {
		out.levels[i] = new(big.Rat).SetFloat64(z)
	}
	bounds := profiles[0].bounds
	for j := 0; j < 2; j++ {
		if profiles[1].bounds[j][0] < bounds[j][0] {
			bounds[j][0] = profiles[1].bounds[j][0]
		}
		if profiles[1].bounds[j][1] > bounds[j][1] {
			bounds[j][1] = profiles[1].bounds[j][1]
		}
	}
	axes := [2]int{(out.axis + 1) % 3, (out.axis + 2) % 3}
	poles := []geom.Vec3{}
	for _, u := range bounds[0] {
		for _, v := range bounds[1] {
			xyz := [3]float64{}
			xyz[out.axis] = inner.lo
			xyz[axes[0]], xyz[axes[1]] = u, v
			poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		}
	}
	surf, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
	if e != nil {
		return nil, e
	}
	patch := intersect.Patch{Surface: surf, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
	arrange := func(owner int, op intersect.AffineRingOperation) (*intersect.AffinePolygonArrangement, error) {
		limits := o.Arrangement
		limits.MaxOperations = arrMax - out.arrangementWork
		if limits.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		q, e := intersect.ArrangeTransverseAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{profiles[owner].trim, profiles[1-owner].trim}, op, o.Patches, limits)
		if e != nil {
			return nil, e
		}
		out.arrangementWork += q.ExactOperations()
		return q, nil
	}
	for owner := 0; owner < 2; owner++ {
		q := &nativeNestedPrismProfile{}
		for r, op := range []intersect.AffineRingOperation{intersect.AffineRingDifference, intersect.AffineRingIntersection} {
			q.regions[r], e = arrange(owner, op)
			if e != nil {
				return nil, e
			}
		}
		out.proofs[owner] = q
	}
	out.union, e = arrange(0, intersect.AffineRingUnion)
	if e != nil {
		return nil, e
	}
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits}
	makeUse := func(owner int, edge intersect.AffineArrangementEdge) NativePrismIntervalWallUse {
		p := profiles[owner]
		side := p.sides[edge.Ring][edge.Edge]
		u := NativePrismIntervalWallUse{Use: NativePrismWallUse{Component: -1, Face: -1, Operand: owner, Wall: side.wall, LowerCap: p.lowerCap, UpperCap: p.upperCap, LowerEdge: side.lower, UpperEdge: side.upper}}
		height := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
		for end, z := range out.levels[1:3] {
			u.HeightParameter[end] = budget.op('/', budget.op('-', z, new(big.Rat).SetFloat64(p.lo)), height)
			u.Use.LowerParameter[end] = new(big.Rat).Set(edge.Parameter[end])
			u.Use.UpperParameter[end] = new(big.Rat).Set(edge.Parameter[end])
			if side.lowerReverse {
				u.Use.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.LowerParameter[end])
			}
			if side.upperReverse {
				u.Use.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), u.Use.UpperParameter[end])
			}
		}
		return u
	}
	area := func(q *intersect.AffinePolygonArrangement) *big.Rat {
		sum := new(big.Rat)
		for _, c := range q.Cycles() {
			sum = budget.op('+', sum, c.SignedAreaA)
		}
		return sum
	}
	regionAreas := [2][2]*big.Rat{}
	for owner, q := range out.proofs {
		rings := append([][][2]*big.Rat{profiles[owner].trim.Outer}, profiles[owner].trim.Holes...)
		parts := make([][][][2]*big.Rat, len(rings))
		for r, ring := range rings {
			parts[r] = make([][][2]*big.Rat, len(ring))
		}
		edges := [2][]intersect.AffineArrangementEdge{q.Region(0).Edges(), q.Region(1).Edges()}
		for r, arr := range q.regions {
			regionAreas[owner][r] = area(arr)
			if regionAreas[owner][r].Sign() <= 0 {
				return nil, geom.ErrDomain
			}
			for _, edge := range edges[r] {
				actual := owner
				if edge.Owner == 1 {
					actual = 1 - owner
				}
				q.uses[r] = append(q.uses[r], makeUse(actual, edge))
				if edge.Owner == 0 {
					if edge.Parameter[0].Cmp(edge.Parameter[1]) >= 0 {
						return nil, geom.ErrInvalid
					}
					parts[edge.Ring][edge.Edge] = append(parts[edge.Ring][edge.Edge], edge.Parameter)
				}
			}
		}
		crossed := false
		for _, ring := range parts {
			for _, intervals := range ring {
				sortNativePrismIntervals(intervals)
				if len(intervals) > 1 {
					crossed = true
				}
				cursor := new(big.Rat)
				for _, p := range intervals {
					budget.check(p[0])
					if p[0].Cmp(cursor) != 0 {
						return nil, geom.ErrInvalid
					}
					cursor = p[1]
				}
				if cursor.Cmp(big.NewRat(1, 1)) != 0 {
					return nil, geom.ErrInvalid
				}
			}
		}
		if !crossed {
			return nil, geom.ErrDomain
		}
		used := make([]bool, len(edges[1]))
		for _, u := range edges[0] {
			if u.Owner != 1 {
				continue
			}
			found := -1
			for j, v := range edges[1] {
				budget.check(u.Parameter[0])
				if v.Owner == 1 && u.Ring == v.Ring && u.Edge == v.Edge && u.Parameter[0].Cmp(v.Parameter[1]) == 0 && u.Parameter[1].Cmp(v.Parameter[0]) == 0 {
					if found >= 0 || used[j] {
						return nil, geom.ErrInvalid
					}
					found = j
				}
			}
			if found < 0 {
				return nil, geom.ErrInvalid
			}
			used[found] = true
		}
		for j, u := range edges[1] {
			if u.Owner == 1 && !used[j] {
				return nil, geom.ErrInvalid
			}
		}
		sum := new(big.Rat)
		for _, ring := range rings {
			for i, v := range ring {
				w := ring[(i+1)%len(ring)]
				sum = budget.op('+', sum, budget.op('-', budget.op('*', v[0], w[1]), budget.op('*', v[1], w[0])))
			}
		}
		sum = budget.op('/', sum, big.NewRat(2, 1))
		if sum.Cmp(budget.op('+', regionAreas[owner][0], regionAreas[owner][1])) != 0 {
			return nil, geom.ErrInvalid
		}
	}
	common := regionAreas[0][1]
	if common.Cmp(regionAreas[1][1]) != 0 {
		return nil, geom.ErrInvalid
	}
	used := [2][]bool{make([]bool, len(out.proofs[0].Region(0).Edges())), make([]bool, len(out.proofs[1].Region(0).Edges()))}
	for _, u := range out.union.Edges() {
		found := -1
		for j, v := range out.proofs[u.Owner].Region(0).Edges() {
			budget.check(u.Parameter[0])
			if v.Owner == 0 && u.Ring == v.Ring && u.Edge == v.Edge && u.Parameter[0].Cmp(v.Parameter[0]) == 0 && u.Parameter[1].Cmp(v.Parameter[1]) == 0 {
				if found >= 0 || used[u.Owner][j] {
					return nil, geom.ErrInvalid
				}
				found = j
			}
		}
		if found < 0 {
			return nil, geom.ErrInvalid
		}
		used[u.Owner][found] = true
		out.uses = append(out.uses, makeUse(u.Owner, u))
	}
	for owner, q := range out.proofs {
		for j, e := range q.Region(0).Edges() {
			if e.Owner == 0 && !used[owner][j] {
				return nil, geom.ErrInvalid
			}
		}
	}
	unionArea := budget.op('+', budget.op('+', regionAreas[0][0], regionAreas[1][0]), common)
	if area(out.union).Cmp(unionArea) != 0 {
		return nil, geom.ErrInvalid
	}
	outerArea := budget.op('+', regionAreas[out.outer][0], common)
	out.volume = budget.op('+', budget.op('*', outerArea, budget.op('-', out.levels[3], out.levels[0])), budget.op('*', regionAreas[1-out.outer][0], budget.op('-', out.levels[2], out.levels[1])))
	if budget.err != nil {
		return nil, budget.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}

func sortNativePrismIntervals(p [][2]*big.Rat) {
	sort.Slice(p, func(i, j int) bool { return p[i][0].Cmp(p[j][0]) < 0 })
}
