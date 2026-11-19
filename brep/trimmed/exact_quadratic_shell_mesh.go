//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

// ExactQuadraticShellFace owns a CCW outer loop and CW hole loops. Seam trim
// indices flatten Outer followed by Holes in their supplied order.
type ExactQuadraticShellFace struct {
	Patch *geom.SurfacePatch
	Outer []ExactQuadraticUV
	Holes [][]ExactQuadraticUV
}

// ExactQuadraticShellMeshOptions retains per-face cumulative budgets and
// per-seam symbolic/composition budgets. Aggregate pre-weld vertices/triangles
// default to100000 (maximum1M); exact collision has its own lower hard limits.
type ExactQuadraticShellMeshOptions struct {
	Join                      ExactQuadraticFaceJoinOptions
	MaxTriangles, MaxVertices int
	Collision                 collision.Options
}

// CertifiedEmbeddedExactQuadraticShell proves the represented display is one
// connected, oriented, closed, embedded manifold with positive exact signed
// volume. It also bounds both directed image distances to the source faces.
// These properties do not prove continuous source-shell embedding or its mass.
// JunctionVertices follows explicit caller junction order; seam arrays follow
// declared seam order. All exact source trims and symbolic agreements survive.
type CertifiedEmbeddedExactQuadraticShell struct {
	// Trim indices flatten outer then original hole loops. Hole polyline
	// certificates are normalized CCW; TrimReversed marks that reversal.
	TrimLoops    [][]int
	TrimReversed [][]bool
	CertifiedQuadraticTrimCollection
	Trims               [][]CertifiedExactQuadraticTrimPolyline
	Agreements          []geom.SurfaceCurveAlgebraicAgreement
	DisplayCollision    collision.Report
	SignedDisplayVolume *big.Rat
}

