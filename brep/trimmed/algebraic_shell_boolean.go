package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type AlgebraicShellBooleanOptions struct {
	Imprint AlgebraicShellImprintOptions
	Shells  SplineShellBooleanOptions
}

type AlgebraicShellBooleanFace struct {
	Operand, Original, Fragment int
	Reversed                    bool
	Outer                       []AlgebraicShellBooleanUse
	Holes                       [][]AlgebraicShellBooleanUse
}

type AlgebraicShellBooleanUse struct {
	Edge     int
	Reversed bool
}

type AlgebraicShellBooleanEdge struct {
	Start, End, Operand, OriginalSubdivisionEdge, SectionPiece int
}

type AlgebraicShellBooleanVertex struct {
	Operand, OriginalSubdivisionVertex int
	SectionPoint                       *geom.ExactComposedCurveEvent
}

// AlgebraicShellBoolean is the exact indexed boundary of a qualified regular
// Boolean. Unsplit and split original edges retain their original supports;
// new section edges are one pair of implicit trims with shared original
// free-coordinate parameters. It is separate from the float-only Solid API.
type AlgebraicShellBoolean struct {
	Topology AlgebraicSubdivisionTopology
	Vertices []AlgebraicShellBooleanVertex
	Edges    []AlgebraicShellBooleanEdge
	Faces    []AlgebraicShellBooleanFace
	source   *algebraicShellSectionSource
	op       parametric.BooleanOp
	parts    [2]AlgebraicShellImprint
	index    []AlgebraicShellBooleanEdge
	faces    []AlgebraicShellBooleanFace
	vertices []AlgebraicShellBooleanVertex
}

// OriginalFace retrieves the original, unsplit support and trim data for one
// retained fragment. Faces supplies the exact new topology on that support.
func (s AlgebraicShellBoolean) OriginalFace(face int) (Face, error) {
	if s.source == nil || face < 0 || face >= len(s.faces) {
		return Face{}, geom.ErrInvalid
	}
	f := s.faces[face]
	return cloneFace([2]Solid{s.source.a, s.source.b}[f.Operand].faces[f.Original]), nil
}

