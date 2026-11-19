//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// MultiSpanTrimFaceJoinOptions bounds each face's cumulative meshing and
// recertification work. MaxSharedVertices defaults to 4096 (maximum 65536).
// Seam bounds the continuous identity proof independently.
type MultiSpanTrimFaceJoinOptions struct {
	Face              MultiSpanCurvedTrimMeshOptions
	Seam              MultiSpanTrimSeamOptions
	MaxSharedVertices int
}

// CertifiedMultiSpanTrimFaceJoin retains the complete exact common partition.
// SharedParameters are normalized in face A's directed original trim, including
// nonbinary knot cuts. Seam retains original parameter maps and knot ancestry.
// Faces retain their initial Subdivisions count; subsequent boundary insertion
// is nonuniform. Boundary stores the original certified fill proposal.
type CertifiedMultiSpanTrimFaceJoin struct {
	// SharedLocalVertices indexes Faces in the same order as SharedParameters.
	SharedLocalVertices [2][]int
	CertifiedQuadraticTrimFaceJoin
	Seam CertifiedMultiSpanTrimSeam
}

// StitchMultiSpanTrimFacesCertified stitches two CCW outer loops along opposite
// single-span degree-one positive rational UV trims. Supports may cross C0 knots and their
// model seam may be curved. All exact knot cuts and both faces' boundary samples
// enter one shared partition. Exact projective inversion avoids rounding source
// parameters. Collinear UV boundary splitting preserves each certified fill;
// every triangle residual is recertified after one common model-point rounding.
// This proves image distance and opposite seam incidence, not global embedding,
// a closed shell, or support for curved quadratic UV seams.
func StitchMultiSpanTrimFacesCertified(ctx context.Context, a, b QuadraticTrimFaceJoinInput, o MultiSpanTrimFaceJoinOptions) (CertifiedMultiSpanTrimFaceJoin, error) {
	return stitchMultiSpanTrimFaces(ctx, a, b, o, nil)
}

