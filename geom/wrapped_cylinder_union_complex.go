package geom

import (
	"context"
	"math/big"
	"runtime"
	"sort"
)

// CylinderQuotientVertex is an exact model point at canonical source UV.
// Only opposite fundamental U endpoints are identified; no proximity welds.
type CylinderQuotientVertex struct {
	UV    [2]*big.Rat
	Point [3]*big.Rat
}
type CylinderQuotientEdge struct {
	Start, End int
	UV         [2][2]*big.Rat
}

// PeriodOffset is the exact source-period translation between this directed
// use and the stored edge after applying Reversed. UV retains the face lift.
type CylinderQuotientUse struct {
	Edge         int
	Reversed     bool
	UV           [2][2]*big.Rat
	PeriodOffset *big.Int
}

// OwnerSourceTurns lifts canonical U to each owner's rectangle expressed in
// A coordinates. OwnerUV then maps that lift into the original owner chart.
// Absent owners have nil coordinates/turns. These are chart occurrences, not
// duplicate physical layers. Uses are CCW and outward on the source cylinder.
type CylinderQuotientFace struct {
	Bounds           [2][2]*big.Rat
	Owners           uint8
	OwnerUV          [2][4][2]*big.Rat
	OwnerSourceTurns [2]*big.Int
	Uses             [4]CylinderQuotientUse
}

// WrappedCylinderUnionComplex is an immutable exact annular surface complex.
// Rational UV and model coordinates are native to this result; it does not
// round them into float SewFaces, claim a closed shell, or supply a volume.
// Two boundary cycles have source angular winding +1 and -1, respectively.
type WrappedCylinderUnionComplex struct {
	source       *WrappedSurfacePatch
	vertices     []CylinderQuotientVertex
	edges        []CylinderQuotientEdge
	faces        []CylinderQuotientFace
	boundaries   [2][]CylinderQuotientUse
	orientations [2]int
	operations   int
	complete     bool
}

func copyQuotientUV(p [2]*big.Rat) (out [2]*big.Rat) {
	for i, q := range p {
		if q != nil {
			out[i] = new(big.Rat).Set(q)
		}
	}
	return
}
func copyQuotientUse(u CylinderQuotientUse) CylinderQuotientUse {
	for i := range u.UV {
		u.UV[i] = copyQuotientUV(u.UV[i])
	}
	if u.PeriodOffset != nil {
		u.PeriodOffset = new(big.Int).Set(u.PeriodOffset)
	}
	return u
}
func (r *WrappedCylinderUnionComplex) Complete() bool { return r != nil && r.complete }
func (r *WrappedCylinderUnionComplex) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}
func (r *WrappedCylinderUnionComplex) OwnerOrientations() [2]int {
	if r == nil {
		return [2]int{}
	}
	return r.orientations
}
func (r *WrappedCylinderUnionComplex) Vertices() []CylinderQuotientVertex {
	if r == nil {
		return nil
	}
	out := append([]CylinderQuotientVertex(nil), r.vertices...)
	for i := range out {
		out[i].UV = copyQuotientUV(out[i].UV)
		for axis, q := range out[i].Point {
			out[i].Point[axis] = new(big.Rat).Set(q)
		}
	}
	return out
}
func (r *WrappedCylinderUnionComplex) Edges() []CylinderQuotientEdge {
	if r == nil {
		return nil
	}
	out := append([]CylinderQuotientEdge(nil), r.edges...)
	for i := range out {
		for end := range out[i].UV {
			out[i].UV[end] = copyQuotientUV(out[i].UV[end])
		}
	}
	return out
}
func (r *WrappedCylinderUnionComplex) Faces() []CylinderQuotientFace {
	if r == nil {
		return nil
	}
	out := append([]CylinderQuotientFace(nil), r.faces...)
	for i := range out {
		f := &out[i]
		for axis := range f.Bounds {
			f.Bounds[axis] = copyQuotientUV(f.Bounds[axis])
		}
		for owner := range f.OwnerUV {
			if f.OwnerSourceTurns[owner] != nil {
				f.OwnerSourceTurns[owner] = new(big.Int).Set(f.OwnerSourceTurns[owner])
			}
			for corner := range f.OwnerUV[owner] {
				f.OwnerUV[owner][corner] = copyQuotientUV(f.OwnerUV[owner][corner])
			}
		}
		for j := range f.Uses {
			f.Uses[j] = copyQuotientUse(f.Uses[j])
		}
	}
	return out
}
func (r *WrappedCylinderUnionComplex) BoundaryCycles() [2][]CylinderQuotientUse {
	var out [2][]CylinderQuotientUse
	if r == nil {
		return out
	}
	for i, c := range r.boundaries {
		for _, u := range c {
			out[i] = append(out[i], copyQuotientUse(u))
		}
	}
	return out
}