// SourceCurveCells restricts an original model edge to the exact retained
// subdomain. Section edges use SectionSegment instead.
func (s AlgebraicShellBoolean) SourceCurveCells(ctx context.Context, edge int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	if s.source == nil || edge < 0 || edge >= len(s.index) || s.index[edge].OriginalSubdivisionEdge < 0 {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	x := s.index[edge]
	return s.parts[x.Operand].Subdivision.ModelCurveCells(ctx, x.OriginalSubdivisionEdge, o)
}

// SectionSegment returns the shared exact tube and its original face pair for
// a generated model edge. The edge traverses the segment's canonical order.
func (s AlgebraicShellBoolean) SectionSegment(edge int) (intersect.TubeSegment, [2]int, error) {
	if s.source == nil || edge < 0 || edge >= len(s.index) || s.index[edge].SectionPiece < 0 {
		return intersect.TubeSegment{}, [2]int{}, geom.ErrInvalid
	}
	x := s.parts[0].Section[s.index[edge].SectionPiece]
	b := s.source.branches[x.Branch]
	return b.Segments[x.Index], b.Faces, nil
}

// CombineAlgebraic assembles the regularized Boolean of a completed section.
// Original-shell embedding, exhaustive regular intersection and exact imprint
// provide the contact proof. A certified ray classifies one original vertex on
// each shell, and regular section edges toggle that classification. Paired
// section edges sew by exact identity. No geometric samples identify vertices.
func (r AlgebraicShellSectionResult) CombineAlgebraic(ctx context.Context, op parametric.BooleanOp,
	o AlgebraicShellBooleanOptions) (AlgebraicShellBoolean, error) {
	var empty AlgebraicShellBoolean
	if ctx == nil || r.source == nil || op > parametric.Difference || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return empty, geom.ErrInvalid
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	parts, e := r.ImprintOriginalShells(ctx, o.Imprint)
	if e != nil {
		return empty, e
	}
	var inside [2][]bool
	for i := range parts {
		inside[i], e = classifyAlgebraicImprint(ctx, parts[i], parts[i^1].Subdivision.source, o.Shells)
		if e != nil {
			return empty, fmt.Errorf("trimmed: classify algebraic shell %d: %w", i, e)
		}
	}
	out, e := sewAlgebraicImprint(ctx, r.source, parts, inside, op)
	if e != nil {
		return empty, fmt.Errorf("trimmed: sew algebraic shell Boolean: %w", e)
	}
	return out, nil
}

func classifyAlgebraicImprint(ctx context.Context, p AlgebraicShellImprint, other Solid, o SplineShellBooleanOptions) ([]bool, error) {
	state := make([]int8, len(p.Faces))
	inside := make([]bool, len(p.Faces))
	if len(p.Faces) == 0 || len(other.faces) == 0 {
		return inside, nil
	}
	inc := make([][2]int, len(p.Edges))
	count := make([]int, len(p.Edges))
	var bounds geom.Box
	for i, f := range other.faces {
		x, e := exactFaceSupportPatch(f, o.Embedding.Patches)
		if e != nil {
			return nil, e
		}
		b, e := x.Bounds()
		if e != nil {
			return nil, e
		}
		if i == 0 {
			bounds = b
		} else {
			bounds, e = bounds.Union(b)
			if e != nil {
				return nil, e
			}
		}
	}
	seed := -1
	for i, f := range p.Faces {
		for _, loop := range append([][]AlgebraicShellImprintUse{f.Outer}, f.Holes...) {
			for _, u := range loop {
				id := p.edgeID(u)
				if count[id] == 2 {
					return nil, geom.ErrConvergence
				}
				inc[id][count[id]] = i
				count[id]++
				v, _ := p.edgeEnds(u)
				original := p.Vertices[v].OriginalVertex
				if seed >= 0 || original < 0 || original >= len(p.Subdivision.source.vertices) {
					continue
				}
				q, e := classifySeparatedSplineWitness(ctx, other, bounds, p.Subdivision.source.vertices[original], o)
				if e != nil {
					return nil, e
				}
				state[i] = -1
				if q {
					state[i] = 1
				}
				seed = i
			}
		}
	}
	if seed < 0 {
		return nil, geom.ErrConvergence
	}
	type neighbor struct {
		face int
		flip bool
	}
	adj := make([][]neighbor, len(p.Faces))
	for id, pair := range inc {
		a, b := pair[0], pair[1]
		if count[id] != 2 || a == b {
			return nil, geom.ErrConvergence
		}
		cross := id >= len(p.Subdivision.edges)
		adj[a] = append(adj[a], neighbor{b, cross})
		adj[b] = append(adj[b], neighbor{a, cross})
	}
	queue := []int{seed}
	for i := 0; i < len(queue); i++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		f := queue[i]
		for _, n := range adj[f] {
			want := state[f]
			if n.flip {
				want = -want
			}
			if state[n.face] == 0 {
				state[n.face] = want
				queue = append(queue, n.face)
			} else if state[n.face] != want {
				return nil, geom.ErrConvergence
			}
		}
	}
	for i, v := range state {
		if v == 0 {
			return nil, geom.ErrConvergence
		}
		inside[i] = v > 0
	}
	return inside, nil
}

