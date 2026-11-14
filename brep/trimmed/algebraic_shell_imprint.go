package trimmed

import (
	"context"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// AlgebraicShellImprintPiece refers to one original, paired tube segment.
// Reversed changes the walk direction without changing its native parameters.
type AlgebraicShellImprintPiece struct {
	Branch, Index int
	Reversed      bool
}

// AlgebraicShellImprintUse selects either an original subdivided edge or a
// verified section piece. OriginalEdge and SectionPiece use -1 when absent.
type AlgebraicShellImprintUse struct {
	OriginalEdge, SectionPiece int
	Reversed                   bool
}

// AlgebraicShellImprintFace describes a region on an original shell support.
// Each use refers to the result's common edge table; endpoints are indices in
// Vertices, never numbers compared after rounding. Both incidences of an
// imprinted cut are retained and the original material is unchanged.
type AlgebraicShellImprintFace struct {
	Original, Fragment int
	Reversed           bool
	Outer              []AlgebraicShellImprintUse
	Holes              [][]AlgebraicShellImprintUse
}

type AlgebraicShellImprintVertex struct {
	OriginalVertex int // index in Subdivision, -1 for an interior tube junction
	SectionPoint   *geom.ExactComposedCurveEvent
}

type AlgebraicShellImprintEdge struct{ Start, End int }

type AlgebraicShellImprintOptions struct {
	Seam      AlgebraicSeamOptions
	Endpoints geom.TubeAlgebraicGraphEndpointOptions
	Exact     geom.ExactCurvePlaneEventOptions
	MaxPieces int
}

type AlgebraicShellImprint struct {
	Subdivision AlgebraicSubdivision
	Vertices    []AlgebraicShellImprintVertex
	Edges       []AlgebraicShellImprintEdge // original subdivision edges, then Section
	Section     []AlgebraicShellImprintPiece
	Faces       []AlgebraicShellImprintFace
	Topology    AlgebraicSubdivisionTopology
}

// ImprintOriginalShells arranges a complete regular section on both original
// rectangular spline shells. It supports proper open cuts, with distinct
// transverse anchors, and closed sections wholly inside an original face.
// Multiple closed loops on one face require disjoint conservative chart
// boxes or independently proved nesting on a common affine graph projection.
// Faces carrying both open and closed cuts remain unresolved.
// Every section segment is shared by corresponding source fragments. Both
// results are returned only after closed incidence and vertex links pass.
func (r AlgebraicShellSectionResult) ImprintOriginalShells(ctx context.Context,
	o AlgebraicShellImprintOptions) (out [2]AlgebraicShellImprint, err error) {
	if ctx == nil || r.source == nil || o.MaxPieces < 0 || o.MaxPieces > 4096 || o.Exact.MaxOperations < 0 ||
		o.Exact.MaxCoefficientBits != 0 && o.Exact.MaxCoefficientBits < 64 || o.Exact.MaxSubdivisions < 0 {
		return out, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			out = [2]AlgebraicShellImprint{}
		}
	}()
	if o.MaxPieces == 0 {
		o.MaxPieces = 256
	}
	if o.Exact.MaxOperations == 0 {
		o.Exact.MaxOperations = 64000000
	}
	sub, e := r.SubdivideOriginalEdges(ctx, o.Seam)
	if e != nil {
		return out, e
	}
	src := r.source
	work := shellImprintWork{ctx: ctx, source: src, exact: o.Exact, endpoint: o.Endpoints}
	var offsets []int
	var pieces []AlgebraicShellImprintPiece
	for i, b := range src.branches {
		if len(b.Segments) == 0 || len(b.Segments) > o.MaxPieces-len(pieces) {
			return out, geom.ErrConvergence
		}
		offsets = append(offsets, len(pieces))
		for j := range b.Segments {
			pieces = append(pieces, AlgebraicShellImprintPiece{Branch: i, Index: j})
		}
	}
	work.offsets = offsets
	points, e := work.modelVertices()
	if e != nil {
		return out, e
	}
	for i := range out {
		out[i], e = work.imprint(i, sub[i], points, pieces)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}

type shellImprintWork struct {
	ctx      context.Context
	source   *algebraicShellSectionSource
	offsets  []int
	exact    geom.ExactCurvePlaneEventOptions
	endpoint geom.TubeAlgebraicGraphEndpointOptions
}

type shellImprintPoint struct {
	event  *geom.ExactComposedCurveEvent
	branch int
	end    int // -1 for internal junction
	vertex int // common section vertex identity
}

func (w *shellImprintWork) modelVertices() ([][2]shellImprintPoint, error) {
	var out [][2]shellImprintPoint
	next := 0
	for i, b := range w.source.branches {
		if e := w.ctx.Err(); e != nil {
			return nil, e
		}
		var start shellImprintPoint
		for j, s := range b.Segments {
			if j == 0 {
				v, e := w.modelPoint(i, j, 0, s)
				if e != nil {
					return nil, e
				}
				start = shellImprintPoint{event: v, branch: i, end: 0, vertex: next}
				next++
			}
			last := j == len(b.Segments)-1
			var finish shellImprintPoint
			if b.Closed && last {
				finish = out[w.offsets[i]][0]
			} else {
				v, e := w.modelPoint(i, j, 1, s)
				if e != nil {
					return nil, e
				}
				end := -1
				if last {
					end = 1
				}
				finish = shellImprintPoint{event: v, branch: i, end: end, vertex: next}
				next++
			}
			out = append(out, [2]shellImprintPoint{start, finish})
			start = finish
		}
	}
	// A whole-shell wire supplies the authoritative identity across
	// different original face pairs, including an irrational terminal.
	for _, ring := range w.source.wires {
		if len(ring) == 1 && w.source.branches[ring[0].Branch].Closed {
			continue
		}
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			endA, endB := 1, 0
			if a.Reversed {
				endA = 0
			}
			if b.Reversed {
				endB = 1
			}
			at := func(x int, end int) *shellImprintPoint {
				j := w.offsets[x]
				if end == 1 {
					j += len(w.source.branches[x].Segments) - 1
				}
				return &out[j][end]
			}
			x, y := at(a.Branch, endA), at(b.Branch, endB)
			x.vertex, y.vertex = min(x.vertex, y.vertex), min(x.vertex, y.vertex)
			y.event = x.event
		}
	}
	return out, nil
}

