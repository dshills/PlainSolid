package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// StoredMixedSeamNetwork gives shared boundary identities for every original
// edge. FaceVertices maps local face boundary IDs to global boundary points;
// interior display vertices are absent. EdgeVertices/Parameters use original
// stored-edge direction. CornerVertices maps original topology vertices to the
// shared network; their model coordinates are independently exact-checked.
// Face meshes remain separate. This is not a closed/embedded shell certificate:
// vertex fans and non-incidence contacts have not been proved.
type StoredMixedSeamNetwork struct {
	Faces            []CertifiedSolidMixedFaceMesh
	Points           []geom.Vec3
	ExactPoints      [][3]*big.Rat
	FaceVertices     []map[int]int
	EdgeVertices     [][]int
	EdgeParameters   [][]*big.Rat
	CornerVertices   map[int]int
	SquaredDistance  *big.Rat
	RefinementRounds int
}

// TessellateMixedBoundaryNetworkCertified accepts a bounded whole Solid with
// 2..16 faces and1..64 nonperiodic edges, exactly two opposite original uses per
// edge. It aligns ALL edge partitions together (at most2048 global samples),
// then proves every shared exact model point and original corner identity before
// assigning shared display IDs. Each face is finally recertified with those
// points and every original seam is audited for opposite triangle incidence.
// Limits apply per face per round (1..8 rounds), plus one final lift per face.
// No generic shell or source embedding acceptance is enabled by this method.
func (s Solid) TessellateMixedBoundaryNetworkCertified(ctx context.Context, options []SolidMixedFaceMeshOptions, maxRounds int) (StoredMixedSeamNetwork, error) {
	var empty StoredMixedSeamNetwork
	if ctx == nil || len(s.faces) < 2 || len(s.faces) > 16 || len(s.edges) < 1 || len(s.edges) > 64 || len(options) != len(s.faces) || maxRounds < 1 || maxRounds > 8 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	options = append([]SolidMixedFaceMeshOptions(nil), options...)
	type owner struct {
		face, loop, use int
		reverse         bool
	}
	owners := make([][]owner, len(s.edges))
	for fi, f := range s.faces {
		total := 0
		for li, loop := range append([][]Use{f.Outer}, f.Holes...) {
			if len(loop) > 256-total {
				return empty, geom.ErrInvalid
			}
			total += len(loop)
			for ui, u := range loop {
				if u.Edge < 0 || u.Edge >= len(owners) {
					return empty, geom.ErrInvalid
				}
				owners[u.Edge] = append(owners[u.Edge], owner{fi, li, ui, u.Reversed != f.Reversed})
			}
		}
	}
	for _, os := range owners {
		if len(os) != 2 || os[0].face == os[1].face || os[0].reverse == os[1].reverse {
			return empty, geom.ErrConvergence
		}
	}
	r := StoredMixedSeamNetwork{Faces: make([]CertifiedSolidMixedFaceMesh, len(s.faces)), FaceVertices: make([]map[int]int, len(s.faces)), EdgeVertices: make([][]int, len(s.edges)), EdgeParameters: make([][]*big.Rat, len(s.edges)), CornerVertices: map[int]int{}, SquaredDistance: new(big.Rat)}
	build := func() error {
		for fi := range s.faces {
			x, e := s.TessellateMixedFaceCertified(ctx, fi, options[fi])
			if e != nil {
				return e
			}
			r.Faces[fi] = x
		}
		return nil
	}
	if e := build(); e != nil {
		return empty, e
	}
	ordered := make([][2][]storedMixedParameterVertex, len(s.edges))
	for round := 0; ; round++ {
		allEqual := true
		total := 0
		plans := make([][]*big.Rat, len(s.edges))
		for edge, os := range owners {
			for k, o := range os {
				var e error
				ordered[edge][k], e = storedMixedEdgeSamples(r.Faces[o.face], edge)
				if e != nil {
					return empty, e
				}
			}
			a, b := ordered[edge][0], ordered[edge][1]
			equal := len(a) == len(b)
			if equal {
				for j, q := range a {
					if q.parameter.Cmp(b[j].parameter) != 0 {
						equal = false
						break
					}
				}
			}
			allEqual = allEqual && equal
			var union []*big.Rat
			for _, list := range ordered[edge] {
				for _, v := range list {
					union = append(union, new(big.Rat).Set(v.parameter))
				}
			}
			sort.Slice(union, func(i, j int) bool { return union[i].Cmp(union[j]) < 0 })
			for _, q := range union {
				if len(plans[edge]) == 0 || plans[edge][len(plans[edge])-1].Cmp(q) != 0 {
					plans[edge] = append(plans[edge], q)
				}
			}
			total += len(plans[edge])
			if total > 2048 {
				return empty, geom.ErrConvergence
			}
		}
		if allEqual {
			break
		}
		if round == maxRounds {
			return empty, geom.ErrConvergence
		}
		for fi, f := range s.faces {
			loops := append([][]Use{f.Outer}, f.Holes...)
			cuts := make([][][]*big.Rat, len(loops))
			for li, loop := range loops {
				cuts[li] = make([][]*big.Rat, len(loop))
				for ui, u := range loop {
					edge := s.edges[u.Edge]
					for _, q := range plans[u.Edge] {
						t := new(big.Rat).Quo(new(big.Rat).Sub(q, ratInset(edge.Range.Min)), new(big.Rat).Sub(ratInset(edge.Range.Max), ratInset(edge.Range.Min)))
						if u.Reversed {
							t.Sub(big.NewRat(1, 1), t)
						}
						cuts[li][ui] = append(cuts[li][ui], t)
					}
					sort.Slice(cuts[li][ui], func(i, j int) bool { return cuts[li][ui][i].Cmp(cuts[li][ui][j]) < 0 })
				}
			}
			options[fi].Face.Face.Cuts = cuts
		}
		if e := build(); e != nil {
			return empty, e
		}
		r.RefinementRounds++
	}
	for i := range r.FaceVertices {
		r.FaceVertices[i] = map[int]int{}
	}
	equalPoint := func(a, b [3]*big.Rat) bool {
		for k := 0; k < 3; k++ {
			if a[k].Cmp(b[k]) != 0 {
				return false
			}
		}
		return true
	}
	for edge, os := range owners {
		a, b := ordered[edge][0], ordered[edge][1]
		if len(a) < 2 {
			return empty, geom.ErrConvergence
		}
		model := s.edges[edge]
		if a[0].parameter.Cmp(ratInset(model.Range.Min)) != 0 || a[len(a)-1].parameter.Cmp(ratInset(model.Range.Max)) != 0 {
			return empty, geom.ErrConvergence
		}
		for j, q := range a {
			pa, pb := r.Faces[os[0].face].ExactPoints[q.vertex], r.Faces[os[1].face].ExactPoints[b[j].vertex]
			if !equalPoint(pa, pb) {
				return empty, geom.ErrConvergence
			}
			id := -1
			corner := -1
			if j == 0 {
				corner = model.Start
			}
			if j == len(a)-1 {
				corner = model.End
			}
			if corner >= 0 {
				if existing, ok := r.CornerVertices[corner]; ok {
					id = existing
					if !equalPoint(pa, r.ExactPoints[id]) {
						return empty, geom.ErrConvergence
					}
				}
			}
			if id < 0 {
				id = len(r.Points)
				point := r.Faces[os[0].face].Mesh.Vertices[q.vertex]
				r.Points = append(r.Points, point)
				copyPoint := [3]*big.Rat{}
				for k := range pa {
					copyPoint[k] = new(big.Rat).Set(pa[k])
				}
				r.ExactPoints = append(r.ExactPoints, copyPoint)
				if corner >= 0 {
					r.CornerVertices[corner] = id
				}
			}
			for k, o := range os {
				local := ordered[edge][k][j].vertex
				if old, ok := r.FaceVertices[o.face][local]; ok && old != id {
					return empty, geom.ErrConvergence
				}
				r.FaceVertices[o.face][local] = id
			}
			r.EdgeVertices[edge] = append(r.EdgeVertices[edge], id)
			r.EdgeParameters[edge] = append(r.EdgeParameters[edge], new(big.Rat).Set(q.parameter))
		}
	}
	// Distinct exact topology identities may not collapse to one display point.
	displays := map[geom.Vec3]int{}
	for i, p := range r.Points {
		if _, ok := displays[p]; ok {
			return empty, geom.ErrConvergence
		}
		displays[p] = i
	}
	for fi := range r.Faces {
		overrides := map[int]geom.Vec3{}
		for local, id := range r.FaceVertices[fi] {
			overrides[local] = r.Points[id]
		}
		fresh, e := s.recertifyMixedNetworkFace(ctx, fi, r.Faces[fi], options[fi], overrides)
		if e != nil {
			return empty, e
		}
		r.Faces[fi].CertifiedCurvedQuadraticTrimMesh = fresh
		if fresh.FillSquaredDistance.Cmp(r.SquaredDistance) > 0 {
			r.SquaredDistance.Set(fresh.FillSquaredDistance)
		}
	}
	counts := make([]map[[2]int]int, len(r.Faces))
	for fi, f := range r.Faces {
		counts[fi] = map[[2]int]int{}
		for _, tri := range f.Mesh.Triangles {
			for j, a := range tri {
				counts[fi][[2]int{a, tri[(j+1)%3]}]++
			}
		}
		for local, id := range r.FaceVertices[fi] {
			if f.Mesh.Vertices[local] != r.Points[id] {
				return empty, geom.ErrConvergence
			}
		}
	}
	for edge, os := range owners {
		for j := 1; j < len(ordered[edge][0]); j++ {
			signs := [2]int{}
			for k, o := range os {
				a, b := ordered[edge][k][j-1].vertex, ordered[edge][k][j].vertex
				forward, reverse := counts[o.face][[2]int{a, b}], counts[o.face][[2]int{b, a}]
				if forward+reverse != 1 {
					return empty, geom.ErrConvergence
				}
				signs[k] = forward - reverse
			}
			if signs[0]+signs[1] != 0 {
				return empty, geom.ErrConvergence
			}
		}
	}
	return r, nil
}

