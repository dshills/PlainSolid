package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// NativePrismFuseBoundary pairs one overlap-layer Union edge with an original
// owner-only shoulder edge. Shoulder is original operand 0 or 1, never a swapped
// helper owner; ShoulderEdge indexes Shoulder(Shoulder).Edges().
type NativePrismFuseBoundary struct{ UnionEdge, Shoulder, ShoulderEdge int }

// NativePrismFuseTransition certifies three axial layers of a strict staggered
// crossing-profile Fuse. It does not publish a sewn native solid. Original cap
// shoulders are A-B and B-A; overlap walls follow the complete A union B profile.
type NativePrismFuseTransition struct {
	owners                [2]brep.Solid
	proofs                [2]*NativePrismCutTransition
	union                 *intersect.AffinePolygonArrangement
	boundary              []NativePrismFuseBoundary
	uses                  []NativePrismIntervalWallUse
	levels                [4]*big.Rat
	controls              [4]NativePrismCapOwner
	volume                *big.Rat
	work, arrangementWork int
}

func (s *NativePrismFuseTransition) OriginalOwner(i int) (brep.Solid, error) {
	if s == nil || i < 0 || i > 1 {
		return brep.Solid{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *NativePrismFuseTransition) Shoulder(i int) *intersect.AffinePolygonArrangement {
	if s == nil || i < 0 || i > 1 {
		return nil
	}
	return s.proofs[i].Region(0)
}

// ShoulderLevel and ShoulderNormalSign specify actual model placement. The
// stored shoulder arrangement is projected onto the other owner's cap plane;
// it must be transported along Axis to this exact level before native use.
func (s *NativePrismFuseTransition) ShoulderLevel(i int) *big.Rat {
	if s == nil || i < 0 || i > 1 {
		return nil
	}
	return new(big.Rat).Set(s.proofs[i].entry)
}
func (s *NativePrismFuseTransition) ShoulderNormalSign(i int) int {
	if s == nil || i < 0 || i > 1 {
		return 0
	}
	return s.proofs[i].normal
}
func (s *NativePrismFuseTransition) Axis() int {
	if s == nil {
		return -1
	}
	return s.proofs[0].axis
}
func (s *NativePrismFuseTransition) UnionProfile() *intersect.AffinePolygonArrangement {
	if s == nil {
		return nil
	}
	return s.union
}
func (s *NativePrismFuseTransition) BoundaryPairs() []NativePrismFuseBoundary {
	if s == nil {
		return nil
	}
	return append([]NativePrismFuseBoundary(nil), s.boundary...)
}

// Levels is strictly increasing; CapOwners identifies every original cap.
func (s *NativePrismFuseTransition) Levels() [4]*big.Rat {
	var out [4]*big.Rat
	if s != nil {
		for i, q := range s.levels {
			out[i] = new(big.Rat).Set(q)
		}
	}
	return out
}
func (s *NativePrismFuseTransition) CapOwners() [4]NativePrismCapOwner {
	if s == nil {
		return [4]NativePrismCapOwner{}
	}
	return s.controls
}
func (s *NativePrismFuseTransition) OverlapWallUses() []NativePrismIntervalWallUse {
	if s == nil {
		return nil
	}
	q := NativePrismIntervalCommon{uses: s.uses}
	return q.WallUses()
}
func (s *NativePrismFuseTransition) ExactMaterialVolume() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.volume)
}
func (s *NativePrismFuseTransition) Operations() (int, int) {
	if s == nil {
		return 0, 0
	}
	return s.work, s.arrangementWork
}