// StitchEmbeddedExactQuadraticShellCertified requires2..16 native faces,
// complete single ownership of all trims by1..32 opposite seam pairs, and
// explicit corner groups. Each group names endpoint0/1 in its seam's FaceA
// traversal. Distinct corners are never inferred from proximity. Rational cuts
// synchronize globally before fixed-boundary interior refinement; shared model
// points must be exactly equal and no certified vertex is subsequently moved.
// Every vertex fan, closed opposite incidence, connectivity, signed volume and
// exact non-incidence contact is checked. Failures return no partial shell.
func StitchEmbeddedExactQuadraticShellCertified(ctx context.Context, faces []ExactQuadraticShellFace, seams []QuadraticTrimStitchSeam, junctions [][]QuadraticTrimSeamEndpoint, o ExactQuadraticShellMeshOptions) (CertifiedEmbeddedExactQuadraticShell, error) {
	empty := CertifiedEmbeddedExactQuadraticShell{}
	if !finite(o.Join.Face.Face.Mesh.Deflection) || o.Join.Face.Face.Mesh.Deflection <= 0 || o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 || ctx == nil || len(faces) < 2 || len(faces) > 16 || len(seams) < 1 || len(seams) > 32 || len(junctions) > 64 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 100000
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 100000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 1000000 || o.MaxVertices < 1 || o.MaxVertices > 1000000 {
		return empty, geom.ErrInvalid
	}
	join := o.Join
	if join.MaxPasses == 0 {
		join.MaxPasses = 8
	}
	if join.MaxSharedVertices == 0 {
		join.MaxSharedVertices = 4096
	}
	if join.MaxPasses < 1 || join.MaxPasses > 16 || join.MaxSharedVertices < 2 || join.MaxSharedVertices > 65536 {
		return empty, geom.ErrInvalid
	}
	options := join.Face.Face
	if options.MaxSpanCells == 0 {
		options.MaxSpanCells = 100000
	}
	if options.MaxSpanPieces == 0 {
		options.MaxSpanPieces = 100000
	}
	if options.Mesh.MaxTriangleChecks == 0 {
		options.Mesh.MaxTriangleChecks = 100000
	}
	if options.Mesh.Boundary.Tessellation.MaxSegments == 0 {
		options.Mesh.Boundary.Tessellation.MaxSegments = 4096
	}
	if options.Mesh.Boundary.Tessellation.MaxChecks == 0 {
		options.Mesh.Boundary.Tessellation.MaxChecks = 2*options.Mesh.Boundary.Tessellation.MaxSegments - 1
	}
	type membership struct {
		seam    int
		reverse bool
	}
	flat := make([][]ExactQuadraticUV, len(faces))
	loops := make([][][]ExactQuadraticUV, len(faces))
	boundaryChecks := make([]int, len(faces))
	owners := make([][]membership, len(faces))
	used := make([][]bool, len(faces))
	cuts := make([][][]*big.Rat, len(faces))
	for fi, f := range faces {
		if f.Patch == nil || len(f.Outer) < 2 || len(f.Holes) > 64 {
			return empty, geom.ErrInvalid
		}
		loops[fi] = append([][]ExactQuadraticUV{f.Outer}, f.Holes...)
		for _, loop := range loops[fi] {
			if len(loop) < 2 {
				return empty, geom.ErrInvalid
			}
			flat[fi] = append(flat[fi], loop...)
		}
		if len(flat[fi]) > 64 {
			return empty, geom.ErrInvalid
		}
		owners[fi] = make([]membership, len(flat[fi]))
		used[fi] = make([]bool, len(flat[fi]))
		cuts[fi] = make([][]*big.Rat, len(flat[fi]))
		for _, trim := range flat[fi] {
			if trim.control[0][0] == nil {
				return empty, geom.ErrInvalid
			}
		}
	}
	agreements := make([]geom.SurfaceCurveAlgebraicAgreement, len(seams))
	for si, s := range seams {
		if s.FaceA < 0 || s.FaceB < 0 || s.FaceA >= len(faces) || s.FaceB >= len(faces) || s.FaceA == s.FaceB || s.TrimA < 0 || s.TrimB < 0 || s.TrimA >= len(flat[s.FaceA]) || s.TrimB >= len(flat[s.FaceB]) {
			return empty, geom.ErrInvalid
		}
		var curves [2]*geom.ExactSplineCurve
		for side, pair := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
			fi, ti := pair[0], pair[1]
			if used[fi][ti] {
				return empty, geom.ErrInvalid
			}
			used[fi][ti] = true
			owners[fi][ti] = membership{si, side == 1}
			trim := flat[fi][ti]
			if side == 1 {
				trim = trim.reversed()
			}
			h := trim.HomogeneousControls()
			net := make([][4]*big.Rat, 3)
			for j, c := range h {
				net[j] = [4]*big.Rat{c[0], c[1], new(big.Rat), c[2]}
			}
			var e error
			curves[side], e = geom.NewExactHomogeneousBezier(ctx, net, join.Face.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
		}
		proof, e := geom.CompareSurfaceCurveCompositions(ctx, faces[s.FaceA].Patch, curves[0], faces[s.FaceB].Patch, curves[1], join.Agreement)
		if e != nil {
			return empty, e
		}
		if !proof.Complete {
			return empty, geom.ErrConvergence
		}
		if !proof.Equal {
			return empty, geom.ErrInvalid
		}
		agreements[si] = proof
		union := map[string]*big.Rat{}
		for side, fi := range []int{s.FaceA, s.FaceB} {
			parts, e := faces[fi].Patch.ComposeParameterCurvePieces(ctx, curves[side], join.Face.Composition)
			if e != nil {
				return empty, e
			}
			if !parts.Complete || !parts.Continuous {
				return empty, geom.ErrConvergence
			}
			for _, p := range parts.Pieces {
				union[p.Start.RatString()] = p.Start
				union[p.End.RatString()] = p.End
			}
		}
		// Establish useful fixed boundary cuts from the model chord proof,
		// then transport that exact same-parameter partition to both owners.
		modelOptions := options.Mesh.Boundary.Tessellation
		modelOptions.Deflection = options.Mesh.Deflection / 8
		if modelOptions.MaxSegments > join.MaxSharedVertices-1 {
			modelOptions.MaxSegments = join.MaxSharedVertices - 1
		}
		remaining := options.Mesh.Boundary.Tessellation.MaxChecks - boundaryChecks[s.FaceA]
		if remaining < 1 {
			return empty, geom.ErrConvergence
		}
		modelOptions.MaxChecks = remaining
		modelLine, e := TessellateExactQuadraticTrimCertified(ctx, faces[s.FaceA].Patch, flat[s.FaceA][s.TrimA], nil, ExactQuadraticTrimMeshOptions{Tessellation: modelOptions, Composition: join.Face.Composition, MaxInputBits: join.Face.MaxInputBits})
		if e != nil {
			return empty, e
		}
		for _, fi := range []int{s.FaceA, s.FaceB} {
			boundaryChecks[fi] += modelLine.Checks
			if boundaryChecks[fi] >= options.Mesh.Boundary.Tessellation.MaxChecks {
				return empty, geom.ErrConvergence
			}
		}
		for _, q := range modelLine.NormalizedParameters {
			union[q.RatString()] = q
		}
		if len(union) > join.MaxSharedVertices {
			return empty, geom.ErrConvergence
		}
		for _, pair := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
			fi, ti := pair[0], pair[1]
			for _, q := range union {
				v := new(big.Rat).Set(q)
				if owners[fi][ti].reverse {
					v.Sub(big.NewRat(1, 1), v)
				}
				cuts[fi][ti] = append(cuts[fi][ti], v)
			}
		}
	}
	for _, face := range used {
		for _, owned := range face {
			if !owned {
				return empty, geom.ErrInvalid
			}
		}
	}
	endpointGroups := map[QuadraticTrimSeamEndpoint]int{}
	for group, ends := range junctions {
		if len(ends) < 2 || len(ends) > len(seams) {
			return empty, geom.ErrInvalid
		}
		seen := map[int]bool{}
		for _, end := range ends {
			if end.Seam < 0 || end.Seam >= len(seams) || end.End < 0 || end.End > 1 || seen[end.Seam] {
				return empty, geom.ErrInvalid
			}
			if _, exists := endpointGroups[end]; exists {
				return empty, geom.ErrInvalid
			}
			seen[end.Seam] = true
			endpointGroups[end] = group
		}
	}
	type identity struct {
		seam      int
		parameter string
	}
	canonical := func(si int, q *big.Rat) identity {
		end := -1
		if q.Sign() == 0 {
			end = 0
		} else if q.Cmp(big.NewRat(1, 1)) == 0 {
			end = 1
		}
		if end >= 0 {
			if group, ok := endpointGroups[QuadraticTrimSeamEndpoint{Seam: si, End: end}]; ok {
				return identity{-1 - group, ""}
			}
		}
		return identity{si, q.RatString()}
	}
	spent := make([]CertifiedCurvedQuadraticTrimMesh, len(faces))
	for pass := 0; pass < join.MaxPasses; pass++ {
		result := CertifiedEmbeddedExactQuadraticShell{Agreements: agreements}
		result.Faces = make([]CertifiedCurvedQuadraticTrimMesh, len(faces))
		result.Trims = make([][]CertifiedExactQuadraticTrimPolyline, len(faces))
		result.TrimLoops = make([][]int, len(faces))
		result.TrimReversed = make([][]bool, len(faces))
		normalizedOffsets := make([][]int, len(faces))
		result.OriginalTrim = make([][]int, len(faces))
		result.SeamParameters = make([][]*big.Rat, len(seams))
		result.SeamVertices = make([][]int, len(seams))
		result.FillSquaredDistance = new(big.Rat)
		totalTriangles, totalVertices := 0, 0
		for fi, f := range faces {
			perFace := RationalCutFaceMeshOptions{Face: options, MaxInputBits: join.Face.MaxInputBits}
			perFace.Face.MaxSpanCells -= spent[fi].SpanCellChecks
			perFace.Face.MaxSpanPieces -= spent[fi].SpanPieceChecks
			perFace.Face.Mesh.MaxTriangleChecks -= spent[fi].TriangleChecks
			perFace.Face.Mesh.Boundary.Tessellation.MaxChecks -= boundaryChecks[fi]
			if perFace.Face.MaxSpanCells < 1 || perFace.Face.MaxSpanPieces < 1 || perFace.Face.Mesh.MaxTriangleChecks < 1 || perFace.Face.Mesh.Boundary.Tessellation.MaxChecks < 1 {
				return empty, geom.ErrConvergence
			}
			var loopCuts [][][]*big.Rat
			var shape [][]SplineUV
			offset := 0
			for _, loop := range loops[fi] {
				loopCuts = append(loopCuts, cuts[fi][offset:offset+len(loop)])
				shape = append(shape, make([]SplineUV, len(loop)))
				offset += len(loop)
			}
			region, e := tessellateControlledCutRegion(ctx, f.Patch, shape[0], shape[1:], loops[fi], loopCuts, perFace, join.Face.Composition, true)
			if e != nil {
				return empty, e
			}
			built := region.CertifiedCurvedQuadraticTrimMesh
			spent[fi].SpanCellChecks += built.SpanCellChecks
			spent[fi].SpanPieceChecks += built.SpanPieceChecks
			spent[fi].TriangleChecks += built.TriangleChecks
			result.Trims[fi] = make([]CertifiedExactQuadraticTrimPolyline, len(flat[fi]))
			result.TrimLoops[fi] = make([]int, len(flat[fi]))
			result.TrimReversed[fi] = make([]bool, len(flat[fi]))
			normalizedOffsets[fi] = make([]int, len(flat[fi]))
			offset = 0
			for li, lines := range region.nativeLoopTrims {
				position := 0
				for j, line := range lines {
					ti := offset + region.Boundary.Region.OriginalSource[li][j]
					result.Trims[fi][ti] = line
					result.TrimLoops[fi][ti] = li
					result.TrimReversed[fi][ti] = li > 0
					normalizedOffsets[fi][ti] = position
					position += len(line.SquaredErrors)
					boundaryChecks[fi] += line.Checks
				}
				if position != len(built.BoundaryLoops[li]) {
					return empty, geom.ErrConvergence
				}
				offset += len(loops[fi][li])
			}
			built.SpanCellChecks, built.SpanPieceChecks, built.TriangleChecks = spent[fi].SpanCellChecks, spent[fi].SpanPieceChecks, spent[fi].TriangleChecks
			result.Faces[fi] = built
			for j := range flat[fi] {
				result.OriginalTrim[fi] = append(result.OriginalTrim[fi], j)
			}
			if built.FillSquaredDistance.Cmp(result.FillSquaredDistance) > 0 {
				result.FillSquaredDistance.Set(built.FillSquaredDistance)
			}
			totalTriangles += len(built.Mesh.Triangles)
			totalVertices += len(built.Mesh.Vertices)
			if totalTriangles > o.MaxTriangles || totalVertices > o.MaxVertices {
				return empty, geom.ErrConvergence
			}
		}
		synchronized := true
		for si, s := range seams {
			union := map[string]*big.Rat{}
			for side, pair := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
				for _, q := range result.Trims[pair[0]][pair[1]].NormalizedParameters {
					v := new(big.Rat).Set(q)
					if result.TrimReversed[pair[0]][pair[1]] {
						v.Sub(big.NewRat(1, 1), v)
					}
					if side == 1 {
						v.Sub(big.NewRat(1, 1), v)
					}
					union[v.RatString()] = v
				}
			}
			if len(union) > join.MaxSharedVertices {
				return empty, geom.ErrConvergence
			}
			for _, q := range union {
				result.SeamParameters[si] = append(result.SeamParameters[si], q)
			}
			sort.Slice(result.SeamParameters[si], func(i, j int) bool { return result.SeamParameters[si][i].Cmp(result.SeamParameters[si][j]) < 0 })
			for side, pair := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
				fi, ti := pair[0], pair[1]
				if len(result.Trims[fi][ti].NormalizedParameters) != len(union) {
					synchronized = false
				}
				cuts[fi][ti] = nil
				for _, q := range result.SeamParameters[si] {
					v := new(big.Rat).Set(q)
					if side == 1 {
						v.Sub(big.NewRat(1, 1), v)
					}
					cuts[fi][ti] = append(cuts[fi][ti], v)
				}
			}
		}
		if !synchronized {
			continue
		}
		common := map[identity]int{}
		commonExact := map[identity][3]*big.Rat{}
		for fi, face := range result.Faces {
			local := map[int]identity{}
			for ti, trim := range result.Trims[fi] {
				owner := owners[fi][ti]
				for j, q := range trim.NormalizedParameters {
					parameter := new(big.Rat).Set(q)
					if result.TrimReversed[fi][ti] {
						parameter.Sub(big.NewRat(1, 1), parameter)
					}
					if owner.reverse {
						parameter.Sub(big.NewRat(1, 1), parameter)
					}
					key := canonical(owner.seam, parameter)
					li := result.TrimLoops[fi][ti]
					boundary := face.BoundaryLoops[li]
					index := (normalizedOffsets[fi][ti] + j) % len(boundary)
					if result.TrimReversed[fi][ti] {
						index = len(boundary) - 1 - index
					}
					id := boundary[index]
					if prior, exists := local[id]; exists && prior != key {
						return empty, geom.ErrInvalid
					}
					local[id] = key
				}
			}
			remap := make([]int, len(face.Mesh.Vertices))
			for id, p := range face.Mesh.Vertices {
				global := -1
				if key, shared := local[id]; shared {
					if old, exists := common[key]; exists {
						global = old
						if result.Mesh.Vertices[old] != p {
							return empty, geom.ErrConvergence
						}
						for axis := 0; axis < 3; axis++ {
							if commonExact[key][axis].Cmp(face.ExactPoints[id][axis]) != 0 {
								return empty, geom.ErrInvalid
							}
						}
					} else {
						global = len(result.Mesh.Vertices)
						common[key] = global
						commonExact[key] = face.ExactPoints[id]
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
				result.Faces[fi].OriginalFace[j] = fi
			}
		}
		for si, parameters := range result.SeamParameters {
			for _, q := range parameters {
				result.SeamVertices[si] = append(result.SeamVertices[si], common[canonical(si, q)])
			}
		}
		for group := range junctions {
			id, ok := common[identity{-1 - group, ""}]
			if !ok {
				return empty, geom.ErrInvalid
			}
			result.JunctionVertices = append(result.JunctionVertices, id)
		}
		if e := result.Mesh.ValidateSolid(); e != nil {
			return empty, e
		}
		vertices := make([]int, len(result.Mesh.Vertices))
		for i := range vertices {
			vertices[i] = i
		}
		if e := validateQuadraticJunctionFans(result.Mesh, vertices); e != nil {
			return empty, e
		}
		adjacency := make([][]int, len(vertices))
		for _, t := range result.Mesh.Triangles {
			for j, a := range t {
				b := t[(j+1)%3]
				adjacency[a] = append(adjacency[a], b)
				adjacency[b] = append(adjacency[b], a)
			}
		}
		seen := make([]bool, len(vertices))
		seen[0] = true
		stack := []int{0}
		count := 0
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			count++
			for _, next := range adjacency[id] {
				if !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
		if count != len(vertices) {
			return empty, geom.ErrConvergence
		}
		volume := new(big.Rat)
		for _, t := range result.Mesh.Triangles {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			a, b, c := exactBoundPoint(result.Mesh.Vertices[t[0]]), exactBoundPoint(result.Mesh.Vertices[t[1]]), exactBoundPoint(result.Mesh.Vertices[t[2]])
			for i := 0; i < 3; i++ {
				j, k := (i+1)%3, (i+2)%3
				cross := new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))
				volume.Add(volume, new(big.Rat).Mul(a[i], cross))
			}
		}
		volume.Quo(volume, big.NewRat(6, 1))
		if volume.Sign() <= 0 {
			return empty, geom.ErrInvalid
		}
		result.SignedDisplayVolume = volume
		report, e := collision.SelfIntersectionsBVHContext(ctx, result.Mesh, o.Collision)
		if e != nil {
			return empty, e
		}
		if !report.Complete || len(report.Contacts) > 0 {
			return empty, geom.ErrConvergence
		}
		result.DisplayCollision = report
		if e = ctx.Err(); e != nil {
			return empty, e
		}
		return result, nil
	}
	return empty, geom.ErrConvergence
}