func (s Solid) recertifyMixedNetworkFace(ctx context.Context, fi int, old CertifiedSolidMixedFaceMesh, o SolidMixedFaceMeshOptions, display map[int]geom.Vec3) (CertifiedCurvedQuadraticTrimMesh, error) {
	var empty CertifiedCurvedQuadraticTrimMesh
	f := s.faces[fi]
	p, e := exactFaceSupportPatch(f, o.Face.Face.Boundary.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	work, e := newTubeLensSpanWork(MultiSpanTubeLensMeshOptions{Span: o.Face.Span, MaxSpanCells: o.Face.MaxSpanCells, MaxSpanPieces: o.Face.MaxSpanPieces})
	if e != nil {
		return empty, e
	}
	l2, _, e := work.lipschitz(ctx, p)
	if e != nil {
		return empty, e
	}
	m := o.Face.Face.Interior
	if m.MaxSubdivisions == 0 {
		m.MaxSubdivisions = 32
	}
	if m.MaxTriangles == 0 {
		m.MaxTriangles = 100000
	}
	if m.MaxTriangleChecks == 0 {
		m.MaxTriangleChecks = 100000
	}
	fresh, e := tessellateCertifiedTrimBoundaryDisplayMode(ctx, p, old.UVRegion.CertifiedAffineQuadraticTrimMesh, m, 1, &work, l2, true, display)
	if e != nil {
		return empty, e
	}
	for i := range fresh.OriginalFace {
		fresh.OriginalFace[i] = fi
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
	return fresh, nil
}
