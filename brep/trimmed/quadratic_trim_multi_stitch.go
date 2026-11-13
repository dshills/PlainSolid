package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// QuadraticTrimStitchFace is one simple CCW parameter loop on a supported patch.
type QuadraticTrimStitchFace struct {
	Patch *geom.SurfacePatch
	Outer []SplineUV
}

// QuadraticTrimStitchSeam pairs oppositely traversed original trim indices.
type QuadraticTrimStitchSeam struct{ FaceA, TrimA, FaceB, TrimB int }

// CertifiedQuadraticTrimCollection has globally shared seam model vertices.
// Each seam parameter increases in its FaceA trim direction. OriginalTrim maps
// expanded trim indices back to original inputs. OriginalFace maps triangles.
// Faces' exact UV chord coordinates and replaced true-curve model points differ
// on seams; all reported residuals include that difference. This is a distance
// and conformance certificate, not embedding or closed-solid qualification.
type CertifiedQuadraticTrimCollection struct {
	JunctionVertices    []int // Explicit junction group order; empty for the original API.
	Mesh                mesh.Mesh
	OriginalFace        []int
	Faces               []CertifiedCurvedQuadraticTrimMesh
	OriginalTrim        [][]int
	SeamVertices        [][]int
	SeamParameters      [][]*big.Rat
	FillSquaredDistance *big.Rat
}

// StitchQuadraticTrimCollectionCertified supports at most16 faces and32 seams,
// retaining the two-face options' per-face/pass limits. Every trim belongs to
// at most one seam. Seams meeting at a vertex of the same face reject: junction
// identity and cyclic link validation require a separate proof. Source splits
// are fixed globally before triangulation; subsequent uniform refinement is
// synchronized across all faces. Every replaced face residual is recomputed.
func StitchQuadraticTrimCollectionCertified(ctx context.Context, inputs []QuadraticTrimStitchFace, seams []QuadraticTrimStitchSeam, o QuadraticTrimFaceJoinOptions) (CertifiedQuadraticTrimCollection, error) {
	return stitchQuadraticTrimCollection(ctx, inputs, seams, nil, o)
}

