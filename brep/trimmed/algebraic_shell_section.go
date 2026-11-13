package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

type AlgebraicShellSectionOptions struct {
	Shells                                    SplineShellBooleanOptions
	Trace                                     intersect.SplinePairTraceOptions
	Endpoints                                 geom.TubeAlgebraicGraphEndpointOptions
	MaxFacePairs, MaxBranches, MaxComparisons int
	// Seeds optionally provides independently rechecked proposals keyed
	// by original face IDs. Unlisted candidate pairs are still discovered
	// or excluded; callers cannot omit an interacting pair.
	Seeds []AlgebraicShellSectionSeed
}

type AlgebraicShellSectionSeed struct {
	Faces [2]int
	Tubes intersect.SplineSeedReport
}

type AlgebraicShellSectionBranch struct {
	Faces          [2]int
	Segments       []intersect.TubeSegment
	Ends           [2]AlgebraicSplineTerminal
	EndEdges       [2]int
	EdgeParameters [2]*geom.ExactAlgebraicRoot // normalized original model-edge traversal
	Closed         bool
}

type AlgebraicShellSectionWireUse struct {
	Branch   int
	Reversed bool
}

type AlgebraicShellSectionResult struct {
	Pairs    [][2]int
	Sections []AlgebraicSplinePairSection
	Branches []AlgebraicShellSectionBranch
	Wires    [][]AlgebraicShellSectionWireUse
	Complete bool
	source   *algebraicShellSectionSource
}

type algebraicShellSectionSource struct {
	a, b     Solid
	pairs    [][2]int
	sections []AlgebraicSplinePairSection
	branches []AlgebraicShellSectionBranch
	wires    [][]AlgebraicShellSectionWireUse
}

