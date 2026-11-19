//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellRegionsMeshOptions bounds complete pair reproof, loop
// containment and the entire shared display. Mesh budgets are cumulative over
// components, including the final global collision check. Endpoint operations
// are also cumulative. Region partition subproof limits apply per loop,
// with at most MaxComponents loops. MaxComponents defaults to 8 and is capped
// at 16. Each native branch is capped at 128 segments.
type AlgebraicShellRegionsMeshOptions struct {
	Section         AlgebraicTrimmedPairSectionOptions
	Regions         AlgebraicTubeRegionOptions
	Mesh            AlgebraicTubeRegionsMeshOptions
	Endpoints       geom.ExactCurvePlaneEventOptions
	MaxComponents   int
	MaxContactPairs int // Total reserved across component and global contact passes; default 8M.
}

type CertifiedAlgebraicShellRegionsMesh struct {
	ProjectionAxes [2]int // Original model projection; all returned points are world-space.
	CertifiedAlgebraicTubeRegionsMesh
	TriangleFaces      []int // Original Boolean face IDs, not merely source chart IDs.
	ComponentBranches  []int
	TopologyVertices   []int
	BoundaryEdges      map[int][]int // Original Boolean edge -> canonical global mesh indices.
	EndpointOperations int
	ContactPairs       int
}