func (w *shellImprintWork) modelPoint(branch, piece, end int, s intersect.TubeSegment) (*geom.ExactComposedCurveEvent, error) {
	b := w.source.branches[branch]
	if !b.Closed && (piece == 0 && end == 0 || piece == len(b.Segments)-1 && end == 1) {
		p := b.Ends[end].Certificate.ModelEvent
		if p == nil {
			return nil, geom.ErrConvergence
		}
		return p, nil
	}
	var faces [2]intersect.Patch
	for i, x := range [2]Solid{w.source.a, w.source.b} {
		f := x.faces[b.Faces[i]]
		d, e := faceUVBounds(f)
		if e != nil {
			return nil, e
		}
		faces[i], e = algebraicShellTracePatch(f, d)
		if e != nil {
			return nil, e
		}
	}
	o := w.endpoint
	o.MaxOperations = capPositiveBudget(o.MaxOperations, w.exact.MaxOperations)
	o.MaxCoefficientBits = capPositiveBudget(o.MaxCoefficientBits, w.exact.MaxCoefficientBits)
	if o.MaxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	p, e := discoverExactTubeSectionEndpoint(w.ctx, s, end, faces[0], faces[1], o)
	w.exact.MaxOperations -= p.ExactOperations
	if e != nil {
		return nil, e
	}
	if !p.Proven || p.ModelEvent == nil {
		return nil, geom.ErrConvergence
	}
	return p.ModelEvent, nil
}

func (w *shellImprintWork) anchorVertex(s AlgebraicSubdivision, edge int, root *geom.ExactAlgebraicRoot) (int, error) {
	for _, x := range s.edges {
		if x.Original != edge || x.Upper == nil {
			continue
		}
		if w.exact.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		q, e := x.Upper.Compare(w.ctx, root, w.exact)
		w.exact.MaxOperations -= q.Operations
		if e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, geom.ErrConvergence
		}
		if q.Order == 0 {
			return x.End, nil
		}
	}
	return 0, geom.ErrConvergence
}

