package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"sort"
)

// CertifiedStoredMixedFacePair retains two face certificates and one shared
// original edge. SharedVertices and ModelParameters are ordered along the stored
// edge, not either face traversal. Only this seam is certified opposite/bit-equal;
// other seams, corner fans, global intersections and shell closure are unproved.
type CertifiedStoredMixedFacePair struct {
	Faces            [2]CertifiedSolidMixedFaceMesh
	Edge             int
	SharedVertices   [][2]int
	ModelParameters  []*big.Rat
	SquaredDistance  *big.Rat
	RefinementRounds int
}

// TessellateMixedFacePairCertified requires exactly one occurrence of edge in
// each distinct face, with opposite effective orientation. It repeatedly unions
// exact model parameters, converts them to original coedge-local cuts, and
// recertifies both fills/lifts until partitions match. maxRounds is1..8; each
// owner retains its independent per-proof budgets for at most maxRounds+1 face
// proofs. A final B lift recertifies A's chosen shared display points. At most
// 2048 shared samples are allowed. Any ambiguity or exhausted limit is atomic.
func (s Solid) TessellateMixedFacePairCertified(ctx context.Context, faces [2]int, edge int, options [2]SolidMixedFaceMeshOptions, maxRounds int) (CertifiedStoredMixedFacePair, error) {
	var empty CertifiedStoredMixedFacePair
	if ctx == nil || faces[0] == faces[1] || edge < 0 || edge >= len(s.edges) || maxRounds < 1 || maxRounds > 8 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	var selected [2]Use
	for owner, fi := range faces {
		if fi < 0 || fi >= len(s.faces) {
			return empty, geom.ErrInvalid
		}
		count := 0
		uses := 0
		if len(s.faces[fi].Holes) > 64 {
			return empty, geom.ErrInvalid
		}
		for _, loop := range append([][]Use{s.faces[fi].Outer}, s.faces[fi].Holes...) {
			if len(loop) > 256-uses {
				return empty, geom.ErrInvalid
			}
			uses += len(loop)
			for _, u := range loop {
				if u.Edge == edge {
					selected[owner] = u
					count++
				}
			}
		}
		if count != 1 {
			return empty, geom.ErrConvergence
		}
	}
	if (selected[0].Reversed != s.faces[faces[0]].Reversed) == (selected[1].Reversed != s.faces[faces[1]].Reversed) {
		return empty, geom.ErrConvergence
	}
	var result CertifiedStoredMixedFacePair
	result.Edge = edge
	build := func() error {
		for i := range faces {
			r, e := s.TessellateMixedFaceCertified(ctx, faces[i], options[i])
			if e != nil {
				return e
			}
			result.Faces[i] = r
		}
		return nil
	}
	if e := build(); e != nil {
		return empty, e
	}
	var ordered [2][]storedMixedParameterVertex
	for round := 0; ; round++ {
		for i := range ordered {
			var e error
			ordered[i], e = storedMixedEdgeSamples(result.Faces[i], edge)
			if e != nil {
				return empty, e
			}
		}
		equal := len(ordered[0]) == len(ordered[1])
		if equal {
			for i, a := range ordered[0] {
				if a.parameter.Cmp(ordered[1][i].parameter) != 0 {
					equal = false
					break
				}
			}
		}
		if equal {
			break
		}
		if round == maxRounds {
			return empty, geom.ErrConvergence
		}
		union := make([]*big.Rat, 0, len(ordered[0])+len(ordered[1]))
		for _, list := range ordered {
			for _, x := range list {
				union = append(union, new(big.Rat).Set(x.parameter))
			}
		}
		sort.Slice(union, func(i, j int) bool { return union[i].Cmp(union[j]) < 0 })
		unique := union[:0]
		for _, q := range union {
			if len(unique) == 0 || q.Cmp(unique[len(unique)-1]) != 0 {
				unique = append(unique, q)
			}
		}
		if len(unique) > 2048 {
			return empty, geom.ErrConvergence
		}
		for owner, fi := range faces {
			loops := append([][]Use{s.faces[fi].Outer}, s.faces[fi].Holes...)
			cuts := make([][][]*big.Rat, len(loops))
			old := options[owner].Face.Face.Cuts
			for li, loop := range loops {
				cuts[li] = make([][]*big.Rat, len(loop))
				for ui, u := range loop {
					if old != nil {
						cuts[li][ui] = append([]*big.Rat(nil), old[li][ui]...)
					}
					if u.Edge != edge {
						continue
					}
					cuts[li][ui] = nil
					for _, q := range unique {
						t := new(big.Rat).Quo(new(big.Rat).Sub(q, ratInset(s.edges[edge].Range.Min)), new(big.Rat).Sub(ratInset(s.edges[edge].Range.Max), ratInset(s.edges[edge].Range.Min)))
						if u.Reversed {
							t.Sub(big.NewRat(1, 1), t)
						}
						cuts[li][ui] = append(cuts[li][ui], t)
					}
					sort.Slice(cuts[li][ui], func(i, j int) bool { return cuts[li][ui][i].Cmp(cuts[li][ui][j]) < 0 })
				}
			}
			options[owner].Face.Face.Cuts = cuts
		}
		if e := build(); e != nil {
			return empty, e
		}
		result.RefinementRounds++
	}
	if len(ordered[0]) < 2 || len(ordered[0]) > 2048 {
		return empty, geom.ErrConvergence
	}
	overrides := map[int]geom.Vec3{}
	for j, a := range ordered[0] {
		b := ordered[1][j]
		for axis := 0; axis < 3; axis++ {
			if result.Faces[0].ExactPoints[a.vertex][axis].Cmp(result.Faces[1].ExactPoints[b.vertex][axis]) != 0 {
				return empty, geom.ErrConvergence
			}
		}
		overrides[b.vertex] = result.Faces[0].Mesh.Vertices[a.vertex]
		result.SharedVertices = append(result.SharedVertices, [2]int{a.vertex, b.vertex})
		result.ModelParameters = append(result.ModelParameters, new(big.Rat).Set(a.parameter))
	}
	f := s.faces[faces[1]]
	base, mapping := parametric.SurfaceMapping(f.Surface)
	bs, ok := base.(*geom.BSplineSurface)
	if !ok {
		return empty, geom.ErrConvergence
	}
	p, e := geom.NewSurfacePatch(bs, options[1].Face.Face.Boundary.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	p, e = p.Transformed(mapping)
	if e != nil {
		return empty, e
	}
	work, e := newTubeLensSpanWork(MultiSpanTubeLensMeshOptions{Span: options[1].Face.Span, MaxSpanCells: options[1].Face.MaxSpanCells, MaxSpanPieces: options[1].Face.MaxSpanPieces})
	if e != nil {
		return empty, e
	}
	l2, _, e := work.lipschitz(ctx, p)
	if e != nil {
		return empty, e
	}
	interior := options[1].Face.Face.Interior
	if interior.MaxSubdivisions == 0 {
		interior.MaxSubdivisions = 32
	}
	if interior.MaxTriangles == 0 {
		interior.MaxTriangles = 100000
	}
	if interior.MaxTriangleChecks == 0 {
		interior.MaxTriangleChecks = 100000
	}
	fresh, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, p, result.Faces[1].UVRegion.CertifiedAffineQuadraticTrimMesh, interior, 1, &work, l2, true, overrides)
	if e != nil {
		return empty, e
	}
	for i := range fresh.OriginalFace {
		fresh.OriginalFace[i] = faces[1]
	}
	if f.Reversed {
		fresh.BoundaryLoops = append([][]int(nil), fresh.BoundaryLoops...)
		for i := range fresh.Mesh.Triangles {
			fresh.Mesh.Triangles[i][1], fresh.Mesh.Triangles[i][2] = fresh.Mesh.Triangles[i][2], fresh.Mesh.Triangles[i][1]
		}
		for i, loop := range fresh.BoundaryLoops {
			x := append([]int(nil), loop...)
			for j, k := 0, len(x)-1; j < k; j, k = j+1, k-1 {
				x[j], x[k] = x[k], x[j]
			}
			fresh.BoundaryLoops[i] = x
		}
	}
	result.Faces[1].CertifiedCurvedQuadraticTrimMesh = fresh
	counts := func(tris [][3]int) map[[2]int]int {
		r := map[[2]int]int{}
		for _, t := range tris {
			for i, a := range t {
				r[[2]int{a, t[(i+1)%3]}]++
			}
		}
		return r
	}
	aEdges, bEdges := counts(result.Faces[0].Mesh.Triangles), counts(result.Faces[1].Mesh.Triangles)
	for i, pair := range result.SharedVertices {
		if result.Faces[0].Mesh.Vertices[pair[0]] != result.Faces[1].Mesh.Vertices[pair[1]] {
			return empty, geom.ErrConvergence
		}
		if i == 0 {
			continue
		}
		prev := result.SharedVertices[i-1]
		af, ar := aEdges[[2]int{prev[0], pair[0]}], aEdges[[2]int{pair[0], prev[0]}]
		bf, br := bEdges[[2]int{prev[1], pair[1]}], bEdges[[2]int{pair[1], prev[1]}]
		if af+ar != 1 || bf+br != 1 || af != br || ar != bf {
			return empty, geom.ErrConvergence
		}
	}
	result.SquaredDistance = new(big.Rat).Set(result.Faces[0].FillSquaredDistance)
	if result.Faces[1].FillSquaredDistance.Cmp(result.SquaredDistance) > 0 {
		result.SquaredDistance.Set(result.Faces[1].FillSquaredDistance)
	}
	return result, nil
}

type storedMixedParameterVertex struct {
	parameter *big.Rat
	vertex    int
}

func storedMixedEdgeSamples(r CertifiedSolidMixedFaceMesh, edge int) ([]storedMixedParameterVertex, error) {
	var out []storedMixedParameterVertex
	count := 0
	for _, c := range r.Coedges {
		if c.Edge != edge {
			continue
		}
		count++
		for j, q := range c.ModelParameters {
			out = append(out, storedMixedParameterVertex{q, c.Vertices[j]})
		}
	}
	if count != 1 || len(out) > 2048 {
		return nil, geom.ErrConvergence
	}
	sort.Slice(out, func(i, j int) bool { return out[i].parameter.Cmp(out[j].parameter) < 0 })
	for i := 1; i < len(out); i++ {
		if out[i-1].parameter.Cmp(out[i].parameter) >= 0 {
			return nil, geom.ErrConvergence
		}
	}
	return out, nil
}
