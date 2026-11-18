//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// PrismCapBoundaryUse identifies original cap edges generating a region boundary;
// these owners need not contribute retained area (notably a Cut tool boundary).
type PrismCapBoundaryUse struct {
	Operand, LowerRing, UpperRing, Edge, LowerEdge, UpperEdge int
	LowerParameter, UpperParameter                            [2]*big.Rat
}
type PrismCapBoundaryPiece struct {
	Base, Top [2][3]*big.Rat
	Sources   []PrismCapBoundaryUse
}

// PrismCapContribution partitions one output component's base/top cap area.
// Owners bits identify A-only1, B-only2 or shared3 area. OriginalCaps contains
// lower/upper input cap IDs only for contributing owners (-1 otherwise).
// Loops are an oriented outer followed by holes; boundary source uses are separate.
type PrismCapContribution struct {
	Component     int
	Faces         [2]int
	Owners        uint8
	OriginalCaps  [2][2]int
	ProjectedArea *big.Rat
	Loops         [][]PrismCapBoundaryPiece
}
type PrismCapContributions struct {
	regions      []PrismCapContribution
	scaleSquared *big.Rat
	work         int
}

// AreaScaleSquared is the exact squared metric converting ProjectedArea to
// physical cap area; it is1 for cardinal charts and rational for oblique charts.
func (s *PrismCapContributions) AreaScaleSquared() *big.Rat {
	if s == nil {
		return nil
	}
	return new(big.Rat).Set(s.scaleSquared)
}
func (s *PrismCapContributions) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.work
}
func (s *PrismCapContributions) Regions() []PrismCapContribution {
	if s == nil {
		return nil
	}
	out := append([]PrismCapContribution(nil), s.regions...)
	for i, r := range out {
		out[i].ProjectedArea = new(big.Rat).Set(r.ProjectedArea)
		out[i].Loops = make([][]PrismCapBoundaryPiece, len(r.Loops))
		for j, loop := range r.Loops {
			out[i].Loops[j] = append([]PrismCapBoundaryPiece(nil), loop...)
			for k, p := range loop {
				v := &out[i].Loops[j][k]
				for end := 0; end < 2; end++ {
					for axis := 0; axis < 3; axis++ {
						v.Base[end][axis] = new(big.Rat).Set(p.Base[end][axis])
						v.Top[end][axis] = new(big.Rat).Set(p.Top[end][axis])
					}
				}
				v.Sources = append([]PrismCapBoundaryUse(nil), p.Sources...)
				for u, s := range v.Sources {
					for end := 0; end < 2; end++ {
						v.Sources[u].LowerParameter[end] = new(big.Rat).Set(s.LowerParameter[end])
						v.Sources[u].UpperParameter[end] = new(big.Rat).Set(s.UpperParameter[end])
					}
				}
			}
		}
	}
	return out
}