func (w *shellImprintWork) imprint(op int, sub AlgebraicSubdivision, points [][2]shellImprintPoint,
	pieces []AlgebraicShellImprintPiece) (out AlgebraicShellImprint, err error) {
	out.Subdivision = sub
	out.Section = append([]AlgebraicShellImprintPiece(nil), pieces...)
	for i := range sub.vertices {
		out.Vertices = append(out.Vertices, AlgebraicShellImprintVertex{OriginalVertex: i})
	}
	for _, x := range sub.edges {
		out.Edges = append(out.Edges, AlgebraicShellImprintEdge{x.Start, x.End})
	}
	bySectionVertex := map[int]int{}
	for _, p := range points {
		for _, x := range p {
			b := w.source.branches[x.branch]
			if x.end < 0 || b.Closed || b.Ends[x.end].Operand != op {
				continue
			}
			v, e := w.anchorVertex(sub, b.EndEdges[x.end], b.EdgeParameters[x.end])
			if e != nil {
				return out, e
			}
			if old, ok := bySectionVertex[x.vertex]; ok && old != v {
				return out, geom.ErrConvergence
			}
			bySectionVertex[x.vertex] = v
			out.Vertices[v].SectionPoint = x.event
		}
	}
	for _, p := range points {
		v := [2]int{}
		for i, x := range p {
			a, ok := bySectionVertex[x.vertex]
			if !ok {
				a = len(out.Vertices)
				bySectionVertex[x.vertex] = a
				out.Vertices = append(out.Vertices, AlgebraicShellImprintVertex{OriginalVertex: -1, SectionPoint: x.event})
			}
			v[i] = a
		}
		if v[0] == v[1] {
			return out, geom.ErrConvergence
		}
		out.Edges = append(out.Edges, AlgebraicShellImprintEdge{v[0], v[1]})
	}
	paths, e := w.facePaths(op, out)
	if e != nil {
		return out, e
	}
	for i, f := range sub.faces {
		regions, e := w.arrangeFace(i, op, f, out, paths[i])
		if e != nil {
			return out, e
		}
		out.Faces = append(out.Faces, regions...)
	}
	if e := out.validateIncidence(w.ctx, true); e != nil {
		return out, e
	}
	return out, nil
}

type shellImprintPath struct {
	uses       []AlgebraicShellImprintUse
	start, end int
	closed     bool
}

func (w *shellImprintWork) facePaths(op int, out AlgebraicShellImprint) (map[int][]shellImprintPath, error) {
	paths := map[int][]shellImprintPath{}
	for _, ring := range w.source.wires {
		if len(ring) == 0 {
			return nil, geom.ErrConvergence
		}
		start := -1
		for i, use := range ring {
			b := w.source.branches[use.Branch]
			end := 0
			if use.Reversed {
				end = 1
			}
			if !b.Closed && b.Ends[end].Operand == op {
				start = i
				break
			}
		}
		closed := start < 0
		if closed {
			start = 0
		}
		path := shellImprintPath{closed: closed}
		face := -1
		for j := range ring {
			x := ring[(start+j)%len(ring)]
			b := w.source.branches[x.Branch]
			if face < 0 {
				face = b.Faces[op]
			}
			if face != b.Faces[op] {
				return nil, geom.ErrConvergence
			}
			for k := range b.Segments {
				index := k
				if x.Reversed {
					index = len(b.Segments) - 1 - k
				}
				id := w.offsets[x.Branch] + index
				use := AlgebraicShellImprintUse{OriginalEdge: -1, SectionPiece: id, Reversed: x.Reversed}
				a, z := out.edgeEnds(use)
				if len(path.uses) == 0 {
					path.start = a
				} else if path.end != a {
					return nil, geom.ErrConvergence
				}
				path.end = z
				path.uses = append(path.uses, use)
			}
			end := 1
			if x.Reversed {
				end = 0
			}
			if !closed && b.Ends[end].Operand == op {
				if path.start == path.end {
					return nil, geom.ErrConvergence
				}
				paths[face] = append(paths[face], path)
				path, face = shellImprintPath{}, -1
			}
		}
		if closed {
			if path.start != path.end || len(path.uses) < 2 {
				return nil, geom.ErrConvergence
			}
			paths[face] = append(paths[face], path)
		} else if len(path.uses) != 0 {
			return nil, geom.ErrConvergence
		}
	}
	return paths, nil
}