func stitchMultiSpanTrimFaces(ctx context.Context, a, b QuadraticTrimFaceJoinInput, o MultiSpanTrimFaceJoinOptions, prepared *[2]CertifiedCurvedQuadraticTrimMesh) (CertifiedMultiSpanTrimFaceJoin, error) {
	empty := CertifiedMultiSpanTrimFaceJoin{}
	if ctx == nil || a.Patch == nil || b.Patch == nil || a.SharedTrim < 0 || a.SharedTrim >= len(a.Outer) || b.SharedTrim < 0 || b.SharedTrim >= len(b.Outer) {
		return empty, geom.ErrInvalid
	}
	if o.MaxSharedVertices == 0 {
		o.MaxSharedVertices = 4096
	}
	if o.MaxSharedVertices < 2 || o.MaxSharedVertices > 65536 {
		return empty, geom.ErrInvalid
	}
	inputs := [2]QuadraticTrimFaceJoinInput{a, b}
	trims := [2]SplineUV{a.Outer[a.SharedTrim], b.Outer[b.SharedTrim]}
	if trims[0].Curve == nil || trims[1].Curve == nil || trims[0].Curve.Degree() != 1 || trims[1].Curve.Degree() != 1 {
		return empty, geom.ErrConvergence
	}
	trims[1].Reversed = !trims[1].Reversed
	proof, e := CertifyMultiSpanTrimSeam(ctx, a.Patch, b.Patch, trims[0], trims[1], o.Seam)
	if e != nil {
		return empty, e
	}
	if len(proof.Pieces)+1 > o.MaxSharedVertices {
		return empty, geom.ErrConvergence
	}
	result := CertifiedMultiSpanTrimFaceJoin{Seam: proof}
	result.SharedTrim = [2]int{a.SharedTrim, b.SharedTrim}
	result.FillSquaredDistance = new(big.Rat)
	var controls [2][3][3]*big.Rat
	for fi, input := range inputs {
		controls[fi], e = quadraticTrimSourceControls(trims[fi])
		if e != nil {
			return empty, e
		}
		if prepared == nil {
			result.Faces[fi], e = TessellateMultiSpanCurvedTrimRegionCertified(ctx, input.Patch, input.Outer, nil, o.Face)
			if e != nil {
				return empty, e
			}
		} else {
			result.Faces[fi] = prepared[fi]
		}
		for j := range input.Outer {
			result.OriginalTrim[fi] = append(result.OriginalTrim[fi], j)
		}
	}
	uvAt := func(fi int, q *big.Rat) [2]*big.Rat {
		h := controls[fi]
		var v [3]*big.Rat
		for k := range v {
			v[k] = new(big.Rat).Add(h[0][k], new(big.Rat).Mul(q, new(big.Rat).Sub(h[2][k], h[0][k])))
		}
		return [2]*big.Rat{new(big.Rat).Quo(v[0], v[2]), new(big.Rat).Quo(v[1], v[2])}
	}
	parameter := func(fi int, uv [2]*big.Rat) (*big.Rat, error) {
		h := controls[fi]
		for axis := 0; axis < 2; axis++ {
			den := new(big.Rat).Sub(new(big.Rat).Sub(h[2][axis], h[0][axis]), new(big.Rat).Mul(uv[axis], new(big.Rat).Sub(h[2][2], h[0][2])))
			if den.Sign() == 0 {
				continue
			}
			q := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(uv[axis], h[0][2]), h[0][axis]), den)
			if q.Sign() < 0 || q.Cmp(big.NewRat(1, 1)) > 0 {
				return nil, geom.ErrConvergence
			}
			exact := uvAt(fi, q)
			if exact[0].Cmp(uv[0]) != 0 || exact[1].Cmp(uv[1]) != 0 {
				return nil, geom.ErrConvergence
			}
			return q, nil
		}
		return nil, geom.ErrConvergence
	}
	local := [2]map[int]*big.Rat{make(map[int]*big.Rat), make(map[int]*big.Rat)}
	all := map[string]*big.Rat{}
	add := func(q *big.Rat) error {
		all[q.RatString()] = q
		if len(all) > o.MaxSharedVertices {
			return geom.ErrConvergence
		}
		return nil
	}
	for _, p := range proof.Pieces {
		if e = add(p.Min); e != nil {
			return empty, e
		}
		if e = add(p.Max); e != nil {
			return empty, e
		}
	}
	// Exact geometric membership is valid for a simple certified UV loop and a
	// straight source edge. It also finds samples inserted by earlier seams.
	for fi := range inputs {
		face := &result.Faces[fi]
		if len(face.BoundaryLoops) != 1 {
			return empty, geom.ErrConvergence
		}
		endpoints := [2]bool{}
		for _, id := range face.BoundaryLoops[0] {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			q, err := parameter(fi, face.ExactUV[id])
			if err != nil {
				continue
			}
			local[fi][id] = q
			if q.Sign() == 0 {
				endpoints[0] = true
			}
			if q.Cmp(big.NewRat(1, 1)) == 0 {
				endpoints[1] = true
			}
			if e = add(q); e != nil {
				return empty, e
			}
		}
		if !endpoints[0] || !endpoints[1] {
			return empty, geom.ErrConvergence
		}
	}

	for _, q := range all {
		result.SharedParameters = append(result.SharedParameters, q)
	}
	sort.Slice(result.SharedParameters, func(i, j int) bool { return result.SharedParameters[i].Cmp(result.SharedParameters[j]) < 0 })
	exact := map[string][3]*big.Rat{}
	display := map[string]geom.Vec3{}
	for _, q := range result.SharedParameters {
		uv := uvAt(0, q)
		jet, err := a.Patch.EvaluateRational(ctx, uv[0], uv[1], o.Face.Mesh.Evaluation)
		if err != nil {
			return empty, err
		}
		p := geom.Vec3{}
		p.X, _ = jet.Point[0].Float64()
		p.Y, _ = jet.Point[1].Float64()
		p.Z, _ = jet.Point[2].Float64()
		if !p.IsFinite() {
			return empty, geom.ErrConvergence
		}
		exact[q.RatString()], display[q.RatString()] = jet.Point, p
	}
	type edge [2]int
	edgeKey := func(a, b int) edge {
		if a > b {
			a, b = b, a
		}
		return edge{a, b}
	}
	for fi, input := range inputs {
		face := &result.Faces[fi]
		ids := map[string]int{}
		for id, q := range local[fi] {
			ids[q.RatString()] = id
		}
		for _, q := range result.SharedParameters {
			key := q.RatString()
			id, ok := ids[key]
			if !ok {
				id = len(face.Mesh.Vertices)
				ids[key] = id
				local[fi][id] = q
				face.ExactUV = append(face.ExactUV, uvAt(fi, q))
				face.Mesh.Vertices = append(face.Mesh.Vertices, geom.Vec3{})
				face.ExactPoints = append(face.ExactPoints, [3]*big.Rat{})
			}
			face.Mesh.Vertices[id], face.ExactPoints[id] = display[key], exact[key]
			result.SharedLocalVertices[fi] = append(result.SharedLocalVertices[fi], id)
		}
		splits := map[edge][]int{}
		loop := face.BoundaryLoops[0]
		var nextLoop []int
		for j, id := range loop {
			end := loop[(j+1)%len(loop)]
			q0, ok0 := local[fi][id]
			q1, ok1 := local[fi][end]
			nextLoop = append(nextLoop, id)
			if !ok0 || !ok1 {
				continue
			}
			lo, hi := q0, q1
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			var inner []int
			start := sort.Search(len(result.SharedParameters), func(k int) bool { return result.SharedParameters[k].Cmp(lo) > 0 })
			for k := start; k < len(result.SharedParameters) && result.SharedParameters[k].Cmp(hi) < 0; k++ {
				inner = append(inner, ids[result.SharedParameters[k].RatString()])
			}
			if q0.Cmp(q1) > 0 {
				for l, r := 0, len(inner)-1; l < r; l, r = l+1, r-1 {
					inner[l], inner[r] = inner[r], inner[l]
				}
			}
			chain := append([]int{id}, inner...)
			chain = append(chain, end)
			splits[edgeKey(id, end)] = chain
			nextLoop = append(nextLoop, inner...)
		}
		face.BoundaryLoops[0] = nextLoop
		limit := o.Face.Mesh.MaxTriangles
		if limit == 0 {
			limit = 100000
		}
		var triangles [][3]int
		for _, t := range face.Mesh.Triangles {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
			split := -1
			var chain []int
			for j, id := range t {
				if c, ok := splits[edgeKey(id, t[(j+1)%3])]; ok && len(c) > 2 {
					if split >= 0 {
						return empty, geom.ErrConvergence
					}
					split = j
					chain = c
				}
			}
			if split < 0 {
				triangles = append(triangles, t)
			} else {
				if chain[0] != t[split] {
					chain = append([]int(nil), chain...)
					for l, r := 0, len(chain)-1; l < r; l, r = l+1, r-1 {
						chain[l], chain[r] = chain[r], chain[l]
					}
				}
				for j := 0; j+1 < len(chain); j++ {
					triangles = append(triangles, [3]int{chain[j], chain[j+1], t[(split+2)%3]})
				}
			}
			if len(triangles) > limit {
				return empty, geom.ErrConvergence
			}
		}
		face.Mesh.Triangles = triangles
		// Continue both counters from the initial face certificate.
		opts := o.Face
		if opts.MaxSpanCells == 0 {
			opts.MaxSpanCells = 100000
		}
		if opts.MaxSpanPieces == 0 {
			opts.MaxSpanPieces = 100000
		}
		if opts.Span.MaxCells == 0 {
			opts.Span.MaxCells = 1024
		}
		if opts.Span.MaxPieces == 0 {
			opts.Span.MaxPieces = 8192
		}
		if opts.Span.MaxInputBits == 0 {
			opts.Span.MaxInputBits = 16384
		}
		work := multiSpanMeshWork{options: opts, cells: face.SpanCellChecks, pieces: face.SpanPieceChecks}
		triangleLimit := opts.Mesh.MaxTriangleChecks
		if triangleLimit == 0 {
			triangleLimit = 100000
		}
		if len(triangles) > triangleLimit-face.TriangleChecks {
			return empty, geom.ErrConvergence
		}
		face.SquaredErrors = nil
		face.OriginalFace = nil
		maximum := new(big.Rat)
		for _, t := range triangles {
			uv := [3][2]*big.Rat{face.ExactUV[t[0]], face.ExactUV[t[1]], face.ExactUV[t[2]]}
			model := [3]geom.Vec3{face.Mesh.Vertices[t[0]], face.Mesh.Vertices[t[1]], face.Mesh.Vertices[t[2]]}
			bound, err := work.bound(ctx, input.Patch, uv, model)
			if err != nil {
				return empty, err
			}
			face.TriangleChecks++
			face.SquaredErrors = append(face.SquaredErrors, bound)
			face.OriginalFace = append(face.OriginalFace, fi)
			if bound.Cmp(maximum) > 0 {
				maximum = bound
			}
		}
		face.SpanCellChecks, face.SpanPieceChecks = work.cells, work.pieces
		face.SurfaceSquaredDistance = maximum
		face.FillSquaredDistance = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(face.TransportedTrimSquaredDistance, maximum))
		target := ratInset(o.Face.Mesh.Deflection)
		target.Mul(target, target)
		if face.FillSquaredDistance.Cmp(target) > 0 {
			return empty, geom.ErrConvergence
		}
		if face.FillSquaredDistance.Cmp(result.FillSquaredDistance) > 0 {
			result.FillSquaredDistance.Set(face.FillSquaredDistance)
		}
	}
	common := map[string]int{}
	for fi, face := range result.Faces {
		remap := make([]int, len(face.Mesh.Vertices))
		for id, p := range face.Mesh.Vertices {
			global := -1
			if q, ok := local[fi][id]; ok {
				key := q.RatString()
				if old, found := common[key]; found {
					global = old
				} else {
					global = len(result.Mesh.Vertices)
					common[key] = global
					result.Mesh.Vertices = append(result.Mesh.Vertices, p)
				}
			} else {
				global = len(result.Mesh.Vertices)
				result.Mesh.Vertices = append(result.Mesh.Vertices, p)
			}
			remap[id] = global
		}
		for _, t := range face.Mesh.Triangles {
			result.Mesh.Triangles = append(result.Mesh.Triangles, [3]int{remap[t[0]], remap[t[1]], remap[t[2]]})
			result.OriginalFace = append(result.OriginalFace, fi)
		}
	}
	for _, q := range result.SharedParameters {
		result.SharedVertices = append(result.SharedVertices, common[q.RatString()])
	}
	if e = result.Mesh.Validate(); e != nil {
		return empty, e
	}
	counts, balances := map[edge]int{}, map[edge]int{}
	for _, t := range result.Mesh.Triangles {
		for j, a := range t {
			b := t[(j+1)%3]
			key := edgeKey(a, b)
			counts[key]++
			if a < b {
				balances[key]++
			} else {
				balances[key]--
			}
		}
	}
	for key, n := range counts {
		if n > 2 || (n == 2 && balances[key] != 0) {
			return empty, geom.ErrConvergence
		}
	}
	for i := 0; i+1 < len(result.SharedVertices); i++ {
		key := edgeKey(result.SharedVertices[i], result.SharedVertices[i+1])
		if counts[key] != 2 || balances[key] != 0 {
			return empty, geom.ErrConvergence
		}
	}
	return result, nil
}
