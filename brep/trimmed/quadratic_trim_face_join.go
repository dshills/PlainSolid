package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// QuadraticTrimFaceJoinInput describes one simple CCW outer loop. SharedTrim
// indexes its common edge; its direction must oppose the other face's edge.
// Holes and multiple shared edges are deliberately outside this join API.
type QuadraticTrimFaceJoinInput struct {
	Patch      *geom.SurfacePatch
	Outer      []SplineUV
	SharedTrim int
}

// QuadraticTrimFaceJoinOptions retains per-face/per-pass proof budgets.
// MaxSharedSegments defaults to64 (power of two, maximum256). MaxPasses defaults
// to4 (maximum8); each retry doubles the common face subdivision requirement.
type QuadraticTrimFaceJoinOptions struct {
	Face                         CurvedQuadraticTrimMeshOptions
	MaxSharedSegments, MaxPasses int
}

// CertifiedQuadraticTrimFaceJoin is an open two-face mesh with one shared
// topological edge chain. OriginalFace is 0 or1 per triangle. SharedParameters
// follows face A's original trim direction; SharedVertices are common mesh IDs.
// OriginalTrim maps expanded source trim indices back to each original loop.
// In Faces, shared ExactUV stays on each chart polygon chord while ExactPoints
// is replaced by the true common curve point; the recomputed residual includes
// this deliberate difference between the UV correspondence and model vertex.
// FillSquaredDistance certifies both directed distances between the union of
// the two source surface images and the stitched display mesh. It does not
// certify global embedding, other contacts, or a closed shell.
type CertifiedQuadraticTrimFaceJoin struct {
	Mesh                mesh.Mesh
	OriginalFace        []int
	Faces               [2]CertifiedCurvedQuadraticTrimMesh
	OriginalTrim        [2][]int
	SharedVertices      []int
	SharedParameters    []*big.Rat
	FillSquaredDistance *big.Rat
	SharedTrim          [2]int
}