func (s AlgebraicShellImprint) edgeID(u AlgebraicShellImprintUse) int {
	if u.OriginalEdge >= 0 {
		return u.OriginalEdge
	}
	return len(s.Subdivision.edges) + u.SectionPiece
}

func (s AlgebraicShellImprint) edgeEnds(u AlgebraicShellImprintUse) (int, int) {
	x := s.Edges[s.edgeID(u)]
	if u.Reversed {
		return x.End, x.Start
	}
	return x.Start, x.End
}

func reverseImprintUses(u []AlgebraicShellImprintUse) []AlgebraicShellImprintUse {
	out := make([]AlgebraicShellImprintUse, len(u))
	for i, x := range u {
		x.Reversed = !x.Reversed
		out[len(u)-1-i] = x
	}
	return out
}

func (w *shellImprintWork) arrangeFace(face, op int, f AlgebraicSubdivisionFace, s AlgebraicShellImprint,
	paths []shellImprintPath) ([]AlgebraicShellImprintFace, error) {
	if len(f.Holes) != 0 {
		return nil, geom.ErrConvergence
	}
	original := AlgebraicShellImprintFace{Original: face, Reversed: f.Reversed}
	for _, u := range f.Outer {
		original.Outer = append(original.Outer, AlgebraicShellImprintUse{OriginalEdge: u.Edge, SectionPiece: -1, Reversed: u.Reversed})
	}
	if len(paths) == 0 {
		return []AlgebraicShellImprintFace{original}, nil
	}
	for _, p := range paths {
		if p.closed {
			return w.arrangeClosedFacePaths(face, op, original, s, paths)
		}
	}
	indices := map[int]int{}
	for i, u := range original.Outer {
		a, _ := s.edgeEnds(u)
		indices[a] = i
	}
	type turn struct {
		uses []AlgebraicShellImprintUse
		end  int
	}
	turns := map[int]turn{}
	for _, p := range paths {
		i, a := indices[p.start]
		j, b := indices[p.end]
		if !a || !b || i == j || turns[i].uses != nil || turns[j].uses != nil {
			return nil, geom.ErrConvergence
		}
		turns[i], turns[j] = turn{p.uses, j}, turn{reverseImprintUses(p.uses), i}
	}
	used := make([]bool, len(original.Outer))
	var out []AlgebraicShellImprintFace
	for i := range used {
		if used[i] {
			continue
		}
		r := AlgebraicShellImprintFace{Original: face, Fragment: len(out), Reversed: f.Reversed}
		for j := i; ; {
			used[j] = true
			r.Outer = append(r.Outer, original.Outer[j])
			j = (j + 1) % len(used)
			if t, ok := turns[j]; ok {
				r.Outer = append(r.Outer, t.uses...)
				j = t.end
			}
			if i == j {
				break
			}
			if used[j] {
				return nil, geom.ErrConvergence
			}
		}
		out = append(out, r)
	}
	if len(out) != len(paths)+1 {
		return nil, geom.ErrConvergence
	}
	return out, nil
}