// CapContributions re-proves disjoint A-only/B-only/shared retained cap regions
// from original profiles. Complete Boolean partitions are grouped into actual
// output components and their exact areas must sum to each native cap's area.
// Quotas are cumulative across partition arrangements and bookkeeping; original
// support recovery retains the separately bounded patch stages. No old single-
// source face-history contract is overloaded.
func (s *NativePrismBoolean) CapContributions(ctx context.Context, p geom.SurfacePatchOptions, o intersect.AffinePolygonContactOptions) (*PrismCapContributions, error) {
	if s == nil || s.material == nil || ctx == nil {
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
	budget := &nativePrismAffineBudget{ctx: ctx, max: o.MaxOperations, bits: o.MaxCoefficientBits}
	profiles := [2]nativePrismProfile{}
	for owner := 0; owner < 2; owner++ {
		q, e := authenticateNativePrism(ctx, s.owners[owner], s.axis, 2*o.MaxEdges)
		if e != nil {
			return nil, e
		}
		if q.lowerCap != s.caps[owner][0] || q.upperCap != s.caps[owner][1] {
			return nil, geom.ErrInvalid
		}
		profiles[owner] = q
	}
	bounds := profiles[0].bounds
	for axis := 0; axis < 2; axis++ {
		if profiles[1].bounds[axis][0] < bounds[axis][0] {
			bounds[axis][0] = profiles[1].bounds[axis][0]
		}
		if profiles[1].bounds[axis][1] > bounds[axis][1] {
			bounds[axis][1] = profiles[1].bounds[axis][1]
		}
	}
	axes := [2]int{(s.axis + 1) % 3, (s.axis + 2) % 3}
	poles := []geom.Vec3{}
	for _, u := range bounds[0] {
		for _, v := range bounds[1] {
			xyz := [3]float64{}
			xyz[s.axis] = profiles[0].lo
			xyz[axes[0]], xyz[axes[1]] = u, v
			poles = append(poles, geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]})
		}
	}
	support, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{bounds[0][0], bounds[0][0], bounds[0][1], bounds[0][1]}, []float64{bounds[1][0], bounds[1][0], bounds[1][1], bounds[1][1]}, nil)
	if e != nil {
		return nil, e
	}
	patch := intersect.Patch{Surface: support, Min: geom.Vec2{X: bounds[0][0], Y: bounds[1][0]}, Max: geom.Vec2{X: bounds[0][1], Y: bounds[1][1]}}
	type task struct {
		owners  uint8
		op      intersect.AffineRingOperation
		swapped bool
	}
	tasks := []task{{3, intersect.AffineRingIntersection, false}}
	switch s.operation {
	case intersect.AffineRingUnion:
		tasks = []task{{1, intersect.AffineRingDifference, false}, {2, intersect.AffineRingDifference, true}, {3, intersect.AffineRingIntersection, false}}
	case intersect.AffineRingDifference:
		tasks = []task{{1, intersect.AffineRingDifference, false}}
	}
	cross := func(a, b, c [2]*big.Rat) *big.Rat {
		return budget.op('-', budget.op('*', budget.op('-', b[0], a[0]), budget.op('-', c[1], a[1])), budget.op('*', budget.op('-', b[1], a[1]), budget.op('-', c[0], a[0])))
	}
	locate := func(q [2]*big.Rat, cycle intersect.AffineArrangementCycle, vertices []intersect.AffineArrangementVertex, edges []intersect.AffineArrangementEdge) (bool, bool) {
		wind := 0
		for _, ei := range cycle.Edges {
			v, w := vertices[edges[ei].Start].A, vertices[edges[ei].End].A
			side := cross(v, w, q).Sign()
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
					return true, true
				}
			}
			if v[1].Cmp(q[1]) <= 0 && w[1].Cmp(q[1]) > 0 && side > 0 {
				wind++
			}
			if v[1].Cmp(q[1]) > 0 && w[1].Cmp(q[1]) <= 0 && side < 0 {
				wind--
			}
		}
		return wind != 0, false
	}
	original := s.material.Arrangement().Area()
	ov, oe, oc := original.Vertices(), original.Edges(), original.Cycles()
	components := s.material.ComponentCycles()
	totals := make([]*big.Rat, len(components))
	for i := range totals {
		totals[i] = new(big.Rat)
	}
	out := &PrismCapContributions{scaleSquared: big.NewRat(1, 1)}
	for _, task := range tasks {
		if budget.work >= budget.max {
			return nil, geom.ErrConvergence
		}
		trims := [2]intersect.AffinePolygonTrim{profiles[0].trim, profiles[1].trim}
		if task.swapped {
			trims[0], trims[1] = trims[1], trims[0]
		}
		limits := o
		limits.MaxOperations = budget.max - budget.work
		arr, e := intersect.ArrangeGeneralAffinePolygons(ctx, patch, patch, trims, task.op, p, limits)
		if e != nil {
			return nil, e
		}
		budget.work += arr.ExactOperations()
		v, edges, cycles := arr.Area().Vertices(), arr.Area().Edges(), arr.Area().Cycles()
		sources := arr.EdgeSources()
		groups := [][]int{}
		for ci, c := range cycles {
			if c.SignedAreaA.Sign() > 0 {
				groups = append(groups, []int{ci})
			}
		}
		for ci, c := range cycles {
			if c.SignedAreaA.Sign() > 0 {
				continue
			}
			q := v[edges[c.Edges[0]].Start].A
			parent := -1
			for gi, g := range groups {
				in, on := locate(q, cycles[g[0]], v, edges)
				if on {
					return nil, geom.ErrInvalid
				}
				if in && (parent < 0 || cycles[g[0]].SignedAreaA.Cmp(cycles[groups[parent][0]].SignedAreaA) < 0) {
					parent = gi
				}
			}
			if budget.err != nil {
				return nil, budget.err
			}
			if parent < 0 {
				return nil, geom.ErrInvalid
			}
			groups[parent] = append(groups[parent], ci)
		}
		for _, group := range groups {
			q := v[edges[cycles[group[0]].Edges[0]].Start].A
			component := -1
			for index, output := range components {
				in, _ := locate(q, oc[output[0]], ov, oe)
				if !in {
					continue
				}
				for _, hole := range output[1:] {
					inside, on := locate(q, oc[hole], ov, oe)
					if inside && !on {
						in = false
					}
				}
				if in {
					if component >= 0 {
						return nil, geom.ErrInvalid
					}
					component = index
				}
			}
			if budget.err != nil {
				return nil, budget.err
			}
			if component < 0 {
				return nil, geom.ErrInvalid
			}
			region := PrismCapContribution{Component: component, Faces: [2]int{0, 1}, Owners: task.owners, OriginalCaps: [2][2]int{{-1, -1}, {-1, -1}}, ProjectedArea: new(big.Rat)}
			for owner := 0; owner < 2; owner++ {
				if task.owners&(1<<owner) != 0 {
					region.OriginalCaps[owner] = s.caps[owner]
				}
			}
			for _, ci := range group {
				region.ProjectedArea = budget.op('+', region.ProjectedArea, cycles[ci].SignedAreaA)
				loop := []PrismCapBoundaryPiece{}
				for _, ei := range cycles[ci].Edges {
					edge := edges[ei]
					piece := PrismCapBoundaryPiece{}
					for end, id := range []int{edge.Start, edge.End} {
						for axis := 0; axis < 3; axis++ {
							piece.Base[end][axis] = new(big.Rat).Set(v[id].Point[axis])
							piece.Top[end][axis] = new(big.Rat).Set(v[id].Point[axis])
						}
						piece.Top[end][s.axis] = new(big.Rat).SetFloat64(profiles[0].hi)
					}
					for _, use := range sources[ei] {
						owner := use.Owner
						if task.swapped {
							owner = 1 - owner
						}
						side := profiles[owner].sides[use.Ring][use.Edge]
						original := PrismCapBoundaryUse{Operand: owner, LowerRing: side.lowerRing, UpperRing: use.Ring, Edge: use.Edge, LowerEdge: side.lower, UpperEdge: side.upper}
						for end := 0; end < 2; end++ {
							original.LowerParameter[end] = new(big.Rat).Set(use.Parameter[end])
							original.UpperParameter[end] = new(big.Rat).Set(use.Parameter[end])
							if side.lowerReverse {
								original.LowerParameter[end] = budget.op('-', big.NewRat(1, 1), original.LowerParameter[end])
							}
							if side.upperReverse {
								original.UpperParameter[end] = budget.op('-', big.NewRat(1, 1), original.UpperParameter[end])
							}
						}
						piece.Sources = append(piece.Sources, original)
					}
					loop = append(loop, piece)
				}
				region.Loops = append(region.Loops, loop)
			}
			if region.ProjectedArea.Sign() <= 0 {
				return nil, geom.ErrInvalid
			}
			totals[component] = budget.op('+', totals[component], region.ProjectedArea)
			out.regions = append(out.regions, region)
		}
	}
	for component, group := range components {
		want := new(big.Rat)
		for _, ci := range group {
			want = budget.op('+', want, oc[ci].SignedAreaA)
		}
		if budget.err != nil {
			return nil, budget.err
		}
		if totals[component].Cmp(want) != 0 {
			return nil, geom.ErrInvalid
		}
	}
	if budget.err != nil {
		return nil, budget.err
	}
	out.work = budget.work
	return out, nil
}