// SectionSplineShellsAlgebraic isolates a section graph of two independently
// embedded rectangular spline shells. Every retained pair has independent
// coverage. Exact cross-chart model point equality joins open terminals;
// original edge identity is required on the terminal owner. It does not
// classify material, split either shell, or resolve simultaneous edge events.
func SectionSplineShellsAlgebraic(ctx context.Context, a, b Solid, o AlgebraicShellSectionOptions) (AlgebraicShellSectionResult, error) {
	var out AlgebraicShellSectionResult
	if ctx == nil || o.MaxFacePairs < 0 || o.MaxFacePairs > 16384 || o.MaxBranches < 0 || o.MaxBranches > 4096 || o.MaxComparisons < 0 || o.MaxComparisons > 262144 {
		return out, geom.ErrInvalid
	}
	if o.MaxFacePairs == 0 {
		o.MaxFacePairs = 256
	}
	if o.MaxBranches == 0 {
		o.MaxBranches = 128
	}
	if o.MaxComparisons == 0 {
		o.MaxComparisons = 32768
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	provided := map[[2]int]*intersect.SplineSeedReport{}
	if len(o.Seeds) > o.MaxFacePairs {
		return out, geom.ErrConvergence
	}
	for i := range o.Seeds {
		s := &o.Seeds[i]
		if s.Faces[0] < 0 || s.Faces[0] >= len(a.faces) || s.Faces[1] < 0 || s.Faces[1] >= len(b.faces) ||
			provided[s.Faces] != nil || len(s.Tubes.Attempts) == 0 {
			return out, geom.ErrInvalid
		}
		provided[s.Faces] = &s.Tubes
	}
	var faces [2][]splineEmbeddingFace
	for i, s := range [2]Solid{a, b} {
		if len(s.faces) == 0 {
			continue
		}
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &faces[i]); e != nil {
			return out, e
		}
	}
	if len(faces[0]) > 0 && len(faces[1]) > o.MaxFacePairs/len(faces[0]) {
		return out, geom.ErrConvergence
	}
	for i, p := range faces[0] {
		for j, q := range faces[1] {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			ids := [2]int{i, j}
			seed := provided[ids]
			if seed == nil {
				e := proveSplinePatchPairDisjoint(ctx, p.patch, q.patch, o.Shells.Separation)
				if e == nil {
					continue
				}
				if !isUnresolvedOpenSpline(e) {
					return out, e
				}
			}
			x, e := intersectSplineFace(a.faces[i], p.patch.Domain())
			if e != nil {
				return out, e
			}
			y, e := intersectSplineFace(b.faces[j], q.patch.Domain())
			if e != nil {
				return out, e
			}
			r, e := SectionSplinePairAlgebraic(ctx, x, y, seed, o.Trace, o.Endpoints)
			if e != nil && seed == nil && o.Trace.PlaneFallback.Enabled && isUnresolvedOpenSpline(e) {
				d := o.Trace.PlaneFallback
				d.Plane.Arcs.ResolveSubdivisionCorners = true
				p, e2 := intersect.DiscoverSplinePlanePairSections(ctx, x, y, o.Trace.Seeds.Patches, d)
				if e2 == nil {
					r, e = SectionSplinePairAlgebraic(ctx, x, y, &p.Seeds, o.Trace, o.Endpoints)
				}
				if e2 != nil && !isUnresolvedOpenSpline(e2) {
					return out, e2
				}
			}
			out.Pairs = append(out.Pairs, ids)
			out.Sections = append(out.Sections, r)
			if e != nil {
				return out, e
			}
			if !r.Complete {
				return out, geom.ErrConvergence
			}
			open := 0
			for k, h := range r.Trace.BranchGraph.Branches {
				if len(out.Branches) >= o.MaxBranches {
					return out, geom.ErrConvergence
				}
				z := AlgebraicShellSectionBranch{Faces: ids, Segments: r.Trace.Curves[k], Closed: h.Closed}
				if !h.Closed {
					z.Ends = r.Open[open].Ends
					open++
					for side, t := range z.Ends {
						owner := t.Operand
						edge, root, e := algebraicShellEdgeTerminal(ctx, [2]Solid{a, b}[owner], ids[owner], faces[owner][ids[owner]], t, o.Endpoints)
						if e != nil {
							return out, e
						}
						z.EndEdges[side], z.EdgeParameters[side] = edge, root
					}
				}
				out.Branches = append(out.Branches, z)
			}
		}
	}
	var e error
	out.Wires, e = joinAlgebraicShellSectionBranches(ctx, out.Branches, faces, o)
	if e != nil {
		return out, e
	}
	out.Complete = true
	source := &algebraicShellSectionSource{a: a, b: b, pairs: append([][2]int(nil), out.Pairs...)}
	for _, b := range out.Branches {
		b.Segments = append([]intersect.TubeSegment(nil), b.Segments...)
		source.branches = append(source.branches, b)
	}
	for _, w := range out.Wires {
		source.wires = append(source.wires, append([]AlgebraicShellSectionWireUse(nil), w...))
	}
	for _, p := range out.Sections {
		x := AlgebraicSplinePairSection{recipes: append([]nativeTubeRecipe(nil), p.recipes...)}
		x.Open = append([]AlgebraicSplineBranchEvidence(nil), p.Open...)
		x.Trace.BranchGraph.Branches = append([]intersect.SplineTubeBranch(nil), p.Trace.BranchGraph.Branches...)
		x.Trace.Seeds.Attempts = append([]intersect.SplineSeedCell(nil), p.Trace.Seeds.Attempts...)
		for i := range x.Trace.Seeds.Attempts {
			a := &x.Trace.Seeds.Attempts[i]
			if a.Tube != nil {
				t := *a.Tube
				a.Tube = &t
			}
			if a.BoundaryAnchor != nil {
				q := *a.BoundaryAnchor
				a.BoundaryAnchor = &q
			}
		}
		source.sections = append(source.sections, x)
	}
	out.source = source
	return out, nil
}

func algebraicShellEdgeTerminal(ctx context.Context, s Solid, face int, chart splineEmbeddingFace, t AlgebraicSplineTerminal,
	o geom.TubeAlgebraicGraphEndpointOptions) (int, *geom.ExactAlgebraicRoot, error) {
	if t.Side > geom.SurfaceEdgeVMax || t.Operand < 0 || t.Operand > 1 {
		return 0, nil, geom.ErrInvalid
	}
	edge := chart.edge[t.Side]
	axis := (int(t.Side) / 2) ^ 1
	root := t.Certificate.Parameters[2*t.Operand+axis]
	if root == nil {
		return 0, nil, geom.ErrConvergence
	}
	for _, u := range s.faces[face].Outer {
		if u.Edge != edge {
			continue
		}
		p, q := [2]float64{u.UVStart.X, u.UVStart.Y}, [2]float64{u.UVEnd.X, u.UVEnd.Y}
		if u.Reversed {
			p, q = q, p
		}
		a, b := ratInset(p[axis]), ratInset(q[axis])
		width := new(big.Rat).Sub(b, a)
		if width.Sign() == 0 {
			return 0, nil, geom.ErrConvergence
		}
		inv := new(big.Rat).Inv(width)
		offset := new(big.Rat).Neg(new(big.Rat).Mul(a, inv))
		x, _, e := root.AffineMap(ctx, offset, inv, geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations,
			MaxSubdivisions: o.MaxSubdivisions, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return 0, nil, e
		}
		return edge, x, nil
	}
	return 0, nil, geom.ErrConvergence
}