// EvaluateEdgeRational evaluates the immutable original wrapped model line at
// exact normalized edge parameter t. UV and model points are never rounded.
func (r *WrappedCylinderUnionComplex) EvaluateEdgeRational(ctx context.Context, edge int, t *big.Rat, o SurfacePatchEvaluationOptions) (RationalSurfacePatchJet, error) {
	var zero RationalSurfacePatchJet
	if ctx == nil || !r.Complete() || edge < 0 || edge >= len(r.edges) || t == nil || t.Sign() < 0 || t.Cmp(big.NewRat(1, 1)) > 0 {
		return zero, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return zero, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	if e := b.check(t); e != nil {
		return zero, e
	}
	var uv [2]*big.Rat
	for axis := 0; axis < 2; axis++ {
		ends := r.edges[edge].UV
		uv[axis] = new(big.Rat).Add(ends[0][axis], new(big.Rat).Mul(t, new(big.Rat).Sub(ends[1][axis], ends[0][axis])))
		runtime.KeepAlive(&ends) // golang/go#82026: keeps ends in the stack map. Remove once the Go fix is in use.
		if e := b.check(uv[axis]); e != nil {
			return zero, e
		}
	}
	left := o.MaxOperations - b.operations
	if left < 1 {
		return zero, ErrConvergence
	}
	result, e := r.source.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	result.ExactOperations += b.operations
	return result, nil
}

// CertifyCylinderUnionComplex re-proves the complete two-overlap cylinder case
// and partitions canonical angular/axial endpoint stations into at most fifteen
// cells. Exact full-period edge correspondence, incidence/fans, Euler zero and
// two simple oppositely winding boundaries are all required before publication.
// This is separate from the projected rectangle algorithm: no full angular
// hull is falsely described as projection-injective. Nonbinary cuts are valid.
func (r *WrappedSurfaceChartRelation) CertifyCylinderUnionComplex(ctx context.Context, a, b [2][2]*big.Rat, o WrappedCylinderTwoOverlapOptions) (*WrappedCylinderUnionComplex, error) {
	proof, e := r.CertifyCylinderTwoRectangleOverlap(ctx, a, b, o)
	if e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits, operations: proof.ExactOperations}
	period := r.source.period[0]
	domain := r.source.Domain()
	lo, hi := new(big.Rat).SetFloat64(domain.UMin), new(big.Rat).SetFloat64(domain.UMax)
	floor := func(q *big.Rat) *big.Int {
		n, rem := new(big.Int), new(big.Int)
		n.QuoRem(q.Num(), q.Denom(), rem)
		if rem.Sign() < 0 {
			n.Sub(n, big.NewInt(1))
		}
		return n
	}
	var mappedB [2][2]*big.Rat
	for axis, target := range r.axis {
		for end := 0; end < 2; end++ {
			q := new(big.Rat).Quo(new(big.Rat).Sub(b[target][end], r.offset[axis]), r.scale[axis])
			if e := budget.check(q); e != nil {
				return nil, e
			}
			mappedB[axis][end] = q
		}
		if mappedB[axis][0].Cmp(mappedB[axis][1]) > 0 {
			mappedB[axis][0], mappedB[axis][1] = mappedB[axis][1], mappedB[axis][0]
		}
	}
	owners := [2][2][2]*big.Rat{a, mappedB}
	ucuts := []*big.Rat{lo, hi}
	var vcuts []*big.Rat
	for _, rect := range owners {
		for end := 0; end < 2; end++ {
			q := new(big.Rat).Quo(new(big.Rat).Sub(rect[0][end], lo), period)
			if e := budget.check(q); e != nil {
				return nil, e
			}
			q.Sub(rect[0][end], new(big.Rat).Mul(new(big.Rat).SetInt(floor(q)), period))
			if e := budget.check(q); e != nil {
				return nil, e
			}
			ucuts = append(ucuts, q)
			vcuts = append(vcuts, new(big.Rat).Set(rect[1][end]))
		}
	}
	unique := func(c []*big.Rat) []*big.Rat {
		sort.Slice(c, func(i, j int) bool { return c[i].Cmp(c[j]) < 0 })
		out := c[:0]
		for _, q := range c {
			if len(out) == 0 || q.Cmp(out[len(out)-1]) != 0 {
				out = append(out, q)
			}
		}
		return out
	}
	ucuts, vcuts = unique(ucuts), unique(vcuts)
	if len(ucuts) < 5 || len(ucuts) > 6 || len(vcuts) < 2 || len(vcuts) > 4 {
		return nil, ErrConvergence
	}
	out := &WrappedCylinderUnionComplex{source: r.source, orientations: proof.OwnerOrientations}
	vertices := map[string]int{}
	edges := map[[2]int]int{}
	for ui := 0; ui < len(ucuts)-1; ui++ {
		var turns [2]*big.Int
		var angular [2]bool
		for owner, rect := range owners {
			q := new(big.Rat).Quo(new(big.Rat).Sub(rect[0][0], ucuts[ui]), period)
			if e := budget.check(q); e != nil {
				return nil, e
			}
			k := new(big.Int).Neg(floor(new(big.Rat).Neg(q)))
			shift := new(big.Rat).Mul(new(big.Rat).SetInt(k), period)
			lower := new(big.Rat).Add(ucuts[ui], shift)
			upper := new(big.Rat).Add(ucuts[ui+1], shift)
			for _, q := range []*big.Rat{lower, upper} {
				if e := budget.check(q); e != nil {
					return nil, e
				}
			}
			angular[owner] = lower.Cmp(rect[0][0]) >= 0 && upper.Cmp(rect[0][1]) <= 0
			turns[owner] = k
		}
		if !angular[0] && !angular[1] {
			return nil, ErrConvergence
		}
		for vi := 0; vi < len(vcuts)-1; vi++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			f := CylinderQuotientFace{Bounds: [2][2]*big.Rat{{ucuts[ui], ucuts[ui+1]}, {vcuts[vi], vcuts[vi+1]}}}
			for owner, rect := range owners {
				if angular[owner] && vcuts[vi].Cmp(rect[1][0]) >= 0 && vcuts[vi+1].Cmp(rect[1][1]) <= 0 {
					f.Owners |= 1 << owner
					f.OwnerSourceTurns[owner] = new(big.Int).Set(turns[owner])
				}
			}
			if f.Owners == 0 {
				continue
			}
			corners := [4][2]*big.Rat{{ucuts[ui], vcuts[vi]}, {ucuts[ui+1], vcuts[vi]}, {ucuts[ui+1], vcuts[vi+1]}, {ucuts[ui], vcuts[vi+1]}}
			var ids [4]int
			for i, p := range corners {
				canonical := copyQuotientUV(p)
				if canonical[0].Cmp(hi) == 0 {
					canonical[0].Set(lo)
				}
				key := canonical[0].RatString() + "," + canonical[1].RatString()
				id, exists := vertices[key]
				if !exists {
					if budget.operations >= o.MaxOperations {
						return nil, ErrConvergence
					}
					jet, e := r.source.EvaluateRational(ctx, canonical[0], canonical[1], SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - budget.operations, MaxCoefficientBits: o.MaxCoefficientBits})
					if e != nil {
						return nil, e
					}
					budget.operations += jet.ExactOperations
					id = len(out.vertices)
					vertices[key] = id
					out.vertices = append(out.vertices, CylinderQuotientVertex{UV: canonical, Point: jet.Point})
				}
				ids[i] = id
				for owner := 0; owner < 2; owner++ {
					if f.Owners&(1<<owner) == 0 {
						continue
					}
					lift := copyQuotientUV(p)
					lift[0].Add(lift[0], new(big.Rat).Mul(new(big.Rat).SetInt(turns[owner]), period))
					if e := budget.check(lift[0]); e != nil {
						return nil, e
					}
					if owner == 0 {
						f.OwnerUV[owner][i] = lift
					} else {
						for axis, target := range r.axis {
							q := new(big.Rat).Add(r.offset[axis], new(big.Rat).Mul(r.scale[axis], lift[axis]))
							if e := budget.check(q); e != nil {
								return nil, e
							}
							f.OwnerUV[owner][i][target] = q
						}
					}
				}
			}
			for i, p := range corners {
				j := (i + 1) % 4
				start, end := ids[i], ids[j]
				if start == end {
					return nil, ErrConvergence
				}
				key := [2]int{start, end}
				if start > end {
					key = [2]int{end, start}
				}
				id, reuse := edges[key]
				use := CylinderQuotientUse{UV: [2][2]*big.Rat{copyQuotientUV(p), copyQuotientUV(corners[j])}, PeriodOffset: new(big.Int)}
				if !reuse {
					id = len(out.edges)
					edges[key] = id
					out.edges = append(out.edges, CylinderQuotientEdge{Start: start, End: end, UV: use.UV})
				} else {
					old := out.edges[id]
					if old.Start != end || old.End != start {
						return nil, ErrConvergence
					}
					use.Reversed = true
					delta := new(big.Rat).Sub(use.UV[0][0], old.UV[1][0])
					if use.UV[0][1].Cmp(old.UV[1][1]) != 0 || use.UV[1][1].Cmp(old.UV[0][1]) != 0 || new(big.Rat).Sub(use.UV[1][0], old.UV[0][0]).Cmp(delta) != 0 {
						return nil, ErrConvergence
					}
					q := new(big.Rat).Quo(delta, period)
					if e := budget.check(q); e != nil {
						return nil, e
					}
					if !q.IsInt() {
						return nil, ErrConvergence
					}
					use.PeriodOffset.Set(q.Num())
				}
				use.Edge = id
				f.Uses[i] = use
			}
			out.faces = append(out.faces, f)
		}
	}
	if len(out.faces) > 15 || len(out.vertices) > 24 || len(out.edges) > 60 {
		return nil, ErrConvergence
	}
	cycles, e := validateCylinderQuotientIncidence(ctx, out, period, budget)
	if e != nil {
		return nil, e
	}
	out.boundaries = cycles
	out.operations = budget.operations
	out.complete = true
	return out, nil
}