// ImprintCrossingNativePrismFuse requires strict staggered original intervals,
// transversely crossing complete profiles and positive owner-only shoulders.
// Both original ring tables and their overlap are independently re-proved;
// all exposed Union edges must match exactly one owner-only shoulder edge.
// Native and arrangement arithmetic quotas are cumulative across both shoulder
// proofs and the union proof. Contact degeneracies and nested spans reject.
func ImprintCrossingNativePrismFuse(ctx context.Context, a, b brep.Solid, o NativePrismBooleanOptions) (*NativePrismFuseTransition, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	maximum, arrMax, bits := o.Native.MaxOperations, o.Arrangement.MaxOperations, o.Native.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if arrMax == 0 {
		arrMax = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	if maximum < 1 || maximum > 128000000 || arrMax < 1 || arrMax > 128000000 || bits < 64 || bits > 65536 {
		return nil, geom.ErrInvalid
	}
	out := &NativePrismFuseTransition{owners: [2]brep.Solid{a, b}}
	for i := 0; i < 2; i++ {
		limits := o
		limits.Native.MaxOperations = maximum - out.work
		limits.Arrangement.MaxOperations = arrMax - out.arrangementWork
		if limits.Native.MaxOperations < 1 || limits.Arrangement.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		q, e := ImprintCrossingNativePrismCut(ctx, out.owners[i], out.owners[1-i], limits)
		if e != nil {
			return nil, e
		}
		out.proofs[i] = q
		out.work += q.work
		out.arrangementWork += q.arrangementWork
	}
	x, y := out.proofs[0], out.proofs[1]
	if x.axis != y.axis || x.normal != -y.normal {
		return nil, geom.ErrDomain
	}
	limit := o.Native.MaxVertices
	if limit == 0 {
		limit = 4096
	}
	profiles := [2]nativePrismProfile{}
	for i := range profiles {
		p, e := authenticateNativePrism(ctx, out.owners[i], x.axis, limit)
		if e != nil {
			return nil, e
		}
		profiles[i] = p
	}
	lowOwner, highOwner := 0, 1
	if profiles[0].lo > profiles[1].lo {
		lowOwner, highOwner = 1, 0
	}
	low, high := profiles[lowOwner], profiles[highOwner]
	if !(low.lo < high.lo && high.lo < low.hi && low.hi < high.hi) {
		return nil, geom.ErrDomain
	}
	for i, z := range []float64{low.lo, high.lo, low.hi, high.hi} {
		out.levels[i] = new(big.Rat).SetFloat64(z)
	}
	out.controls = [4]NativePrismCapOwner{{lowOwner, low.lowerCap}, {highOwner, high.lowerCap}, {lowOwner, low.upperCap}, {highOwner, high.upperCap}}
	patch, e := x.Region(0).OriginalContact().Owner(0)
	if e != nil {
		return nil, e
	}
	remain := o.Arrangement
	remain.MaxOperations = arrMax - out.arrangementWork
	if remain.MaxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	out.union, e = intersect.ArrangeTransverseAffinePolygons(ctx, patch, patch, [2]intersect.AffinePolygonTrim{profiles[0].trim, profiles[1].trim}, intersect.AffineRingUnion, o.Patches, remain)
	if e != nil {
		return nil, e
	}
	out.arrangementWork += out.union.ExactOperations()
	budget := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits, work: out.work}
	// Compare complete Common boundary tables in original owner/ring/edge units.
	common := [2][]intersect.AffineArrangementEdge{x.Region(1).Edges(), y.Region(1).Edges()}
	if len(common[0]) != len(common[1]) {
		return nil, geom.ErrInvalid
	}
	matched := make([]bool, len(common[1]))
	for _, u := range common[0] {
		found := -1
		for j, v := range common[1] {
			budget.check(u.Parameter[0])
			if u.Owner == 1-v.Owner && u.Ring == v.Ring && u.Edge == v.Edge && u.Parameter[0].Cmp(v.Parameter[0]) == 0 && u.Parameter[1].Cmp(v.Parameter[1]) == 0 {
				if found >= 0 || matched[j] {
					return nil, geom.ErrInvalid
				}
				found = j
			}
		}
		if found < 0 {
			return nil, geom.ErrInvalid
		}
		matched[found] = true
	}
	shoulder := [2][]intersect.AffineArrangementEdge{x.Region(0).Edges(), y.Region(0).Edges()}
	used := [2][]bool{make([]bool, len(shoulder[0])), make([]bool, len(shoulder[1]))}
	uv := out.union.Vertices()
	for i, u := range out.union.Edges() {
		owner := u.Owner
		found := -1
		for j, v := range shoulder[owner] {
			budget.check(u.Parameter[0])
			if v.Owner == 0 && u.Ring == v.Ring && u.Edge == v.Edge && u.Parameter[0].Cmp(v.Parameter[0]) == 0 && u.Parameter[1].Cmp(v.Parameter[1]) == 0 {
				if found >= 0 || used[owner][j] {
					return nil, geom.ErrInvalid
				}
				found = j
			}
		}
		if found < 0 {
			return nil, geom.ErrInvalid
		}
		used[owner][found] = true
		// Independent projected model endpoint identity, without comparing different
		// original cap heights or rounding their original edge parameters.
		q := out.proofs[owner]
		v := q.Region(0).Vertices()
		se := shoulder[owner][found]
		for _, pair := range [][2]int{{u.Start, se.Start}, {u.End, se.End}} {
			for k := 0; k < 3; k++ {
				if k == x.axis {
					continue
				}
				budget.check(uv[pair[0]].Point[k])
				if uv[pair[0]].Point[k].Cmp(v[pair[1]].Point[k]) != 0 {
					return nil, geom.ErrInvalid
				}
			}
		}
		out.boundary = append(out.boundary, NativePrismFuseBoundary{i, owner, found})
		use := q.BoundaryUses(0)[found]
		use.Use.Operand = owner
		p := profiles[owner]
		height := budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))
		for end, z := range out.levels[1:3] {
			use.HeightParameter[end] = budget.op('/', budget.op('-', z, new(big.Rat).SetFloat64(p.lo)), height)
		}
		out.uses = append(out.uses, use)
	}
	for owner, edges := range shoulder {
		for i, e := range edges {
			if e.Owner == 0 && !used[owner][i] {
				return nil, geom.ErrInvalid
			}
		}
	}
	area := func(q *intersect.AffinePolygonArrangement) *big.Rat {
		sum := new(big.Rat)
		for _, c := range q.Cycles() {
			sum = budget.op('+', sum, c.SignedAreaA)
		}
		return sum
	}
	commonArea := area(x.Region(1))
	if commonArea.Cmp(area(y.Region(1))) != 0 {
		return nil, geom.ErrInvalid
	}
	aa, ba := budget.op('+', area(x.Region(0)), commonArea), budget.op('+', area(y.Region(0)), commonArea)
	if area(out.union).Cmp(budget.op('-', budget.op('+', aa, ba), commonArea)) != 0 {
		return nil, geom.ErrInvalid
	}
	volume := new(big.Rat)
	for i, ar := range []*big.Rat{aa, ba} {
		p := profiles[i]
		volume = budget.op('+', volume, budget.op('*', ar, budget.op('-', new(big.Rat).SetFloat64(p.hi), new(big.Rat).SetFloat64(p.lo))))
	}
	out.volume = budget.op('-', volume, budget.op('*', commonArea, budget.op('-', out.levels[2], out.levels[1])))
	if budget.err != nil {
		return nil, budget.err
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.work = budget.work
	return out, nil
}