// StitchQuadraticTrimFacesCertified proves exact continuous common-parameter
// seam identity, synchronizes a dyadic source partition before triangulation,
// and uses a common face refinement level. Every boundary sample on the seam
// is replaced by the exact common model curve. All triangle residuals are then
// recomputed; no pre-replacement certificate is reused. Shared vertices weld
// by exact normalized parameter identity, never geometric proximity.
func StitchQuadraticTrimFacesCertified(ctx context.Context, a, b QuadraticTrimFaceJoinInput, o QuadraticTrimFaceJoinOptions) (CertifiedQuadraticTrimFaceJoin, error) {
	empty := CertifiedQuadraticTrimFaceJoin{}
	if ctx == nil || a.Patch == nil || b.Patch == nil || a.SharedTrim < 0 || a.SharedTrim >= len(a.Outer) || b.SharedTrim < 0 || b.SharedTrim >= len(b.Outer) {
		return empty, geom.ErrInvalid
	}
	if o.MaxSharedSegments == 0 {
		o.MaxSharedSegments = 64
	}
	if o.MaxPasses == 0 {
		o.MaxPasses = 4
	}
	if o.Face.MaxSubdivisions == 0 {
		o.Face.MaxSubdivisions = 32
	}
	if o.MaxSharedSegments < 1 || o.MaxSharedSegments > 256 || o.MaxSharedSegments&(o.MaxSharedSegments-1) != 0 || o.MaxPasses < 1 || o.MaxPasses > 8 {
		return empty, geom.ErrInvalid
	}
	inputs := [2]QuadraticTrimFaceJoinInput{a, b}
	shared := [2]SplineUV{a.Outer[a.SharedTrim], b.Outer[b.SharedTrim]}
	canonicalB := shared[1]
	canonicalB.Reversed = !canonicalB.Reversed
	_, e := CertifyQuadraticTrimSeam(ctx, a.Patch, b.Patch, shared[0], canonicalB, QuadraticTrimSeamOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: o.Face.Deflection, Evaluation: o.Face.Evaluation}})
	if e != nil {
		return empty, e
	}
	var uvPatches [2]*geom.SurfacePatch
	var tolerances [2]float64
	for i, input := range inputs {
		_, l1, ok := quadraticPatchLipschitz(input.Patch)
		if !ok || l1.Sign() == 0 {
			return empty, geom.ErrConvergence
		}
		q := new(big.Rat).Quo(ratInset(o.Face.Deflection), new(big.Rat).Mul(big.NewRat(8, 1), l1))
		tolerances[i], _ = q.Float64()
		if !finite(tolerances[i]) || tolerances[i] <= 0 {
			return empty, geom.ErrConvergence
		}
		d := input.Patch.Domain()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}, []float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
		if e != nil {
			return empty, e
		}
		uvPatches[i], e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			return empty, e
		}
	}
	part := func(trim SplineUV, j, n int) (SplineUV, error) {
		t := func(k int) (float64, error) {
			q := big.NewRat(int64(k), int64(n))
			if trim.Reversed {
				q.Sub(big.NewRat(1, 1), q)
			}
			q.Mul(q, new(big.Rat).Sub(ratInset(trim.Range.Max), ratInset(trim.Range.Min)))
			q.Add(q, ratInset(trim.Range.Min))
			f, _ := q.Float64()
			if !finite(f) || ratInset(f).Cmp(q) != 0 {
				return 0, geom.ErrConvergence
			}
			return f, nil
		}
		lo, e := t(j)
		if e != nil {
			return SplineUV{}, e
		}
		hi, e := t(j + 1)
		if e != nil {
			return SplineUV{}, e
		}
		trim.Range = UVInterval{Min: lo, Max: hi}
		if lo > hi {
			trim.Range.Min, trim.Range.Max = hi, lo
		}
		return trim, nil
	}
	pieces := 0
	for n := 1; n <= o.MaxSharedSegments; n *= 2 {
		accepted := true
		for i := range inputs {
			for j := 0; j < n; j++ {
				trim, e := part(shared[i], j, n)
				if e != nil {
					return empty, e
				}
				_, e = TessellateQuadraticSplineTrimCertified(ctx, uvPatches[i], trim, QuadraticTrimTessellationOptions{Deflection: tolerances[i], MaxSegments: 1, Evaluation: o.Face.Evaluation})
				if e != nil {
					if ctx.Err() != nil {
						return empty, ctx.Err()
					}
					accepted = false
					break
				}
			}
		}
		if accepted {
			pieces = n
			break
		}
	}
	if pieces == 0 {
		return empty, geom.ErrConvergence
	}
	var expanded [2][]SplineUV
	var ancestry [2][]int
	for i, input := range inputs {
		for j, trim := range input.Outer {
			if j != input.SharedTrim {
				expanded[i] = append(expanded[i], trim)
				ancestry[i] = append(ancestry[i], j)
			} else {
				for k := 0; k < pieces; k++ {
					p, e := part(trim, k, pieces)
					if e != nil {
						return empty, e
					}
					expanded[i] = append(expanded[i], p)
					ancestry[i] = append(ancestry[i], j)
				}
			}
		}
	}
	h, e := quadraticTrimSourceControls(shared[0])
	if e != nil {
		return empty, e
	}
	exactSeam := func(q *big.Rat) ([3]*big.Rat, geom.Vec3, error) {
		one := new(big.Rat).Sub(big.NewRat(1, 1), q)
		basis := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, q)), new(big.Rat).Mul(q, q)}
		var v [3]*big.Rat
		for k := range v {
			v[k] = new(big.Rat)
			for j, w := range basis {
				v[k].Add(v[k], new(big.Rat).Mul(w, h[j][k]))
			}
		}
		jet, e := a.Patch.EvaluateRational(ctx, new(big.Rat).Quo(v[0], v[2]), new(big.Rat).Quo(v[1], v[2]), o.Face.Evaluation)
		if e != nil {
			return [3]*big.Rat{}, geom.Vec3{}, e
		}
		x, _ := jet.Point[0].Float64()
		y, _ := jet.Point[1].Float64()
		z, _ := jet.Point[2].Float64()
		p := geom.Vec3{X: x, Y: y, Z: z}
		if !p.IsFinite() {
			return [3]*big.Rat{}, geom.Vec3{}, geom.ErrConvergence
		}
		return jet.Point, p, nil
	}
	minimum := 1
	target := ratInset(o.Face.Deflection)
	target.Mul(target, target)
	for pass := 0; pass < o.MaxPasses && minimum <= o.Face.MaxSubdivisions; pass++ {
		result := CertifiedQuadraticTrimFaceJoin{OriginalTrim: ancestry, SharedTrim: [2]int{a.SharedTrim, b.SharedTrim}, FillSquaredDistance: new(big.Rat)}
		level := minimum
		for i, input := range inputs {
			result.Faces[i], e = tessellateCurvedQuadraticTrimRegionCertified(ctx, input.Patch, expanded[i], nil, o.Face, minimum)
			if e != nil {
				return empty, e
			}
			if result.Faces[i].Subdivisions > level {
				level = result.Faces[i].Subdivisions
			}
		}
		for i, input := range inputs {
			if result.Faces[i].Subdivisions != level {
				spent := result.Faces[i].TriangleChecks
				remaining := o.Face
				if remaining.MaxTriangleChecks == 0 {
					remaining.MaxTriangleChecks = 100000
				}
				remaining.MaxTriangleChecks -= spent
				if remaining.MaxTriangleChecks < 1 {
					return empty, geom.ErrConvergence
				}
				result.Faces[i], e = tessellateCurvedQuadraticTrimRegionCertified(ctx, input.Patch, expanded[i], nil, remaining, level)
				if e != nil {
					return empty, e
				}
				result.Faces[i].TriangleChecks += spent
			}
		}
		localKeys := [2]map[int]*big.Rat{make(map[int]*big.Rat), make(map[int]*big.Rat)}
		good := true
		for fi, input := range inputs {
			face := &result.Faces[fi]
			boundary := face.Boundary.Region.Loops[0].Boundary
			normalized := func(parameter float64) *big.Rat {
				q := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(parameter), ratInset(shared[fi].Range.Min)), new(big.Rat).Sub(ratInset(shared[fi].Range.Max), ratInset(shared[fi].Range.Min)))
				if shared[fi].Reversed {
					q.Sub(big.NewRat(1, 1), q)
				}
				if fi == 1 {
					q.Sub(big.NewRat(1, 1), q)
				}
				return q
			}
			if len(face.BoundaryLoops[0]) != len(boundary.Segments)*level {
				return empty, geom.ErrConvergence
			}
			for j, s := range boundary.Segments {
				if ancestry[fi][s.Source] != input.SharedTrim {
					continue
				}
				q0, q1 := normalized(s.Start), normalized(s.End)
				for k := 0; k <= level; k++ {
					q := new(big.Rat).Add(q0, new(big.Rat).Mul(new(big.Rat).Sub(q1, q0), big.NewRat(int64(k), int64(level))))
					id := face.BoundaryLoops[0][(j*level+k)%len(face.BoundaryLoops[0])]
					if old, exists := localKeys[fi][id]; exists && old.Cmp(q) != 0 {
						return empty, geom.ErrConvergence
					}
					localKeys[fi][id] = q
					exact, p, e := exactSeam(q)
					if e != nil {
						return empty, e
					}
					face.Mesh.Vertices[id] = p
					face.ExactPoints[id] = exact
				}
			}
			limit := o.Face.MaxTriangleChecks
			if limit == 0 {
				limit = 100000
			}
			if len(face.Mesh.Triangles) > limit-face.TriangleChecks {
				return empty, geom.ErrConvergence
			}
			maximum := new(big.Rat)
			for j, t := range face.Mesh.Triangles {
				face.TriangleChecks++
				uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
				model := [3]geom.Vec3{face.Mesh.Vertices[t[0]], face.Mesh.Vertices[t[1]], face.Mesh.Vertices[t[2]]}
				bound, ok := rationalBiquadraticExactCorrespondenceBound(input.Patch, uv, model)
				if !ok {
					return empty, geom.ErrConvergence
				}
				face.SquaredErrors[j] = bound
				if bound.Cmp(maximum) > 0 {
					maximum = bound
				}
			}
			face.SurfaceSquaredDistance = maximum
			face.FillSquaredDistance = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(face.TransportedTrimSquaredDistance, maximum))
			if face.FillSquaredDistance.Cmp(target) > 0 {
				good = false
			}
			if face.FillSquaredDistance.Cmp(result.FillSquaredDistance) > 0 {
				result.FillSquaredDistance = new(big.Rat).Set(face.FillSquaredDistance)
			}
		}
		if !good {
			minimum = 2 * level
			continue
		}
		common := map[string]int{}
		seen := [2]map[string]bool{make(map[string]bool), make(map[string]bool)}
		parameters := map[string]*big.Rat{}
		for fi, face := range result.Faces {
			remap := make([]int, len(face.Mesh.Vertices))
			for j, p := range face.Mesh.Vertices {
				id := -1
				if q, shared := localKeys[fi][j]; shared {
					key := q.RatString()
					seen[fi][key] = true
					parameters[key] = q
					if old, ok := common[key]; ok {
						id = old
						if result.Mesh.Vertices[id] != p {
							return empty, geom.ErrConvergence
						}
					} else {
						id = len(result.Mesh.Vertices)
						common[key] = id
						result.Mesh.Vertices = append(result.Mesh.Vertices, p)
					}
				} else {
					id = len(result.Mesh.Vertices)
					result.Mesh.Vertices = append(result.Mesh.Vertices, p)
				}
				remap[j] = id
			}
			for _, t := range face.Mesh.Triangles {
				result.Mesh.Triangles = append(result.Mesh.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
				result.OriginalFace = append(result.OriginalFace, fi)
			}
		}
		if len(seen[0]) != len(seen[1]) {
			return empty, geom.ErrConvergence
		}
		for k := range seen[0] {
			if !seen[1][k] {
				return empty, geom.ErrConvergence
			}
			result.SharedParameters = append(result.SharedParameters, parameters[k])
		}
		sort.Slice(result.SharedParameters, func(i, j int) bool { return result.SharedParameters[i].Cmp(result.SharedParameters[j]) < 0 })
		for _, q := range result.SharedParameters {
			result.SharedVertices = append(result.SharedVertices, common[q.RatString()])
		}
		if e := result.Mesh.Validate(); e != nil {
			return empty, e
		}
		// Linear work incidence check for the shared oriented edge chain.
		type pair [2]int
		counts, balances := map[pair]int{}, map[pair]int{}
		edgeKey := func(a, b int) (pair, int) {
			if a > b {
				return pair{b, a}, -1
			}
			return pair{a, b}, 1
		}
		for _, t := range result.Mesh.Triangles {
			for j, a := range t {
				k, sign := edgeKey(a, t[(j+1)%3])
				counts[k]++
				balances[k] += sign
			}
		}
		for i := 0; i+1 < len(result.SharedVertices); i++ {
			k, _ := edgeKey(result.SharedVertices[i], result.SharedVertices[i+1])
			if counts[k] != 2 || balances[k] != 0 {
				return empty, geom.ErrConvergence
			}
		}

		return result, nil
	}
	return empty, geom.ErrConvergence
}