func joinAlgebraicShellSectionBranches(ctx context.Context, branches []AlgebraicShellSectionBranch, faces [2][]splineEmbeddingFace,
	o AlgebraicShellSectionOptions) ([][]AlgebraicShellSectionWireUse, error) {
	var out [][]AlgebraicShellSectionWireUse
	type endpoint struct {
		branch, end, operand, edge int
		event                      *geom.ExactComposedCurveEvent
		parameter                  *geom.ExactAlgebraicRoot
	}
	var terminals []endpoint
	for i, b := range branches {
		if b.Closed {
			out = append(out, []AlgebraicShellSectionWireUse{{Branch: i}})
			continue
		}
		for j, t := range b.Ends {
			k := t.Operand
			if k < 0 || k > 1 || b.Faces[k] < 0 || b.Faces[k] >= len(faces[k]) || t.Side > geom.SurfaceEdgeVMax || !t.Certificate.Proven || t.Certificate.ModelEvent == nil {
				return nil, geom.ErrConvergence
			}
			terminals = append(terminals, endpoint{i, j, k, faces[k][b.Faces[k]].edge[t.Side], t.Certificate.ModelEvent, b.EdgeParameters[j]})
		}
	}
	match := make([]int, len(terminals))
	for i := range match {
		match[i] = -1
	}
	comparisons := 0
	work := o.Endpoints.MaxOperations
	if work == 0 {
		work = 8000000
	}
	for i, a := range terminals {
		for j := i + 1; j < len(terminals); j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			b := terminals[j]
			if a.operand != b.operand || a.edge != b.edge || a.branch == b.branch {
				continue
			}
			if branches[a.branch].Faces[a.operand] == branches[b.branch].Faces[b.operand] ||
				branches[a.branch].Faces[a.operand^1] != branches[b.branch].Faces[b.operand^1] {
				continue
			}
			if comparisons >= o.MaxComparisons || work < 1 {
				return nil, geom.ErrConvergence
			}
			comparisons++
			if a.parameter != nil && b.parameter != nil {
				q, e := a.parameter.Compare(ctx, b.parameter, geom.ExactCurvePlaneEventOptions{MaxOperations: work,
					MaxCoefficientBits: o.Endpoints.MaxCoefficientBits, MaxSubdivisions: o.Endpoints.MaxSubdivisions})
				work -= q.Operations
				if e != nil {
					return nil, e
				}
				if !q.Proven {
					return nil, geom.ErrConvergence
				}
				if q.Order != 0 {
					continue
				}
				if work < 1 {
					return nil, geom.ErrConvergence
				}
			}
			equal, n, e := a.event.CompareModelPoint(ctx, b.event, geom.ExactCurvePlaneEventOptions{MaxOperations: work,
				MaxCoefficientBits: o.Endpoints.MaxCoefficientBits, MaxSubdivisions: o.Endpoints.MaxSubdivisions})
			work -= n
			if e != nil {
				return nil, e
			}
			if !equal {
				continue
			}
			if match[i] >= 0 || match[j] >= 0 {
				return nil, geom.ErrConvergence
			}
			match[i], match[j] = j, i
		}
	}
	for _, i := range match {
		if i < 0 {
			return nil, geom.ErrConvergence
		}
	}
	seen := map[int]bool{}
	for start := range terminals {
		if seen[terminals[start].branch] {
			continue
		}
		var wire []AlgebraicShellSectionWireUse
		p := start
		for {
			if seen[terminals[p].branch] {
				if p != start {
					return nil, geom.ErrConvergence
				}
				break
			}
			x := terminals[p]
			seen[x.branch] = true
			wire = append(wire, AlgebraicShellSectionWireUse{Branch: x.branch, Reversed: x.end == 1})
			p = match[p^1]
		}
		out = append(out, wire)
	}
	return out, nil
}