func validateCylinderQuotientIncidence(ctx context.Context, r *WrappedCylinderUnionComplex, period *big.Rat, b *polynomialBudget) (out [2][]CylinderQuotientUse, err error) {
	fail := func() ([2][]CylinderQuotientUse, error) { return [2][]CylinderQuotientUse{}, ErrConvergence }
	if len(r.faces) == 0 || len(r.faces) > 15 || len(r.edges) > 60 || len(r.vertices) > 24 {
		return fail()
	}
	type incidence struct {
		face, start, end int
		use              CylinderQuotientUse
	}
	uses := make([][]incidence, len(r.edges))
	fans := make([]map[int]bool, len(r.vertices))
	adj := make([][]int, len(r.faces))
	for fi, f := range r.faces {
		seen := map[int]bool{}
		var starts, ends [4]int
		for i, u := range f.Uses {
			if u.Edge < 0 || u.Edge >= len(r.edges) || seen[u.Edge] || u.PeriodOffset == nil {
				return fail()
			}
			seen[u.Edge] = true
			e := r.edges[u.Edge]
			s, t := e.Start, e.End
			if u.Reversed {
				s, t = t, s
			}
			if s < 0 || t < 0 || s >= len(fans) || t >= len(fans) || s == t {
				return fail()
			}
			starts[i], ends[i] = s, t
			for end := 0; end < 2; end++ {
				j := end
				if u.Reversed {
					j = 1 - end
				}
				for axis := 0; axis < 2; axis++ {
					q := new(big.Rat).Set(e.UV[j][axis])
					if axis == 0 {
						q.Add(q, new(big.Rat).Mul(new(big.Rat).SetInt(u.PeriodOffset), period))
					}
					if er := b.check(q); er != nil {
						return out, er
					}
					if u.UV[end][axis] == nil || q.Cmp(u.UV[end][axis]) != 0 {
						return fail()
					}
				}
			}
			if fans[s] == nil {
				fans[s] = map[int]bool{}
			}
			fans[s][fi] = true
			uses[u.Edge] = append(uses[u.Edge], incidence{fi, s, t, u})
		}
		for i := 0; i < 4; i++ {
			if ends[i] != starts[(i+1)%4] {
				return fail()
			}
		}
	}
	boundary := map[int]incidence{}
	degree := make([]int, len(fans))
	for _, list := range uses {
		switch len(list) {
		case 1:
			u := list[0]
			if _, ok := boundary[u.start]; ok {
				return fail()
			}
			boundary[u.start] = u
			degree[u.start]++
			degree[u.end]++
		case 2:
			a, c := list[0], list[1]
			if a.face == c.face || a.start != c.end || a.end != c.start {
				return fail()
			}
			adj[a.face] = append(adj[a.face], c.face)
			adj[c.face] = append(adj[c.face], a.face)
		default:
			return fail()
		}
	}
	connected := func(nodes map[int]bool, links [][]int) bool {
		if len(nodes) == 0 {
			return false
		}
		var first int
		for n := range nodes {
			first = n
			break
		}
		seen := map[int]bool{first: true}
		queue := []int{first}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for _, v := range links[n] {
				if nodes[v] && !seen[v] {
					seen[v] = true
					queue = append(queue, v)
				}
			}
		}
		return len(seen) == len(nodes)
	}
	all := map[int]bool{}
	for i := range r.faces {
		all[i] = true
	}
	if !connected(all, adj) {
		return fail()
	}
	for vertex, nodes := range fans {
		if degree[vertex] != 0 && degree[vertex] != 2 {
			return fail()
		}
		links := make([][]int, len(r.faces))
		for _, list := range uses {
			if len(list) == 2 && (list[0].start == vertex || list[0].end == vertex) {
				a, c := list[0].face, list[1].face
				links[a] = append(links[a], c)
				links[c] = append(links[c], a)
			}
		}
		if !connected(nodes, links) {
			return fail()
		}
	}
	if len(r.vertices)-len(r.edges)+len(r.faces) != 0 {
		return fail()
	}
	count := 0
	for len(boundary) > 0 {
		if count == 2 {
			return fail()
		}
		start := int(^uint(0) >> 1)
		for v := range boundary {
			if v < start {
				start = v
			}
		}
		v := start
		var cycle []CylinderQuotientUse
		du, dv := new(big.Rat), new(big.Rat)
		for {
			u, ok := boundary[v]
			if !ok {
				return fail()
			}
			delete(boundary, v)
			cycle = append(cycle, u.use)
			du.Add(du, new(big.Rat).Sub(u.use.UV[1][0], u.use.UV[0][0]))
			dv.Add(dv, new(big.Rat).Sub(u.use.UV[1][1], u.use.UV[0][1]))
			if er := b.check(du); er != nil {
				return out, er
			}
			v = u.end
			if v == start {
				break
			}
		}
		winding := new(big.Rat).Quo(du, period)
		if er := b.check(winding); er != nil {
			return out, er
		}
		if len(cycle) < 4 || dv.Sign() != 0 || !winding.IsInt() || new(big.Int).Abs(winding.Num()).Cmp(big.NewInt(1)) != 0 {
			return fail()
		}
		slot := 0
		if winding.Sign() < 0 {
			slot = 1
		}
		if len(out[slot]) != 0 {
			return fail()
		}
		out[slot] = cycle
		count++
	}
	if count != 2 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return [2][]CylinderQuotientUse{}, e
	}
	return out, nil
}
