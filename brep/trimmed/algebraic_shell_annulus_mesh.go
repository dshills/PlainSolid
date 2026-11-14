package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellAnnulusMeshOptions bounds original-pair reproof, two-loop
// containment, and selected-region display. Endpoint work is cumulative;
// region partition subproof limits apply per loop (exactly two loops).
type AlgebraicShellAnnulusMeshOptions struct {
	Section   AlgebraicTrimmedPairSectionOptions
	Regions   AlgebraicTubeRegionOptions
	Mesh      AlgebraicTubeRegionMeshOptions
	Endpoints geom.ExactCurvePlaneEventOptions
}

type CertifiedAlgebraicShellAnnulusMesh struct {
	// Footprint.Mesh uses these two model axes; all boundary/model points remain world-space.
	// Region is -1 for projected scratch fills (there is no standalone region ID).
	ProjectionAxes [2]int
	CertifiedAlgebraicTubeRegionMesh
	TriangleFaces      []int  // Original immutable Boolean face IDs.
	LoopBranches       [2]int // Outer, then inner original section branch.
	TopologyVertices   []int
	BoundaryEdges      map[int][]int // Canonical native edge direction.
	EndpointOperations int
}

// TessellateAnnulusCertified fills a common bounded by two original affine-XY
// graph faces in model XY or cyclic YZ projection, each retaining one outer and one inner whole section loop.
// Original domains and complete pair coverage are independently requalified;
// exact recipes and intervals bind every displayed boundary to native IDs.
// The returned closed manifold has positive displayed volume and no exact
// nonincidence triangle contacts. Its two-sided source-image distance bound
// does not imply a volume-error bound or embedding of the original source.
// Unsupported partial loops, extra components and retained source edges fail
// atomically. No disjointness of the unselected nested disk is asserted.
func (s AlgebraicShellBoolean) TessellateAnnulusCertified(ctx context.Context, o AlgebraicShellAnnulusMeshOptions) (CertifiedAlgebraicShellAnnulusMesh, error) {
	var empty CertifiedAlgebraicShellAnnulusMesh
	if ctx == nil || s.source == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.Regions.MaxLoops < 0 || o.Regions.MaxLoops > 64 {
		return empty, geom.ErrInvalid
	}
	if o.Regions.MaxLoops == 1 {
		return empty, geom.ErrConvergence
	}
	if _, e := newTubeLensSpanWork(o.Mesh.Surface); e != nil {
		return empty, e
	}
	if o.Endpoints.MaxOperations == 0 {
		o.Endpoints.MaxOperations = 8000000
	}
	if o.Endpoints.MaxOperations < 1 || o.Endpoints.MaxOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	if s.op != parametric.Intersection || len(s.faces) != 2 || len(s.source.branches) != 2 {
		return empty, geom.ErrConvergence
	}
	pair := s.source.branches[0].Faces
	total := 0
	for _, b := range s.source.branches {
		if !b.Closed || b.Faces != pair || len(b.Segments) < 3 || len(b.Segments) > 128 {
			return empty, geom.ErrConvergence
		}
		total += len(b.Segments)
	}
	if len(s.index) != total || len(s.vertices) != total {
		return empty, geom.ErrConvergence
	}
	edgeIDs := map[[2]int]int{}
	for i, e := range s.index {
		if e.OriginalSubdivisionEdge >= 0 || e.SectionPiece < 0 {
			return empty, geom.ErrConvergence
		}
		p := s.parts[0].Section[e.SectionPiece]
		if p.Branch < 0 || p.Branch >= 2 || p.Index < 0 || p.Index >= len(s.source.branches[p.Branch].Segments) || p.Reversed {
			return empty, geom.ErrConvergence
		}
		key := [2]int{p.Branch, p.Index}
		if _, ok := edgeIDs[key]; ok {
			return empty, geom.ErrConvergence
		}
		edgeIDs[key] = i
	}
	owners := [2]int{-1, -1}
	loopBranches := [2]int{-1, -1}
	for fi, f := range s.faces {
		if f.Operand < 0 || f.Operand > 1 || owners[f.Operand] >= 0 || f.Original != pair[f.Operand] || len(f.Holes) != 1 {
			return empty, geom.ErrConvergence
		}
		owners[f.Operand] = fi
		for li, row := range [][]AlgebraicShellBooleanUse{f.Outer, f.Holes[0]} {
			if len(row) < 3 {
				return empty, geom.ErrConvergence
			}
			first := s.parts[0].Section[s.index[row[0].Edge].SectionPiece]
			branch := first.Branch
			n := len(s.source.branches[branch].Segments)
			if len(row) != n {
				return empty, geom.ErrConvergence
			}
			if loopBranches[li] < 0 {
				loopBranches[li] = branch
			}
			if loopBranches[li] != branch {
				return empty, geom.ErrConvergence
			}
			for j, u := range row {
				p := s.parts[0].Section[s.index[u.Edge].SectionPiece]
				want := (first.Index + j) % n
				if row[0].Reversed {
					want = (first.Index + n - j) % n
				}
				if p.Branch != branch || p.Index != want || u.Reversed != row[0].Reversed {
					return empty, geom.ErrConvergence
				}
			}
		}
	}
	if loopBranches[0] == loopBranches[1] {
		return empty, geom.ErrConvergence
	}
	var faces [2]Face
	var patches [2]intersect.Patch
	for i := range faces {
		if owners[i] < 0 {
			return empty, geom.ErrConvergence
		}
		var e error
		faces[i], e = s.OriginalFace(owners[i])
		if e != nil {
			return empty, e
		}
		if !supportsExactSplinePatch(faces[i].Surface) {
			return empty, geom.ErrConvergence
		}
		d, e := faceUVBounds(faces[i])
		if e != nil {
			return empty, e
		}
		patches[i], e = algebraicShellTracePatch(faces[i], d)
		if e != nil {
			return empty, e
		}
	}
	pi := -1
	for i, p := range s.source.pairs {
		if p == pair {
			if pi >= 0 {
				return empty, geom.ErrConvergence
			}
			pi = i
		}
	}
	if pi < 0 {
		return empty, geom.ErrConvergence
	}
	seed := s.source.sections[pi].Trace.Seeds
	section, e := SectionTrimmedSplinePair(ctx, patches, faces, &seed, o.Section)
	if e != nil {
		return empty, e
	}
	source := section.runSource()
	if source == nil || len(source.segments) != 2 || (source.projection != [2]int{0, 1} && source.projection != [2]int{1, 2}) {
		return empty, geom.ErrConvergence
	}
	for i, b := range source.segments {
		if len(b) != len(s.source.branches[i].Segments) {
			return empty, geom.ErrConvergence
		}
		for j, p := range b {
			if !sameShellMeshTube(p, s.source.branches[i].Segments[j]) {
				return empty, geom.ErrConvergence
			}
		}
	}

	if o.Regions.MaxLoops == 0 || o.Regions.MaxLoops > 2 {
		o.Regions.MaxLoops = 2
	}
	var regions AlgebraicTubeRegions
	if source.projection == [2]int{1, 2} {
		regions, e = projectedBooleanAnnulusView(ctx, section, loopBranches, o.Regions)
	} else {
		regions, e = section.SealAlgebraicTubeRegions(ctx, o.Regions)
	}
	if e != nil {
		return empty, e
	}
	if len(regions.regions) != 2 || len(regions.loops) != 2 {
		return empty, geom.ErrConvergence
	}
	selected := -1
	for i, r := range regions.regions {
		if len(r.Holes) != 1 {
			continue
		}
		if regions.loops[r.Outer].Branch != loopBranches[0] || regions.loops[r.Holes[0]].Branch != loopBranches[1] || regions.loops[r.Outer].Parent != -1 || regions.loops[r.Holes[0]].Parent != r.Outer {
			return empty, geom.ErrConvergence
		}
		if selected >= 0 {
			return empty, geom.ErrConvergence
		}
		selected = i
	}
	if selected < 0 {
		return empty, geom.ErrConvergence
	}
	for chart, f := range regions.RegionFaces(selected) {
		if owners[chart] < 0 || f.Reversed != s.faces[owners[chart]].Reversed {
			return empty, geom.ErrConvergence
		}
	}
	var display CertifiedAlgebraicTubeRegionMesh
	if source.projection == [2]int{1, 2} {
		display, e = tessellateProjectedBooleanAnnulus(ctx, regions, o.Mesh)
	} else {
		display, e = regions.TessellateRegionClosedCertified(ctx, selected, o.Mesh)
	}
	if e != nil {
		return empty, e
	}
	result := CertifiedAlgebraicShellAnnulusMesh{ProjectionAxes: source.projection, CertifiedAlgebraicTubeRegionMesh: display, LoopBranches: loopBranches, TopologyVertices: make([]int, len(s.vertices)), BoundaryEdges: map[int][]int{}}
	for i := range result.TopologyVertices {
		result.TopologyVertices[i] = -1
	}
	for _, chart := range display.OriginalChart {
		result.TriangleFaces = append(result.TriangleFaces, owners[chart])
	}
	for li, loop := range display.Footprint.Boundary.Loops {
		branch := regions.loops[li].Branch
		for pieceIndex, piece := range loop.Run.Partition.Cut {
			original := -1
			for j, p := range s.source.branches[branch].Segments {
				if sameShellMeshTube(p, piece.Segment) {
					if original >= 0 {
						return empty, geom.ErrConvergence
					}
					original = j
				}
			}
			if original < 0 {
				return empty, geom.ErrConvergence
			}
			edge := edgeIDs[[2]int{branch, original}]
			if _, ok := result.BoundaryEdges[edge]; ok {
				return empty, geom.ErrConvergence
			}
			_, domain, reverse, e := piece.Segment.ExactFreeDomain()
			if e != nil {
				return empty, e
			}
			var ends [2]*big.Rat
			for i, root := range []*geom.ExactAlgebraicRoot{piece.FreeStart, piece.FreeEnd} {
				opts := o.Endpoints
				opts.MaxOperations -= result.EndpointOperations
				if opts.MaxOperations < 1 {
					return empty, geom.ErrConvergence
				}
				c, e := root.Classify(ctx, opts)
				result.EndpointOperations += c.Operations
				if e != nil {
					return empty, e
				}
				if c.Kind != geom.ExactRootKindRational {
					return empty, geom.ErrConvergence
				}
				ends[i] = c.Rational
			}
			a, b := domain.Lower, domain.Upper
			if reverse {
				a, b = b, a
			}
			back := ends[0].Cmp(b) == 0 && ends[1].Cmp(a) == 0
			if !back && (ends[0].Cmp(a) != 0 || ends[1].Cmp(b) != 0) {
				return empty, geom.ErrConvergence
			}
			var path []int
			for _, local := range loop.Run.PieceVertices[pieceIndex] {
				network := display.Footprint.Boundary.LoopVertices[li][local]
				global, ok := display.GlobalBoundaryVertices[network]
				if !ok {
					return empty, geom.ErrConvergence
				}
				if display.Mesh.Vertices[global] != loop.Run.Vertices[local] {
					return empty, geom.ErrConvergence
				}
				path = append(path, global)
			}
			if back {
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
			}
			if len(path) < 2 {
				return empty, geom.ErrConvergence
			}
			for i, v := range []int{s.index[edge].Start, s.index[edge].End} {
				index := path[0]
				if i == 1 {
					index = path[len(path)-1]
				}
				if result.TopologyVertices[v] >= 0 && result.TopologyVertices[v] != index {
					return empty, geom.ErrConvergence
				}
				result.TopologyVertices[v] = index
			}
			result.BoundaryEdges[edge] = path
		}
	}
	if len(result.BoundaryEdges) != total {
		return empty, geom.ErrConvergence
	}
	unique := map[int]bool{}
	for _, v := range result.TopologyVertices {
		if v < 0 || unique[v] {
			return empty, geom.ErrConvergence
		}
		unique[v] = true
	}
	return result, nil
}