// TessellateDisjointLensesCertified fills an intersection whose complete
// boundary consists of hole-free, mutually disjoint lenses between the same
// two original spline graph faces in model XY, YZ or XZ projection. Full original domains are
// requalified together; no face truncation or omitted branch is permitted.
// Exact source recipes, whole edge intervals and physical face orientation
// bind the existing region fill proof to native Boolean IDs. Every component
// retains a two-sided source-image bound; exact global triangle contacts reject
// crossings/touching between displays. This proves neither a volume-error bound
// nor arbitrary disconnected Boolean fill. Unsupported/nested cases are atomic.
func (s AlgebraicShellBoolean) TessellateDisjointLensesCertified(ctx context.Context, o AlgebraicShellRegionsMeshOptions) (CertifiedAlgebraicShellRegionsMesh, error) {
	var empty CertifiedAlgebraicShellRegionsMesh
	if ctx == nil || s.source == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.Regions.MaxLoops < 0 || o.Regions.MaxLoops > 64 || o.Mesh.Component.Collision.MaxPairs < 0 || o.Mesh.Component.Collision.MaxContacts < 0 {
		return empty, geom.ErrInvalid
	}
	if o.MaxComponents == 0 {
		o.MaxComponents = 8
	}
	if o.MaxComponents < 1 || o.MaxComponents > 16 {
		return empty, geom.ErrInvalid
	}
	if o.Endpoints.MaxOperations == 0 {
		o.Endpoints.MaxOperations = 8000000
	}
	if o.Endpoints.MaxOperations < 1 || o.Endpoints.MaxOperations > 1000000000 {
		return empty, geom.ErrInvalid
	}
	n := len(s.source.branches)
	if s.op != parametric.Intersection || n < 1 || n > o.MaxComponents || len(s.faces) != 2*n {
		return empty, geom.ErrConvergence
	}
	if o.MaxContactPairs == 0 {
		o.MaxContactPairs = 8000000
	}
	if o.MaxContactPairs < 1 || o.MaxContactPairs > 1000000000 {
		return empty, geom.ErrInvalid
	}
	quota := o.MaxContactPairs / (n + 1)
	if quota < 1 {
		return empty, geom.ErrConvergence
	}
	if o.Mesh.Component.Collision.MaxPairs == 0 {
		o.Mesh.Component.Collision.MaxPairs = collision.DefaultMaxPairs
	}
	if o.Mesh.Component.Collision.MaxPairs > quota {
		o.Mesh.Component.Collision.MaxPairs = quota
	}
	pair := s.source.branches[0].Faces
	owners := make([][2]int, n)
	for i := range owners {
		owners[i] = [2]int{-1, -1}
	}
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
		if p.Branch < 0 || p.Branch >= n || p.Index < 0 || p.Index >= len(s.source.branches[p.Branch].Segments) || p.Reversed {
			return empty, geom.ErrConvergence
		}
		key := [2]int{p.Branch, p.Index}
		if _, ok := edgeIDs[key]; ok {
			return empty, geom.ErrConvergence
		}
		edgeIDs[key] = i
	}
	for fi, f := range s.faces {
		if f.Operand < 0 || f.Operand > 1 || f.Original != pair[f.Operand] || len(f.Outer) == 0 || len(f.Holes) != 0 {
			return empty, geom.ErrConvergence
		}
		branch := -1
		used := map[int]bool{}
		for _, u := range f.Outer {
			p := s.parts[0].Section[s.index[u.Edge].SectionPiece]
			if branch < 0 {
				branch = p.Branch
			}
			if p.Branch != branch || used[p.Index] {
				return empty, geom.ErrConvergence
			}
			used[p.Index] = true
		}
		if len(used) != len(s.source.branches[branch].Segments) || owners[branch][f.Operand] >= 0 {
			return empty, geom.ErrConvergence
		}
		owners[branch][f.Operand] = fi
	}
	var faces [2]Face
	var patches [2]intersect.Patch
	for i := range faces {
		if owners[0][i] < 0 {
			return empty, geom.ErrConvergence
		}
		var e error
		faces[i], e = s.OriginalFace(owners[0][i])
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
	if source == nil || len(source.segments) != n || (source.projection != [2]int{0, 1} && source.projection != [2]int{1, 2} && source.projection != [2]int{0, 2}) {
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
	if o.Regions.MaxLoops == 0 || o.Regions.MaxLoops > o.MaxComponents {
		o.Regions.MaxLoops = o.MaxComponents
	}
	var regions AlgebraicTubeRegions
	if source.projection != [2]int{0, 1} {
		branches := make([]int, n)
		for i := range branches {
			branches[i] = i
		}
		regions, e = projectedBooleanRegionsView(ctx, section, branches, false, o.Regions)
	} else {
		regions, e = section.SealAlgebraicTubeRegions(ctx, o.Regions)
	}
	if e != nil {
		return empty, e
	}
	if len(regions.regions) != n || len(regions.loops) != n {
		return empty, geom.ErrConvergence
	}
	branches := make([]int, n)
	seen := map[int]bool{}
	for i, r := range regions.regions {
		loop := regions.loops[r.Outer]
		if len(r.Holes) != 0 || loop.Parent != -1 || loop.Branch < 0 || loop.Branch >= n || seen[loop.Branch] {
			return empty, geom.ErrConvergence
		}
		seen[loop.Branch] = true
		branches[i] = loop.Branch
		lens := AlgebraicTubeLens{faces: regions.partitions[r.Outer].Faces, graphs: regions.graphs, upper: r.UpperChart}
		for chart, f := range lens.Faces() {
			id := owners[loop.Branch][chart]
			physicalReverse := f.Reversed
			if projectedAxisParity(source.projection) < 0 {
				physicalReverse = !physicalReverse
			}
			if id < 0 || physicalReverse != s.faces[id].Reversed {
				return empty, geom.ErrConvergence
			}
		}
	}
	var display CertifiedAlgebraicTubeRegionsMesh
	if source.projection != [2]int{0, 1} {
		display, e = tessellateProjectedDisjointBooleanRegions(ctx, regions, o.Mesh)
	} else {
		display, e = regions.TessellateDisjointClosedCertified(ctx, o.Mesh)
	}
	if e != nil {
		return empty, e
	}
	result := CertifiedAlgebraicShellRegionsMesh{ProjectionAxes: source.projection, CertifiedAlgebraicTubeRegionsMesh: display, ComponentBranches: branches, TopologyVertices: make([]int, len(s.vertices)), BoundaryEdges: map[int][]int{}}
	result.ContactPairs = display.Collision.BroadPhasePairs
	for _, component := range display.Components {
		result.ContactPairs += component.Collision.BroadPhasePairs
	}
	if result.ContactPairs > o.MaxContactPairs {
		return empty, geom.ErrConvergence
	}
	for i := range result.TopologyVertices {
		result.TopologyVertices[i] = -1
	}
	for i, r := range display.OriginalRegion {
		result.TriangleFaces = append(result.TriangleFaces, owners[branches[r]][display.OriginalChart[i]])
	}
	for li, loop := range display.Boundary.Loops {
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
				global := display.Boundary.LoopVertices[li][local]
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