func sewAlgebraicImprint(ctx context.Context, src *algebraicShellSectionSource, p [2]AlgebraicShellImprint,
	inside [2][]bool, op parametric.BooleanOp) (AlgebraicShellBoolean, error) {
	var empty AlgebraicShellBoolean
	out := AlgebraicShellBoolean{source: src, op: op, parts: p}
	if len(p[0].Section) != len(p[1].Section) {
		return empty, geom.ErrConvergence
	}
	// Merge the local vertex indices of the two imprints only where a
	// canonical paired section edge identifies its exact same endpoint.
	base := len(p[0].Vertices)
	parent := make([]int, base+len(p[1].Vertices))
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for i, piece := range p[0].Section {
		if piece != p[1].Section[i] {
			return empty, geom.ErrConvergence
		}
		a, b := p[0].Edges[len(p[0].Subdivision.edges)+i], p[1].Edges[len(p[1].Subdivision.edges)+i]
		for j, x := range [2]int{a.Start, a.End} {
			y := [2]int{b.Start, b.End}[j] + base
			if y = find(y); y != find(x) {
				parent[y] = find(x)
			}
		}
	}
	vertices := map[int]int{}
	vertex := func(operand, id int) int {
		k := id
		if operand == 1 {
			k += base
		}
		key := find(k)
		if v, ok := vertices[key]; ok {
			return v
		}
		v := p[operand].Vertices[id]
		index := len(out.Vertices)
		vertices[key] = index
		out.Vertices = append(out.Vertices, AlgebraicShellBooleanVertex{operand, v.OriginalVertex, v.SectionPoint})
		return index
	}
	edges := map[[2]int]int{}
	var check AlgebraicShellImprint
	for operand, q := range p {
		for i, f := range q.Faces {
			if e := ctx.Err(); e != nil {
				return empty, e
			}
			want := op == parametric.Intersection || op == parametric.Difference && operand == 1
			if inside[operand][i] != want {
				continue
			}
			flipped := f.Reversed != (op == parametric.Difference && operand == 1)
			x := AlgebraicShellBooleanFace{Operand: operand, Original: f.Original, Fragment: f.Fragment, Reversed: flipped}
			y := AlgebraicShellImprintFace{Reversed: flipped}
			for k, ring := range append([][]AlgebraicShellImprintUse{f.Outer}, f.Holes...) {
				var outer []AlgebraicShellBooleanUse
				var checkRing []AlgebraicShellImprintUse
				for _, u := range ring {
					id := q.edgeID(u)
					key := [2]int{operand, id}
					if u.SectionPiece >= 0 {
						key = [2]int{-1, u.SectionPiece}
					}
					a, b := vertex(operand, q.Edges[id].Start), vertex(operand, q.Edges[id].End)
					newID, ok := edges[key]
					if !ok {
						newID = len(out.Edges)
						edges[key] = newID
						out.Edges = append(out.Edges, AlgebraicShellBooleanEdge{a, b, operand, u.OriginalEdge, u.SectionPiece})
						check.Edges = append(check.Edges, AlgebraicShellImprintEdge{a, b})
					} else if out.Edges[newID].Start != a || out.Edges[newID].End != b {
						return empty, geom.ErrConvergence
					}
					outer = append(outer, AlgebraicShellBooleanUse{newID, u.Reversed})
					checkRing = append(checkRing, AlgebraicShellImprintUse{OriginalEdge: newID, SectionPiece: -1, Reversed: u.Reversed})
				}
				if k == 0 {
					x.Outer, y.Outer = outer, checkRing
				} else {
					x.Holes, y.Holes = append(x.Holes, outer), append(y.Holes, checkRing)
				}
			}
			out.Faces = append(out.Faces, x)
			check.Faces = append(check.Faces, y)
		}
	}
	check.Vertices = make([]AlgebraicShellImprintVertex, len(out.Vertices))
	if e := check.validateIncidence(ctx, false); e != nil {
		return empty, e
	}
	out.Topology = check.Topology
	out.index = append([]AlgebraicShellBooleanEdge(nil), out.Edges...)
	out.vertices = append([]AlgebraicShellBooleanVertex(nil), out.Vertices...)
	for _, f := range out.Faces {
		q := f
		q.Outer = append([]AlgebraicShellBooleanUse(nil), f.Outer...)
		q.Holes = make([][]AlgebraicShellBooleanUse, len(f.Holes))
		for j, ring := range f.Holes {
			q.Holes[j] = append([]AlgebraicShellBooleanUse(nil), ring...)
		}
		out.faces = append(out.faces, q)
	}
	return out, nil
}
