package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellLensMeshOptions independently bounds pair reproof, graph-lens
// sealing and display construction. This bridge accepts at most 128 complete
// section segments; it never expands the existing lens mesher's support domain.
type AlgebraicShellLensMeshOptions struct {
	Section AlgebraicTrimmedPairSectionOptions
	Lens    AlgebraicTubeLensOptions
	Mesh    MultiSpanTubeLensMeshOptions
}

type CertifiedAlgebraicShellLensMesh struct {
	CertifiedAlgebraicTubeLensMesh
	SourceFaces      [2][2]int     // Mesh face index -> operand, original face.
	TopologyVertices []int         // Boolean vertex -> shared mesh index.
	BoundaryEdges    map[int][]int // Boolean edge -> shared mesh indices, canonical traversal.
}

// TessellateLensCertified fills an intersection consisting solely of two graph
// faces bounded by one complete closed section. Both original face domains are
// rechecked, then every tube recipe is matched exactly to immutable Boolean
// evidence before reusing the projected lens proof. Surface orientation must
// agree with the original material result. The returned two-sided source-image
// distance includes boundary homotopy, chart transport, triangle residual and
// rounding. Closed manifold incidence, positive exact displayed volume and
// absence of nonincident triangle contacts are mandatory. Volume approximation
// itself has no certified error bound. Source edges, holes, multiple branches,
// non-B-spline supports and unsupported graph charts fail without partial output.
func (s AlgebraicShellBoolean) TessellateLensCertified(ctx context.Context, o AlgebraicShellLensMeshOptions) (CertifiedAlgebraicShellLensMesh, error) {
	var empty CertifiedAlgebraicShellLensMesh
	if ctx == nil || s.source == nil {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if s.op != parametric.Intersection || len(s.faces) != 2 || len(s.source.branches) != 1 {
		return empty, fmt.Errorf("material topology: %w", geom.ErrConvergence)
	}
	branch := s.source.branches[0]
	n := len(branch.Segments)
	if !branch.Closed || n < 3 || n > 128 || len(s.index) != n || len(s.vertices) != n {
		return empty, fmt.Errorf("closed branch size: %w", geom.ErrConvergence)
	}
	var faces [2]Face
	var patches [2]intersect.Patch
	for i, f := range s.faces {
		if f.Operand != i || f.Original != branch.Faces[i] || len(f.Outer) != n || len(f.Holes) != 0 {
			return empty, fmt.Errorf("original face ownership: %w", geom.ErrConvergence)
		}
		var e error
		faces[i], e = s.OriginalFace(i)
		if e != nil {
			return empty, e
		}
		if _, ok := faces[i].Surface.(*geom.BSplineSurface); !ok {
			return empty, fmt.Errorf("unsupported exact support: %w", geom.ErrConvergence)
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
	originalEdges := make([]int, n)
	seen := make([]bool, n)
	for ei, edge := range s.index {
		if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 {
			return empty, fmt.Errorf("retained source edge: %w", geom.ErrConvergence)
		}
		p := s.parts[0].Section[edge.SectionPiece]
		if p.Branch != 0 || p.Reversed || p.Index < 0 || p.Index >= n || seen[p.Index] {
			return empty, fmt.Errorf("section edge ownership: %w", geom.ErrConvergence)
		}
		seen[p.Index] = true
		originalEdges[p.Index] = ei
	}
	pair := -1
	for i, p := range s.source.pairs {
		if p == branch.Faces {
			if pair >= 0 {
				return empty, fmt.Errorf("ambiguous face pair: %w", geom.ErrConvergence)
			}
			pair = i
		}
	}
	if pair < 0 || pair >= len(s.source.sections) {
		return empty, fmt.Errorf("missing original face pair: %w", geom.ErrConvergence)
	}
	seeds := s.source.sections[pair].Trace.Seeds
	section, e := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, o.Section)
	if e != nil {
		return empty, fmt.Errorf("pair reproof: %w", e)
	}
	if section.partitionSource == nil || len(section.partitionSource.segments) != 1 || len(section.partitionSource.segments[0]) != n {
		return empty, fmt.Errorf("reproved branch count: %w", geom.ErrConvergence)
	}
	for i, p := range section.partitionSource.segments[0] {
		if !sameShellMeshTube(p, branch.Segments[i]) {
			return empty, fmt.Errorf("changed original tube recipe: %w", geom.ErrConvergence)
		}
	}
	lens, e := section.SealProjectedAlgebraicTubeLens(ctx, o.Lens)
	if e != nil {
		return empty, fmt.Errorf("lens sealing: %w", e)
	}
	for i, f := range lens.Faces() {
		if f.Reversed != s.faces[i].Reversed {
			return empty, fmt.Errorf("material face orientation: %w", geom.ErrConvergence)
		}
	}
	display, e := lens.TessellateClosedCertified(ctx, o.Mesh)
	if e != nil {
		return empty, fmt.Errorf("lens fill: %w", e)
	}
	result := CertifiedAlgebraicShellLensMesh{CertifiedAlgebraicTubeLensMesh: display, BoundaryEdges: map[int][]int{}, TopologyVertices: make([]int, len(s.vertices))}
	for i := range result.TopologyVertices {
		result.TopologyVertices[i] = -1
	}
	for i, f := range s.faces {
		result.SourceFaces[i] = [2]int{f.Operand, f.Original}
	}
	for pi, piece := range display.Boundary.Run.Partition.Cut {
		index := -1
		for j, p := range branch.Segments {
			if sameShellMeshTube(piece.Segment, p) {
				if index >= 0 {
					return empty, fmt.Errorf("ambiguous cut tube: %w", geom.ErrConvergence)
				}
				index = j
			}
		}
		if index < 0 {
			return empty, fmt.Errorf("missing cut tube: %w", geom.ErrConvergence)
		}
		edge := originalEdges[index]
		if _, ok := result.BoundaryEdges[edge]; ok {
			return empty, fmt.Errorf("repeated cut tube: %w", geom.ErrConvergence)
		}
		_, domain, reverse, e := piece.Segment.ExactFreeDomain()
		if e != nil {
			return empty, e
		}
		roots := [2]*geom.ExactAlgebraicRoot{piece.FreeStart, piece.FreeEnd}
		var values [2]*big.Rat
		for j, root := range roots {
			c, e := root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || c.Kind != geom.ExactRootKindRational {
				if e != nil {
					return empty, e
				}
				return empty, fmt.Errorf("nonrational whole-tube endpoint: %w", geom.ErrConvergence)
			}
			values[j] = c.Rational
		}
		a, b := domain.Lower, domain.Upper
		if reverse {
			a, b = b, a
		}
		reversed := values[0].Cmp(b) == 0 && values[1].Cmp(a) == 0
		if !reversed && (values[0].Cmp(a) != 0 || values[1].Cmp(b) != 0) {
			return empty, fmt.Errorf("partial original tube domain: %w", geom.ErrConvergence)
		}
		var path []int
		for _, id := range display.Boundary.Run.PieceVertices[pi] {
			v, ok := display.BoundaryVertices[id]
			if !ok {
				return empty, fmt.Errorf("missing shared mesh vertex: %w", geom.ErrConvergence)
			}
			path = append(path, v)
		}
		if reversed {
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
		}
		if len(path) < 2 {
			return empty, geom.ErrConvergence
		}
		for j, original := range []int{s.index[edge].Start, s.index[edge].End} {
			vertex := path[0]
			if j == 1 {
				vertex = path[len(path)-1]
			}
			if result.TopologyVertices[original] >= 0 && result.TopologyVertices[original] != vertex {
				return empty, geom.ErrConvergence
			}
			result.TopologyVertices[original] = vertex
		}
		result.BoundaryEdges[edge] = path
	}
	if len(result.BoundaryEdges) != n {
		return empty, fmt.Errorf("incomplete native edge ancestry: %w", geom.ErrConvergence)
	}
	vertices := map[int]bool{}
	for _, v := range result.TopologyVertices {
		if v < 0 || vertices[v] {
			return empty, geom.ErrConvergence
		}
		vertices[v] = true
	}
	return result, nil
}

// Both operands already carry independent tube proofs. Equal original chart
// recipes, boxes, free axes and traversal select the same unique branch. Proof
// budgets are deliberately excluded; they do not describe source geometry.
func sameShellMeshTube(a, b intersect.TubeSegment) bool {
	ar, ai, _ := a.TrimA().Recipe()
	br, bi, _ := b.TrimA().Recipe()
	if ai != bi {
		return false
	}
	ad, bd := ar.Data(), br.Data()
	if ad.A.Surface == nil || bd.A.Surface == nil {
		return false
	}
	ad.Options = intersect.TubeCurveOptions{}
	bd.Options = intersect.TubeCurveOptions{}
	return reflect.DeepEqual(ad, bd)
}
