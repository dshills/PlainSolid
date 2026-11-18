//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"sort"
)

// AlgebraicShellRegionCollectionMeshOptions bounds original pair coverage,
// original region containment, shared display work and cumulative endpoint proof.
// The source scope is one complete XY graph pair with at most sixteen loops.
type AlgebraicShellRegionCollectionMeshOptions struct {
	Section   AlgebraicTrimmedPairSectionOptions
	Regions   AlgebraicTubeRegionOptions
	Mesh      AlgebraicTubeRegionCollectionMeshOptions
	Endpoints geom.ExactCurvePlaneEventOptions
}

type CertifiedAlgebraicShellRegionCollectionMesh struct {
	CertifiedAlgebraicTubeRegionCollectionMesh
	TriangleFaces      []int
	ComponentFaces     [][2]int
	ComponentBranches  [][]int
	TopologyVertices   []int
	BoundaryEdges      map[int][]int // original native edge direction, not display winding
	EndpointOperations int
}

// TessellateTubeRegionComponentsCertified independently reconstructs complete
// original graph regions and binds each selected disk or annulus to immutable
// native face, edge and vertex owners. Only whole original section loops and
// original XY graph charts qualify. Shared zero-thickness rims reject. Every
// component and the combined display must be closed, positively oriented and
// free of exact nonincidence contacts. The returned source-image error is not a
// volume-error or a general source embedding certificate. Unsupported domains
// and exhausted work return an empty result, never a partial component list.
func (s AlgebraicShellBoolean) TessellateTubeRegionComponentsCertified(ctx context.Context, o AlgebraicShellRegionCollectionMeshOptions) (CertifiedAlgebraicShellRegionCollectionMesh, error) {
	empty := CertifiedAlgebraicShellRegionCollectionMesh{}
	layouts, e := s.nativeTubeRegions(ctx)
	if e != nil {
		return empty, e
	}
	if len(layouts) == 0 || len(layouts) > 8 || len(s.source.branches) == 0 || len(s.source.branches) > 16 {
		return empty, geom.ErrConvergence
	}
	if o.Regions.MaxLoops < 0 || o.Regions.MaxLoops > 64 {
		return empty, geom.ErrInvalid
	}
	if o.Endpoints.MaxOperations == 0 {
		o.Endpoints.MaxOperations = 8000000
	}
	if o.Endpoints.MaxOperations < 1 || o.Endpoints.MaxOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	if o.Section.MaxSegments == 0 {
		o.Section.MaxSegments = 512
	}
	pair := s.source.branches[0].Faces
	for _, b := range s.source.branches {
		if b.Faces != pair || !b.Closed || len(b.Segments) < 3 || len(b.Segments) > 256 {
			return empty, geom.ErrConvergence
		}
	}
	retainedBranches := map[int]bool{}
	for _, l := range layouts {
		for chart, owner := range l.Owners {
			if owner < 0 || owner >= len(s.faces) || s.faces[owner].Operand != chart || s.faces[owner].Original != pair[chart] {
				return empty, geom.ErrConvergence
			}
		}
		for _, b := range l.Branches {
			if retainedBranches[b] {
				return empty, geom.ErrConvergence
			}
			retainedBranches[b] = true
		}
	}
	edgeIDs := map[[2]int]int{}
	for i, edge := range s.index {
		if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 || edge.SectionPiece >= len(s.parts[0].Section) {
			return empty, geom.ErrConvergence
		}
		p := s.parts[0].Section[edge.SectionPiece]
		if !retainedBranches[p.Branch] || p.Reversed || p.Index < 0 || p.Index >= len(s.source.branches[p.Branch].Segments) {
			return empty, geom.ErrConvergence
		}
		key := [2]int{p.Branch, p.Index}
		if _, ok := edgeIDs[key]; ok {
			return empty, geom.ErrConvergence
		}
		edgeIDs[key] = i
	}
	var faces [2]Face
	var patches [2]intersect.Patch
	for i := range faces {
		faces[i], e = s.OriginalFace(layouts[0].Owners[i])
		if e != nil {
			return empty, e
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
	if source == nil || source.projection != [2]int{0, 1} || len(source.segments) != len(s.source.branches) {
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
	if o.Regions.MaxLoops == 0 || o.Regions.MaxLoops > 16 {
		o.Regions.MaxLoops = 16
	}
	regions, e := section.SealAlgebraicTubeRegions(ctx, o.Regions)
	if e != nil {
		return empty, e
	}
	layoutByRegion := map[int]int{}
	selected := []int{}
	for li, l := range layouts {
		found := -1
		for ri, r := range regions.regions {
			if regions.loops[r.Outer].Branch != l.Branches[0] || len(r.Holes) != len(l.Branches)-1 {
				continue
			}
			holes := make([]int, len(r.Holes))
			for i, h := range r.Holes {
				holes[i] = regions.loops[h].Branch
			}
			sort.Ints(holes)
			same := true
			for i, b := range holes {
				if b != l.Branches[i+1] {
					same = false
				}
			}
			if !same {
				continue
			}
			if found >= 0 {
				return empty, geom.ErrConvergence
			}
			found = ri
		}
		if found < 0 {
			return empty, geom.ErrConvergence
		}
		if _, ok := layoutByRegion[found]; ok {
			return empty, geom.ErrConvergence
		}
		for chart, f := range regions.RegionFaces(found) {
			if f.Reversed != s.faces[l.Owners[chart]].Reversed {
				return empty, geom.ErrConvergence
			}
		}
		layoutByRegion[found] = li
		selected = append(selected, found)
	}
	sort.Ints(selected)
	display, e := regions.TessellateSelectedRegionsClosedCertified(ctx, selected, o.Mesh)
	if e != nil {
		return empty, e
	}
	result := CertifiedAlgebraicShellRegionCollectionMesh{CertifiedAlgebraicTubeRegionCollectionMesh: display, TopologyVertices: make([]int, len(s.vertices)), BoundaryEdges: map[int][]int{}}
	for i := range result.TopologyVertices {
		result.TopologyVertices[i] = -1
	}
	for _, ri := range selected {
		l := layouts[layoutByRegion[ri]]
		result.ComponentFaces = append(result.ComponentFaces, l.Owners)
		result.ComponentBranches = append(result.ComponentBranches, append([]int(nil), l.Branches...))
	}
	for i, chart := range display.OriginalChart {
		l := layouts[layoutByRegion[display.OriginalRegion[i]]]
		result.TriangleFaces = append(result.TriangleFaces, l.Owners[chart])
	}
	for li, loop := range display.Boundary.Loops {
		branch := regions.loops[li].Branch
		if !retainedBranches[branch] {
			continue
		}
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
			edge, exists := edgeIDs[[2]int{branch, original}]
			if !exists {
				return empty, geom.ErrConvergence
			}
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
				network := display.Boundary.LoopVertices[li][local]
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
	if len(result.BoundaryEdges) != len(s.index) {
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