func stitchQuadraticTrimCollection(ctx context.Context, inputs []QuadraticTrimStitchFace, seams []QuadraticTrimStitchSeam, junctions [][]QuadraticTrimSeamEndpoint, o QuadraticTrimFaceJoinOptions) (CertifiedQuadraticTrimCollection, error) {
	empty := CertifiedQuadraticTrimCollection{}
	if ctx == nil || len(inputs) < 2 || len(inputs) > 16 || len(seams) < 1 || len(seams) > 32 {
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
	if o.Face.MaxSubdivisions < 1 || o.Face.MaxSubdivisions > 128 || o.Face.MaxSubdivisions&(o.Face.MaxSubdivisions-1) != 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxSharedSegments < 1 || o.MaxSharedSegments > 256 || o.MaxSharedSegments&(o.MaxSharedSegments-1) != 0 || o.MaxPasses < 1 || o.MaxPasses > 8 || !finite(o.Face.Deflection) || o.Face.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	type use struct {
		seam    int
		reverse bool
	}
	memberships := make([]map[int]use, len(inputs))
	for i, p := range inputs {
		if p.Patch == nil {
			return empty, geom.ErrInvalid
		}
		memberships[i] = map[int]use{}
	}
	pieces := make([]int, len(seams))
	controls := make([][3][3]*big.Rat, len(seams))
	for si, s := range seams {
		if s.FaceA < 0 || s.FaceB < 0 || s.FaceA >= len(inputs) || s.FaceB >= len(inputs) || s.FaceA == s.FaceB || s.TrimA < 0 || s.TrimB < 0 || s.TrimA >= len(inputs[s.FaceA].Outer) || s.TrimB >= len(inputs[s.FaceB].Outer) {
			return empty, geom.ErrInvalid
		}
		if _, ok := memberships[s.FaceA][s.TrimA]; ok {
			return empty, geom.ErrInvalid
		}
		if _, ok := memberships[s.FaceB][s.TrimB]; ok {
			return empty, geom.ErrInvalid
		}
		memberships[s.FaceA][s.TrimA] = use{si, false}
		memberships[s.FaceB][s.TrimB] = use{si, true}
		a, b := inputs[s.FaceA], inputs[s.FaceB]
		ta, tb := a.Outer[s.TrimA], b.Outer[s.TrimB]
		n, e := quadraticSharedSourcePartition(ctx, a.Patch, b.Patch, ta, tb, o)
		if e != nil {
			return empty, e
		}
		pieces[si] = n
		controls[si], e = quadraticTrimSourceControls(ta)
		if e != nil {
			return empty, e
		}
	}
	expanded := make([][]SplineUV, len(inputs))
	ancestry := make([][]int, len(inputs))
	for fi, input := range inputs {
		for j, t := range input.Outer {
			n := 1
			if m, ok := memberships[fi][j]; ok {
				n = pieces[m.seam]
			}
			for k := 0; k < n; k++ {
				p, e := quadraticDirectedTrimPart(t, k, n)
				if e != nil {
					return empty, e
				}
				expanded[fi] = append(expanded[fi], p)
				ancestry[fi] = append(ancestry[fi], j)
			}
		}
	}
	seamPoint := func(si int, q *big.Rat) ([3]*big.Rat, geom.Vec3, error) {
		one := new(big.Rat).Sub(big.NewRat(1, 1), q)
		basis := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, q)), new(big.Rat).Mul(q, q)}
		var h [3]*big.Rat
		for k := range h {
			h[k] = new(big.Rat)
			for j, w := range basis {
				h[k].Add(h[k], new(big.Rat).Mul(w, controls[si][j][k]))
			}
		}
		jet, e := inputs[seams[si].FaceA].Patch.EvaluateRational(ctx, new(big.Rat).Quo(h[0], h[2]), new(big.Rat).Quo(h[1], h[2]), o.Face.Evaluation)
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
	endpointGroups := map[[2]int]int{}
	if len(junctions) > 2*len(seams) {
		return empty, geom.ErrInvalid
	}
	for group, endpoints := range junctions {
		if len(endpoints) < 2 || len(endpoints) > 2*len(seams) {
			return empty, geom.ErrInvalid
		}
		var reference [3]*big.Rat
		seenSeams := map[int]bool{}
		for i, endpoint := range endpoints {
			if endpoint.Seam < 0 || endpoint.Seam >= len(seams) || endpoint.End < 0 || endpoint.End > 1 || seenSeams[endpoint.Seam] {
				return empty, geom.ErrInvalid
			}
			seenSeams[endpoint.Seam] = true
			key := [2]int{endpoint.Seam, endpoint.End}
			if _, exists := endpointGroups[key]; exists {
				return empty, geom.ErrInvalid
			}
			endpointGroups[key] = group
			point, _, e := seamPoint(endpoint.Seam, big.NewRat(int64(endpoint.End), 1))
			if e != nil {
				return empty, e
			}
			if i == 0 {
				reference = point
			} else {
				for k := range point {
					if point[k].Cmp(reference[k]) != 0 {
						return empty, geom.ErrInvalid
					}
				}
			}
		}
	}
	minimum := 1
	target := ratInset(o.Face.Deflection)
	target.Mul(target, target)
	limit := o.Face.MaxTriangleChecks
	if limit == 0 {
		limit = 100000
	}
	for pass := 0; pass < o.MaxPasses && minimum <= o.Face.MaxSubdivisions; pass++ {
		result := CertifiedQuadraticTrimCollection{Faces: make([]CertifiedCurvedQuadraticTrimMesh, len(inputs)), OriginalTrim: ancestry, FillSquaredDistance: new(big.Rat), SeamVertices: make([][]int, len(seams)), SeamParameters: make([][]*big.Rat, len(seams))}
		level := minimum
		for fi, input := range inputs {
			f, e := tessellateCurvedQuadraticTrimRegionCertified(ctx, input.Patch, expanded[fi], nil, o.Face, minimum)
			if e != nil {
				return empty, e
			}
			result.Faces[fi] = f
			if f.Subdivisions > level {
				level = f.Subdivisions
			}
		}
		for fi, input := range inputs {
			if result.Faces[fi].Subdivisions != level {
				spent := result.Faces[fi].TriangleChecks
				remaining := o.Face
				remaining.MaxTriangleChecks = limit - spent
				if remaining.MaxTriangleChecks < 1 {
					return empty, geom.ErrConvergence
				}
				f, e := tessellateCurvedQuadraticTrimRegionCertified(ctx, input.Patch, expanded[fi], nil, remaining, level)
				if e != nil {
					return empty, e
				}
				f.TriangleChecks += spent
				result.Faces[fi] = f
			}
		}
		type identity struct {
			seam int
			q    string
		}
		canonical := func(key identity) identity {
			end := -1
			if key.q == "0" {
				end = 0
			}
			if key.q == "1" {
				end = 1
			}
			if end >= 0 {
				if group, ok := endpointGroups[[2]int{key.seam, end}]; ok {
					return identity{-1 - group, ""}
				}
			}
			return key
		}
		sourceSeen := map[identity]map[int]bool{}
		local := make([]map[int]identity, len(inputs))
		parameters := map[identity]*big.Rat{}
		good := true
		for fi, input := range inputs {
			local[fi] = map[int]identity{}
			face := &result.Faces[fi]
			boundary := face.Boundary.Region.Loops[0].Boundary
			if len(face.BoundaryLoops[0]) != len(boundary.Segments)*level {
				return empty, geom.ErrConvergence
			}
			for j, s := range boundary.Segments {
				original := ancestry[fi][s.Source]
				member, ok := memberships[fi][original]
				if !ok {
					continue
				}
				trim := input.Outer[original]
				normalize := func(t float64) *big.Rat {
					q := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(t), ratInset(trim.Range.Min)), new(big.Rat).Sub(ratInset(trim.Range.Max), ratInset(trim.Range.Min)))
					if trim.Reversed {
						q.Sub(big.NewRat(1, 1), q)
					}
					if member.reverse {
						q.Sub(big.NewRat(1, 1), q)
					}
					return q
				}
				q0, q1 := normalize(s.Start), normalize(s.End)
				for k := 0; k <= level; k++ {
					q := new(big.Rat).Add(q0, new(big.Rat).Mul(new(big.Rat).Sub(q1, q0), big.NewRat(int64(k), int64(level))))
					id := face.BoundaryLoops[0][(j*level+k)%len(face.BoundaryLoops[0])]
					key := identity{member.seam, q.RatString()}
					if old, exists := local[fi][id]; exists && old != canonical(key) {
						return empty, geom.ErrConvergence
					}
					local[fi][id] = canonical(key)
					if sourceSeen[key] == nil {
						sourceSeen[key] = map[int]bool{}
					}
					sourceSeen[key][fi] = true
					parameters[key] = q
					exact, p, e := seamPoint(member.seam, q)
					if e != nil {
						return empty, e
					}
					face.ExactPoints[id] = exact
					face.Mesh.Vertices[id] = p
				}
			}
			if len(face.Mesh.Triangles) > limit-face.TriangleChecks {
				return empty, geom.ErrConvergence
			}
			maximum := new(big.Rat)
			for j, t := range face.Mesh.Triangles {
				if e := ctx.Err(); e != nil {
					return empty, e
				}
				uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
				model := [3]geom.Vec3{face.Mesh.Vertices[t[0]], face.Mesh.Vertices[t[1]], face.Mesh.Vertices[t[2]]}
				bound, ok := rationalBiquadraticExactCorrespondenceBound(input.Patch, uv, model)
				face.TriangleChecks++
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
		common := map[identity]int{}
		for fi, face := range result.Faces {
			remap := make([]int, len(face.Mesh.Vertices))
			for j, p := range face.Mesh.Vertices {
				id := -1
				if key, ok := local[fi][j]; ok {
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
		for key, faces := range sourceSeen {
			if len(faces) != 2 {
				return empty, geom.ErrConvergence
			}
			result.SeamParameters[key.seam] = append(result.SeamParameters[key.seam], parameters[key])
		}
		for si := range seams {
			sort.Slice(result.SeamParameters[si], func(i, j int) bool { return result.SeamParameters[si][i].Cmp(result.SeamParameters[si][j]) < 0 })
			for _, q := range result.SeamParameters[si] {
				result.SeamVertices[si] = append(result.SeamVertices[si], common[canonical(identity{si, q.RatString()})])
			}
		}
		if e := result.Mesh.Validate(); e != nil {
			return empty, e
		}
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
				k, s := edgeKey(a, t[(j+1)%3])
				counts[k]++
				balances[k] += s
			}
		}
		for _, loop := range result.SeamVertices {
			if len(loop) < 2 {
				return empty, geom.ErrConvergence
			}
			for i := 0; i+1 < len(loop); i++ {
				k, _ := edgeKey(loop[i], loop[i+1])
				if counts[k] != 2 || balances[k] != 0 {
					return empty, geom.ErrConvergence
				}
			}
		}
		for k, count := range counts {
			if count > 2 || count == 2 && balances[k] != 0 {
				return empty, geom.ErrConvergence
			}
		}
		for group := range junctions {
			id, ok := common[identity{-1 - group, ""}]
			if !ok {
				return empty, geom.ErrConvergence
			}
			result.JunctionVertices = append(result.JunctionVertices, id)
		}
		if e := validateQuadraticJunctionFans(result.Mesh, result.JunctionVertices); e != nil {
			return empty, e
		}
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		return result, nil
	}
	return empty, geom.ErrConvergence
}