func (w *shellImprintWork) arrangeClosedFacePaths(face, op int, original AlgebraicShellImprintFace,
	s AlgebraicShellImprint, paths []shellImprintPath) ([]AlgebraicShellImprintFace, error) {
	var boxes []geom.UVBox
	if len(paths) > 128 {
		return nil, geom.ErrConvergence
	}
	for _, p := range paths {
		if e := w.ctx.Err(); e != nil {
			return nil, e
		}
		if !p.closed {
			return nil, geom.ErrConvergence
		}
		if len(paths) == 1 {
			continue
		}
		var box geom.UVBox
		for i, use := range p.uses {
			x := s.Section[use.SectionPiece]
			b := w.source.branches[x.Branch]
			if b.Faces[op] != face {
				return nil, geom.ErrConvergence
			}
			seg := b.Segments[x.Index]
			q, e := [2]intersect.TubeTrim{seg.TrimA(), seg.TrimB()}[op].BoundsUV()
			if e != nil {
				return nil, e
			}
			if i == 0 {
				box = q
				continue
			}
			box.Min.X, box.Min.Y = min(box.Min.X, q.Min.X), min(box.Min.Y, q.Min.Y)
			box.Max.X, box.Max.Y = max(box.Max.X, q.Max.X), max(box.Max.Y, q.Max.Y)
		}
		boxes = append(boxes, box)
	}
	parents := make([]int, len(paths))
	for i := range parents {
		parents[i] = -1
	}
	// A strict gap between conservative boxes excludes nesting. Where
	// the boxes overlap, classify the exact closed loops instead.
	needContainment := false
	for i, box := range boxes {
		for _, other := range boxes[:i] {
			needContainment = needContainment ||
				(box.Min.X <= other.Max.X && other.Min.X <= box.Max.X && box.Min.Y <= other.Max.Y && other.Min.Y <= box.Max.Y)
		}
	}
	if needContainment {
		var e error
		parents, e = w.closedPathParents(face, op, s, paths)
		if e != nil {
			return nil, e
		}
	}
	out := make([]AlgebraicShellImprintFace, len(paths)+1)
	out[0] = original
	for i, p := range paths {
		winding, e := w.closedPathWinding(face, op, p, s)
		if e != nil {
			return nil, e
		}
		outer, inner := p.uses, reverseImprintUses(p.uses)
		if winding < 0 {
			outer, inner = inner, outer
		}
		parent := &out[parents[i]+1]
		parent.Holes = append(parent.Holes, inner)
		// Earlier children may already have added holes to this fragment.
		f := &out[i+1]
		f.Original, f.Fragment, f.Reversed, f.Outer = face, i+1, original.Reversed, outer
	}
	return out, nil
}

func (s *AlgebraicShellImprint) validateIncidence(ctx context.Context, preserveEuler bool) error {
	type atVertex struct{ vertex, edge int }
	links := map[atVertex][]int{}
	valence := make([]int, len(s.Vertices))
	count, sign := make([]int, len(s.Edges)), make([]int, len(s.Edges))
	for _, x := range s.Edges {
		if x.Start < 0 || x.Start >= len(valence) || x.End < 0 || x.End >= len(valence) || x.Start == x.End {
			return geom.ErrConvergence
		}
		valence[x.Start]++
		valence[x.End]++
	}
	for _, f := range s.Faces {
		if e := ctx.Err(); e != nil {
			return e
		}
		for _, ring := range append([][]AlgebraicShellImprintUse{f.Outer}, f.Holes...) {
			if len(ring) < 2 {
				return geom.ErrConvergence
			}
			seen := map[int]bool{}
			for i, u := range ring {
				v := ring[(i+1)%len(ring)]
				id, next := s.edgeID(u), s.edgeID(v)
				_, a := s.edgeEnds(u)
				b, _ := s.edgeEnds(v)
				if a != b || id == next || seen[a] {
					return geom.ErrConvergence
				}
				seen[a] = true
				count[id]++
				if u.Reversed == f.Reversed {
					sign[id]++
				} else {
					sign[id]--
				}
				links[atVertex{a, id}] = append(links[atVertex{a, id}], next)
				links[atVertex{a, next}] = append(links[atVertex{a, next}], id)
			}
		}
	}
	visited := make([]bool, len(valence))
	for node, adjacent := range links {
		if len(adjacent) != 2 {
			return geom.ErrConvergence
		}
		if visited[node.vertex] {
			continue
		}
		seen := map[int]bool{}
		stack := []int{node.edge}
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !seen[i] {
				seen[i] = true
				stack = append(stack, links[atVertex{node.vertex, i}]...)
			}
		}
		if len(seen) != valence[node.vertex] {
			return geom.ErrConvergence
		}
		visited[node.vertex] = true
	}
	for i, n := range count {
		if n != 2 || sign[i] != 0 {
			return geom.ErrConvergence
		}
	}
	for _, known := range visited {
		if !known {
			return geom.ErrConvergence
		}
	}
	t := AlgebraicSubdivisionTopology{Vertices: len(s.Vertices), Edges: len(s.Edges), Faces: len(s.Faces)}
	for _, f := range s.Faces {
		t.InnerWires += len(f.Holes)
	}
	t.EulerCharacteristic = t.Vertices - t.Edges + t.Faces - t.InnerWires
	if preserveEuler && t.EulerCharacteristic != s.Subdivision.Topology().EulerCharacteristic {
		return geom.ErrConvergence
	}
	s.Topology = t
	return nil
}
