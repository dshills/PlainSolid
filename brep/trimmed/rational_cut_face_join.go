//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// RationalCutFaceJoinOptions bounds exact common partition synchronization.
// MaxPasses defaults to8 (maximum16); MaxSharedVertices to4096 (maximum65536).
// Face triangle/cell/piece and boundary check budgets accumulate across ALL
// synchronization retries and final seam recertification.
type RationalCutFaceJoinOptions struct {
	Face                         RationalCutFaceMeshOptions
	MaxPasses, MaxSharedVertices int
}
type CertifiedRationalCutFaceJoin struct {
	nativeTrims        [2][]CertifiedExactQuadraticTrimPolyline
	nativeAgreement    geom.SurfaceCurveAlgebraicAgreement
	nativeCompositions [2]geom.SurfaceCurveCompositionPieces
	CertifiedQuadraticTrimFaceJoin
	Trims [2][]CertifiedRationalCutTrimPolyline
	Seam  CertifiedMultiSpanTrimSeam
}

// StitchRationalCutFacesCertified supports positive rational quadratic or
// linear UV seams on piecewise biquadratic supports. The exact common source
// partition is synchronized before polygon triangulation. Common dyadic
// refinements retain exact source ancestry. All seam samples are replaced by
// one exact common model value and every triangle is recertified afterward.
// Independent source/polygon proofs justify each closed UV fill; opposite seam
// incidence is verified. No global embedding or closed-shell claim is made.
func StitchRationalCutFacesCertified(ctx context.Context, a, b QuadraticTrimFaceJoinInput, o RationalCutFaceJoinOptions) (CertifiedRationalCutFaceJoin, error) {
	return stitchControlledCutFaces(ctx, a, b, o, nil, geom.SurfaceCurveCompositionOptions{}, geom.SurfaceCurveAgreementOptions{}, false)
}
func stitchControlledCutFaces(ctx context.Context, a, b QuadraticTrimFaceJoinInput, o RationalCutFaceJoinOptions, native *[2][]ExactQuadraticUV, composition geom.SurfaceCurveCompositionOptions, agreementOptions geom.SurfaceCurveAgreementOptions, fixedBoundary bool) (CertifiedRationalCutFaceJoin, error) {
	empty := CertifiedRationalCutFaceJoin{}
	if !finite(o.Face.Face.Mesh.Deflection) || o.Face.Face.Mesh.Deflection <= 0 || ctx == nil || a.Patch == nil || b.Patch == nil || a.SharedTrim < 0 || b.SharedTrim < 0 || a.SharedTrim >= len(a.Outer) || b.SharedTrim >= len(b.Outer) {
		return empty, geom.ErrInvalid
	}
	if o.MaxPasses == 0 {
		o.MaxPasses = 8
	}
	if o.MaxSharedVertices == 0 {
		o.MaxSharedVertices = 4096
	}
	if o.MaxPasses < 1 || o.MaxPasses > 16 || o.MaxSharedVertices < 2 || o.MaxSharedVertices > 65536 {
		return empty, geom.ErrInvalid
	}
	if o.Face.Face.MaxSpanCells == 0 {
		o.Face.Face.MaxSpanCells = 100000
	}
	if o.Face.Face.MaxSpanPieces == 0 {
		o.Face.Face.MaxSpanPieces = 100000
	}
	if o.Face.Face.Span.MaxCells == 0 {
		o.Face.Face.Span.MaxCells = 1024
	}
	if o.Face.Face.Span.MaxPieces == 0 {
		o.Face.Face.Span.MaxPieces = 8192
	}
	if o.Face.Face.Span.MaxInputBits == 0 {
		o.Face.Face.Span.MaxInputBits = 16384
	}
	if o.Face.Face.Mesh.MaxTriangleChecks == 0 {
		o.Face.Face.Mesh.MaxTriangleChecks = 100000
	}
	if o.Face.Face.Mesh.MaxSubdivisions == 0 {
		o.Face.Face.Mesh.MaxSubdivisions = 32
	}
	if o.Face.Face.Mesh.Boundary.Tessellation.MaxSegments == 0 {
		o.Face.Face.Mesh.Boundary.Tessellation.MaxSegments = 4096
	}
	if o.Face.Face.Mesh.Boundary.Tessellation.MaxChecks == 0 {
		o.Face.Face.Mesh.Boundary.Tessellation.MaxChecks = 2*o.Face.Face.Mesh.Boundary.Tessellation.MaxSegments - 1
	}
	inputs := [2]QuadraticTrimFaceJoinInput{a, b}
	shared := [2]SplineUV{a.Outer[a.SharedTrim], b.Outer[b.SharedTrim]}
	canonicalB := shared[1]
	canonicalB.Reversed = !canonicalB.Reversed
	var proof CertifiedMultiSpanTrimSeam
	var e error
	var agreement geom.SurfaceCurveAlgebraicAgreement
	var compositions [2]geom.SurfaceCurveCompositionPieces
	if native == nil {
		proof, e = CertifyMultiSpanTrimSeam(ctx, a.Patch, b.Patch, shared[0], canonicalB, o.Face.Partition)
		if e != nil {
			return empty, e
		}
	} else {
		var curves [2]*geom.ExactSplineCurve
		for fi, input := range inputs {
			trim := native[fi][input.SharedTrim]
			if fi == 1 {
				trim = trim.reversed()
			}
			h := trim.HomogeneousControls()
			controls := make([][4]*big.Rat, 3)
			for j, c := range h {
				controls[j] = [4]*big.Rat{c[0], c[1], new(big.Rat), c[2]}
			}
			curves[fi], e = geom.NewExactHomogeneousBezier(ctx, controls, composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
		}
		agreement, e = geom.CompareSurfaceCurveCompositions(ctx, a.Patch, curves[0], b.Patch, curves[1], agreementOptions)
		if e != nil {
			return empty, e
		}
		if !agreement.Complete {
			return empty, geom.ErrConvergence
		}
		if !agreement.Equal {
			return empty, geom.ErrInvalid
		}
		var parameters []*big.Rat
		for fi, input := range inputs {
			compositions[fi], e = input.Patch.ComposeParameterCurvePieces(ctx, curves[fi], composition)
			if e != nil {
				return empty, e
			}
			if !compositions[fi].Complete || !compositions[fi].Continuous {
				return empty, geom.ErrConvergence
			}
			for _, piece := range compositions[fi].Pieces {
				parameters = append(parameters, piece.Start, piece.End)
			}
		}
		sort.Slice(parameters, func(i, j int) bool { return parameters[i].Cmp(parameters[j]) < 0 })
		unique := parameters[:0]
		for _, q := range parameters {
			if len(unique) == 0 || q.Cmp(unique[len(unique)-1]) != 0 {
				unique = append(unique, q)
			}
		}
		parameters = unique
		for i := 0; i+1 < len(parameters); i++ {
			proof.Pieces = append(proof.Pieces, MultiSpanTrimSeamPiece{Min: parameters[i], Max: parameters[i+1]})
		}
	}
	var cuts [2][][]*big.Rat
	var ancestry [2][]int
	var boundaryChecks [2]int
	for fi, in := range inputs {
		cuts[fi] = make([][]*big.Rat, len(in.Outer))
		for j := range in.Outer {
			ancestry[fi] = append(ancestry[fi], j)
		}
		for _, piece := range proof.Pieces {
			for _, q := range []*big.Rat{piece.Min, piece.Max} {
				v := new(big.Rat).Set(q)
				if fi == 1 {
					v.Sub(big.NewRat(1, 1), v)
				}
				cuts[fi][in.SharedTrim] = append(cuts[fi][in.SharedTrim], v)
			}
		}
	}
	var h [3][3]*big.Rat
	if native == nil {
		h, e = quadraticTrimSourceControls(shared[0])
		if e != nil {
			return empty, e
		}
	} else {
		h = native[0][a.SharedTrim].HomogeneousControls()
	}
	type seamSample struct {
		exact   [3]*big.Rat
		display geom.Vec3
	}
	seamSamples := map[string]seamSample{}
	exactSeam := func(q *big.Rat) ([3]*big.Rat, geom.Vec3, error) {
		if sample, ok := seamSamples[q.RatString()]; ok {
			return sample.exact, sample.display, nil
		}
		one := new(big.Rat).Sub(big.NewRat(1, 1), q)
		basis := [3]*big.Rat{new(big.Rat).Mul(one, one), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(one, q)), new(big.Rat).Mul(q, q)}
		var v [3]*big.Rat
		for k := range v {
			v[k] = new(big.Rat)
			for j, w := range basis {
				v[k].Add(v[k], new(big.Rat).Mul(w, h[j][k]))
			}
		}
		jet, e := a.Patch.EvaluateRational(ctx, new(big.Rat).Quo(v[0], v[2]), new(big.Rat).Quo(v[1], v[2]), o.Face.Face.Mesh.Evaluation)
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
		seamSamples[q.RatString()] = seamSample{jet.Point, p}
		return jet.Point, p, nil
	}
	minimum := 1
	spent := [2]CertifiedCurvedQuadraticTrimMesh{}
	target := ratInset(o.Face.Face.Mesh.Deflection)
	target.Mul(target, target)
	for pass := 0; pass < o.MaxPasses && minimum <= o.Face.Face.Mesh.MaxSubdivisions; pass++ {

		result := CertifiedRationalCutFaceJoin{Seam: proof, nativeAgreement: agreement, nativeCompositions: compositions}
		result.OriginalTrim = ancestry
		result.SharedTrim = [2]int{a.SharedTrim, b.SharedTrim}
		result.FillSquaredDistance = new(big.Rat)
		level := minimum
		for fi, input := range inputs {
			options := o.Face
			options.Face.MaxSpanCells -= spent[fi].SpanCellChecks
			options.Face.MaxSpanPieces -= spent[fi].SpanPieceChecks
			options.Face.Mesh.MaxTriangleChecks -= spent[fi].TriangleChecks
			options.Face.Mesh.Boundary.Tessellation.MaxChecks -= boundaryChecks[fi]
			if options.Face.MaxSpanCells < 1 || options.Face.MaxSpanPieces < 1 || options.Face.Mesh.MaxTriangleChecks < 1 || options.Face.Mesh.Boundary.Tessellation.MaxChecks < 1 {
				return empty, geom.ErrConvergence
			}
			var face CertifiedRationalCutFaceMesh
			var err error
			if native == nil {
				face, err = tessellateRationalCutFace(ctx, input.Patch, input.Outer, cuts[fi], options, minimum)
			} else {
				face, err = buildControlledCutFace(ctx, input.Patch, input.Outer, native[fi], cuts[fi], options, minimum, false, composition, fixedBoundary)
			}
			if err != nil {
				return empty, err
			}
			spent[fi].SpanCellChecks += face.SpanCellChecks
			spent[fi].SpanPieceChecks += face.SpanPieceChecks
			spent[fi].TriangleChecks += face.TriangleChecks
			for _, trim := range face.Trims {
				boundaryChecks[fi] += trim.Checks
			}
			face.SpanCellChecks, face.SpanPieceChecks, face.TriangleChecks = spent[fi].SpanCellChecks, spent[fi].SpanPieceChecks, spent[fi].TriangleChecks
			result.Faces[fi] = face.CertifiedCurvedQuadraticTrimMesh
			result.Trims[fi] = face.Trims
			result.nativeTrims[fi] = face.nativeTrims
			if face.Subdivisions > level {
				level = face.Subdivisions
			}
		}
		// A single global source partition must exist before either polygon
		// triangulation is accepted. Refine both originals at the exact union.
		union := map[string]*big.Rat{}
		for fi, input := range inputs {
			for _, q := range result.Trims[fi][input.SharedTrim].NormalizedParameters {
				canonical := new(big.Rat).Set(q)
				if fi == 1 {
					canonical.Sub(big.NewRat(1, 1), canonical)
				}
				union[canonical.RatString()] = canonical
			}
		}
		if len(union) > o.MaxSharedVertices {
			return empty, geom.ErrConvergence
		}
		partitionParameters := make([]*big.Rat, 0, len(union))
		for _, q := range union {
			partitionParameters = append(partitionParameters, q)
		}
		sort.Slice(partitionParameters, func(i, j int) bool { return partitionParameters[i].Cmp(partitionParameters[j]) < 0 })
		synchronized := true
		for fi, input := range inputs {
			if len(result.Trims[fi][input.SharedTrim].NormalizedParameters) != len(partitionParameters) {
				synchronized = false
			}
			cuts[fi][input.SharedTrim] = nil
			for _, q := range partitionParameters {
				v := new(big.Rat).Set(q)
				if fi == 1 {
					v.Sub(big.NewRat(1, 1), v)
				}
				cuts[fi][input.SharedTrim] = append(cuts[fi][input.SharedTrim], v)
			}
		}
		if !synchronized || result.Faces[0].Subdivisions != level || result.Faces[1].Subdivisions != level {
			minimum = level
			continue
		}
		localKeys := [2]map[int]*big.Rat{make(map[int]*big.Rat), make(map[int]*big.Rat)}
		good := true
		for fi, input := range inputs {
			face := &result.Faces[fi]

			start := 0
			for j := 0; j < input.SharedTrim; j++ {
				start += len(result.Trims[fi][j].SquaredErrors) * level
			}
			line := result.Trims[fi][input.SharedTrim]
			if len(line.SquaredErrors) > (o.MaxSharedVertices-1)/level {
				return empty, geom.ErrConvergence
			}
			for j := range line.SquaredErrors {
				q0, q1 := new(big.Rat).Set(line.NormalizedParameters[j]), new(big.Rat).Set(line.NormalizedParameters[j+1])
				if fi == 1 {
					q0.Sub(big.NewRat(1, 1), q0)
					q1.Sub(big.NewRat(1, 1), q1)
				}
				for k := 0; k <= level; k++ {
					q := new(big.Rat).Add(q0, new(big.Rat).Mul(new(big.Rat).Sub(q1, q0), big.NewRat(int64(k), int64(level))))
					id := face.BoundaryLoops[0][(start+j*level+k)%len(face.BoundaryLoops[0])]
					if old, exists := localKeys[fi][id]; exists && old.Cmp(q) != 0 {
						return empty, geom.ErrConvergence
					}
					localKeys[fi][id] = q
					exact, p, e := exactSeam(q)
					if e != nil {
						return empty, e
					}
					face.Mesh.Vertices[id], face.ExactPoints[id] = p, exact
				}
			}
			limit := o.Face.Face.Mesh.MaxTriangleChecks
			if limit == 0 {
				limit = 100000
			}
			if len(face.Mesh.Triangles) > limit-face.TriangleChecks {
				return empty, geom.ErrConvergence
			}
			work := multiSpanMeshWork{options: o.Face.Face, cells: face.SpanCellChecks, pieces: face.SpanPieceChecks}
			maximum := new(big.Rat)
			for j, t := range face.Mesh.Triangles {
				face.TriangleChecks++
				uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
				model := [3]geom.Vec3{face.Mesh.Vertices[t[0]], face.Mesh.Vertices[t[1]], face.Mesh.Vertices[t[2]]}
				bound, err := work.bound(ctx, input.Patch, uv, model)
				if err != nil {
					return empty, err
				}
				face.SquaredErrors[j] = bound
				if bound.Cmp(maximum) > 0 {
					maximum = bound
				}
			}
			face.SpanCellChecks, face.SpanPieceChecks = work.cells, work.pieces
			spent[fi].SpanCellChecks, spent[fi].SpanPieceChecks, spent[fi].TriangleChecks = face.SpanCellChecks, face.SpanPieceChecks, face.TriangleChecks
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
		if len(localKeys[0]) > o.MaxSharedVertices || len(localKeys[1]) > o.MaxSharedVertices {
			return empty, geom.ErrConvergence
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
		for key, count := range counts {
			if count > 2 || (count == 2 && balances[key] != 0) {
				return empty, geom.ErrConvergence
			}
		}
		if e := validateQuadraticJunctionFans(result.Mesh, result.SharedVertices); e != nil {
			return empty, e
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
