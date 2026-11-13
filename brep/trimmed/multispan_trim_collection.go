package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

// MultiSpanTrimCollectionOptions retains cumulative per-face Join budgets.
// Aggregate triangle and pre-weld vertex limits default to 1M, with maximum 1M.
// CheckDisplayEmbedding optionally proves the assembled display has no contact
// beyond indexed incidence; Collision bounds that independent exact BVH query.
type MultiSpanTrimCollectionOptions struct {
	Join                      MultiSpanTrimFaceJoinOptions
	MaxTriangles, MaxVertices int
	CheckDisplayEmbedding     bool
	Collision                 collision.Options
}

// CertifiedMultiSpanTrimCollection joins disjoint declared seams by exact
// normalized parameter identity. Seam arrays follow declaration order; processing
// uses canonical face/trim order. Original source faces and trim indices survive.
// DisplayEmbedded is true only after the optional complete exact contact check.
// Neither flag nor mesh distance proves embedding of the continuous surfaces.
type CertifiedMultiSpanTrimCollection struct {
	CertifiedQuadraticTrimCollection
	Seams            []CertifiedMultiSpanTrimSeam
	DisplayEmbedded  bool
	DisplayCollision collision.Report
}

// StitchMultiSpanTrimCollectionCertified accepts 2..16 faces and 1..32 disjoint
// single-span degree-one rational UV seams. Each face is initially meshed once; later seam
// insertion only splits exact collinear boundary edges. Thus earlier seam
// partitions and local IDs survive, and all recertification work accumulates.
// A trim may belong to only one seam; adjacent trims or shared endpoint vertices
// reject rather than inventing junction identity. Final opposite edge incidence
// and every vertex link are checked. Errors return no partial collection.
func StitchMultiSpanTrimCollectionCertified(ctx context.Context, inputs []QuadraticTrimStitchFace, seams []QuadraticTrimStitchSeam, o MultiSpanTrimCollectionOptions) (CertifiedMultiSpanTrimCollection, error) {
	empty := CertifiedMultiSpanTrimCollection{}
	if ctx == nil || len(inputs) < 2 || len(inputs) > 16 || len(seams) < 1 || len(seams) > 32 {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 1000000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 || o.MaxVertices < 1 || o.MaxVertices > 1000000 {
		return empty, geom.ErrInvalid
	}
	used := make([]map[int]bool, len(inputs))
	for i, in := range inputs {
		if in.Patch == nil {
			return empty, geom.ErrInvalid
		}
		used[i] = map[int]bool{}
	}
	for _, s := range seams {
		if s.FaceA < 0 || s.FaceB < 0 || s.FaceA >= len(inputs) || s.FaceB >= len(inputs) || s.FaceA == s.FaceB || s.TrimA < 0 || s.TrimB < 0 || s.TrimA >= len(inputs[s.FaceA].Outer) || s.TrimB >= len(inputs[s.FaceB].Outer) {
			return empty, geom.ErrInvalid
		}
		for _, m := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
			n := len(inputs[m[0]].Outer)
			if used[m[0]][m[1]] || used[m[0]][(m[1]+1)%n] || used[m[0]][(m[1]+n-1)%n] {
				return empty, geom.ErrInvalid
			}
			used[m[0]][m[1]] = true
		}
	}
	result := CertifiedMultiSpanTrimCollection{}
	result.Faces = make([]CertifiedCurvedQuadraticTrimMesh, len(inputs))
	result.OriginalTrim = make([][]int, len(inputs))
	result.Seams = make([]CertifiedMultiSpanTrimSeam, len(seams))
	result.SeamVertices = make([][]int, len(seams))
	result.SeamParameters = make([][]*big.Rat, len(seams))
	result.FillSquaredDistance = new(big.Rat)
	aggregate := func() error {
		tris, verts := 0, 0
		for _, f := range result.Faces {
			tris += len(f.Mesh.Triangles)
			verts += len(f.Mesh.Vertices)
			if tris > o.MaxTriangles || verts > o.MaxVertices {
				return geom.ErrConvergence
			}
		}
		return nil
	}
	for fi, in := range inputs {
		f, e := TessellateMultiSpanCurvedTrimRegionCertified(ctx, in.Patch, in.Outer, nil, o.Join.Face)
		if e != nil {
			return empty, e
		}
		result.Faces[fi] = f
		for j := range in.Outer {
			result.OriginalTrim[fi] = append(result.OriginalTrim[fi], j)
		}
		if e = aggregate(); e != nil {
			return empty, e
		}
	}
	type membership struct {
		seam      int
		parameter string
	}
	members := make([]map[int]membership, len(inputs))
	for fi := range members {
		members[fi] = map[int]membership{}
	}
	order := make([]int, len(seams))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := seams[order[i]], seams[order[j]]
		ka, kb := [4]int{a.FaceA, a.TrimA, a.FaceB, a.TrimB}, [4]int{b.FaceA, b.TrimA, b.FaceB, b.TrimB}
		for k := range ka {
			if ka[k] != kb[k] {
				return ka[k] < kb[k]
			}
		}
		return false
	})
	for _, si := range order {
		s := seams[si]
		a, b := inputs[s.FaceA], inputs[s.FaceB]
		prepared := [2]CertifiedCurvedQuadraticTrimMesh{result.Faces[s.FaceA], result.Faces[s.FaceB]}
		join, e := stitchMultiSpanTrimFaces(ctx, QuadraticTrimFaceJoinInput{Patch: a.Patch, Outer: a.Outer, SharedTrim: s.TrimA}, QuadraticTrimFaceJoinInput{Patch: b.Patch, Outer: b.Outer, SharedTrim: s.TrimB}, o.Join, &prepared)
		if e != nil {
			return empty, e
		}
		result.Faces[s.FaceA], result.Faces[s.FaceB] = join.Faces[0], join.Faces[1]
		result.Seams[si], result.SeamParameters[si] = join.Seam, join.SharedParameters
		for side, fi := range []int{s.FaceA, s.FaceB} {
			for j, id := range join.SharedLocalVertices[side] {
				if _, exists := members[fi][id]; exists {
					return empty, geom.ErrInvalid
				}
				members[fi][id] = membership{si, join.SharedParameters[j].RatString()}
			}
		}
		if e = aggregate(); e != nil {
			return empty, e
		}
	}
	common := map[membership]int{}
	for fi := range result.Faces {
		face := &result.Faces[fi]
		remap := make([]int, len(face.Mesh.Vertices))
		for id, p := range face.Mesh.Vertices {
			global := -1
			if m, ok := members[fi][id]; ok {
				if old, exists := common[m]; exists {
					global = old
					if result.Mesh.Vertices[old] != p {
						return empty, geom.ErrConvergence
					}
				} else {
					global = len(result.Mesh.Vertices)
					common[m] = global
					result.Mesh.Vertices = append(result.Mesh.Vertices, p)
				}
			} else {
				global = len(result.Mesh.Vertices)
				result.Mesh.Vertices = append(result.Mesh.Vertices, p)
			}
			remap[id] = global
		}
		for j, t := range face.Mesh.Triangles {
			result.Mesh.Triangles = append(result.Mesh.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
			result.OriginalFace = append(result.OriginalFace, fi)
			face.OriginalFace[j] = fi
		}
		if face.FillSquaredDistance.Cmp(result.FillSquaredDistance) > 0 {
			result.FillSquaredDistance.Set(face.FillSquaredDistance)
		}
	}
	for si, parameters := range result.SeamParameters {
		for _, q := range parameters {
			result.SeamVertices[si] = append(result.SeamVertices[si], common[membership{si, q.RatString()}])
		}
	}
	if e := result.Mesh.Validate(); e != nil {
		return empty, e
	}
	type edge [2]int
	key := func(a, b int) edge {
		if a > b {
			a, b = b, a
		}
		return edge{a, b}
	}
	counts, balances := map[edge]int{}, map[edge]int{}
	for _, t := range result.Mesh.Triangles {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		for j, a := range t {
			b := t[(j+1)%3]
			k := key(a, b)
			counts[k]++
			if a < b {
				balances[k]++
			} else {
				balances[k]--
			}
		}
	}
	for k, n := range counts {
		if n > 2 || (n == 2 && balances[k] != 0) {
			return empty, geom.ErrConvergence
		}
	}
	for _, ids := range result.SeamVertices {
		for i := 0; i+1 < len(ids); i++ {
			k := key(ids[i], ids[i+1])
			if counts[k] != 2 || balances[k] != 0 {
				return empty, geom.ErrConvergence
			}
		}
	}
	vertices := make([]int, len(result.Mesh.Vertices))
	for i := range vertices {
		vertices[i] = i
	}
	if e := validateQuadraticJunctionFans(result.Mesh, vertices); e != nil {
		return empty, e
	}
	if o.CheckDisplayEmbedding {
		report, e := collision.SelfIntersectionsBVHContext(ctx, result.Mesh, o.Collision)
		if e != nil {
			return empty, e
		}
		if !report.Complete || len(report.Contacts) > 0 {
			return empty, geom.ErrConvergence
		}
		result.DisplayCollision = report
		result.DisplayEmbedded = true
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