func quadraticDirectedTrimPart(trim SplineUV, j, n int) (SplineUV, error) {
	parameter := func(k int) (float64, error) {
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
	a, e := parameter(j)
	if e != nil {
		return SplineUV{}, e
	}
	b, e := parameter(j + 1)
	if e != nil {
		return SplineUV{}, e
	}
	trim.Range = UVInterval{Min: a, Max: b}
	if a > b {
		trim.Range.Min, trim.Range.Max = b, a
	}
	return trim, nil
}

func quadraticSharedSourcePartition(ctx context.Context, a, b *geom.SurfacePatch, ta, tb SplineUV, o QuadraticTrimFaceJoinOptions) (int, error) {
	reversed := tb
	reversed.Reversed = !reversed.Reversed
	_, e := CertifyQuadraticTrimSeam(ctx, a, b, ta, reversed, QuadraticTrimSeamOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: o.Face.Deflection, Evaluation: o.Face.Evaluation}})
	if e != nil {
		return 0, e
	}
	patches := [2]*geom.SurfacePatch{a, b}
	trims := [2]SplineUV{ta, tb}
	var uv [2]*geom.SurfacePatch
	var tolerance [2]float64
	for i, p := range patches {
		_, l1, ok := quadraticPatchLipschitz(p)
		if !ok || l1.Sign() == 0 {
			return 0, geom.ErrConvergence
		}
		q := new(big.Rat).Quo(ratInset(o.Face.Deflection), new(big.Rat).Mul(big.NewRat(8, 1), l1))
		tolerance[i], _ = q.Float64()
		if !finite(tolerance[i]) || tolerance[i] <= 0 {
			return 0, geom.ErrConvergence
		}
		d := p.Domain()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}, []float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
		if e != nil {
			return 0, e
		}
		uv[i], e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			return 0, e
		}
	}
	for n := 1; n <= o.MaxSharedSegments; n *= 2 {
		accepted := true
		for i := range patches {
			for j := 0; j < n; j++ {
				p, e := quadraticDirectedTrimPart(trims[i], j, n)
				if e != nil {
					return 0, e
				}
				_, e = TessellateQuadraticSplineTrimCertified(ctx, uv[i], p, QuadraticTrimTessellationOptions{Deflection: tolerance[i], MaxSegments: 1, Evaluation: o.Face.Evaluation})
				if e != nil {
					if ctx.Err() != nil {
						return 0, ctx.Err()
					}
					accepted = false
					break
				}
			}
		}
		if accepted {
			return n, nil
		}
	}
	return 0, geom.ErrConvergence
}