// CapContributions retains the same original membership proof for an oblique
// native result. Boundary coordinates are mapped by the proved rational inverse;
// physical area is ProjectedArea*sqrt(AreaScaleSquared), never a rounded claim.
func (s *AffineNativePrismBoolean) CapContributions(ctx context.Context, p geom.SurfacePatchOptions, o intersect.AffinePolygonContactOptions) (*PrismCapContributions, error) {
	if s == nil || s.canonical == nil {
		return nil, geom.ErrInvalid
	}
	out, e := s.canonical.CapContributions(ctx, p, o)
	if e != nil {
		return nil, e
	}
	maximum, bits := o.MaxOperations, o.MaxCoefficientBits
	if maximum == 0 {
		maximum = 8000000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &nativePrismAffineBudget{ctx: ctx, max: maximum, bits: bits, work: out.work}
	for _, row := range s.inverse {
		for _, q := range row {
			b.check(q)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	for ri := range out.regions {
		for li := range out.regions[ri].Loops {
			for pi := range out.regions[ri].Loops[li] {
				piece := &out.regions[ri].Loops[li][pi]
				for _, vertices := range []*[2][3]*big.Rat{&piece.Base, &piece.Top} {
					for end := 0; end < 2; end++ {
						q := [3]*big.Rat{}
						for axis := 0; axis < 3; axis++ {
							q[axis] = new(big.Rat).Set(s.inverse[axis][3])
							for j := 0; j < 3; j++ {
								q[axis] = b.op('+', q[axis], b.op('*', s.inverse[axis][j], vertices[end][j]))
							}
						}
						vertices[end] = q
					}
				}
			}
		}
	}
	j, k := (s.canonical.axis+1)%3, (s.canonical.axis+2)%3
	metric := new(big.Rat)
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		cross := b.op('-', b.op('*', s.inverse[u][j], s.inverse[v][k]), b.op('*', s.inverse[v][j], s.inverse[u][k]))
		metric = b.op('+', metric, b.op('*', cross, cross))
	}
	if b.err != nil {
		return nil, b.err
	}
	if metric.Sign() <= 0 {
		return nil, geom.ErrInvalid
	}
	out.scaleSquared = metric
	out.work = b.work
	return out, nil
}
