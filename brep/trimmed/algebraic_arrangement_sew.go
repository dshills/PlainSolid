package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// AlgebraicSewFace selects a source fragment. Reverse flips its physical
// orientation without changing either original chart or loop traversal.
type AlgebraicSewFace struct {
	Face    int
	Reverse bool
}

// AlgebraicSewPair identifies an edge from each arrangement. Reverse means
// their increasing model-edge parameters run in opposite physical directions.
// For unrelated curves ProjectionAxis selects the common affine coordinate;
// -1 selects exact projective conic matching with nonlinear parameter speeds.
type AlgebraicSewPair struct {
	Edges          [2]int
	Reverse        bool
	ProjectionAxis int
}

type AlgebraicSewEdgeSource struct{ Operand, Edge int }

type AlgebraicSewFaceSource struct{ Operand, Face, Original, Fragment int }

// AlgebraicSewResult is a closed indexed assembly of selected exact
// fragments. It retains both input curves on every sewn edge. This proves
// sewing and manifold vertex links; input classification and global material
// embedding require separate certificates.
type AlgebraicSewResult struct {
	Edges               [][2]int
	Faces               []AlgebraicArrangementFace
	EdgeSources         [][]AlgebraicSewEdgeSource
	FaceSources         []AlgebraicSewFaceSource
	VertexSources       [][][2]int
	EulerCharacteristic int
	inputs              [2]AlgebraicFaceArrangement
	recipeSelection     [2][]AlgebraicSewFace
	recipePairs         []AlgebraicSewPair
}

// OriginalFace returns the unmodified source face chart and trim loops for
// one result face. The result's own indexed loops describe its fragment.
func (s AlgebraicSewResult) OriginalFace(face int) (Face, error) {
	if face < 0 || face >= len(s.FaceSources) {
		return Face{}, geom.ErrInvalid
	}
	x := s.FaceSources[face]
	if x.Operand < 0 || x.Operand > 1 || x.Original < 0 || x.Original >= len(s.inputs[x.Operand].subdivision.source.faces) {
		return Face{}, geom.ErrInvalid
	}
	return cloneFace(s.inputs[x.Operand].subdivision.source.faces[x.Original]), nil
}

// SourceModelCurveCells gives the exact geometry and original parameter for
// one retained owner of a sewn edge. The result does not reparameterize it.
func (s AlgebraicSewResult) SourceModelCurveCells(ctx context.Context, edge, owner int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	if edge < 0 || edge >= len(s.EdgeSources) || owner < 0 || owner >= len(s.EdgeSources[edge]) {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	x := s.EdgeSources[edge][owner]
	if x.Operand < 0 || x.Operand > 1 {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	return s.inputs[x.Operand].ModelCurveCells(ctx, x.Edge, o)
}

// SewAlgebraicArrangementFragments verifies specified pairs, preserves exact
// vertex identity even when displays coincide, and requires every remaining
// edge to have two opposite uses. It never pairs edges by spatial proximity.
func SewAlgebraicArrangementFragments(ctx context.Context, a, b AlgebraicFaceArrangement,
	selection [2][]AlgebraicSewFace, pairs []AlgebraicSewPair, o AlgebraicArrangementContactOptions) (AlgebraicSewResult, error) {
	var empty AlgebraicSewResult
	if ctx == nil || len(pairs) > 256 || len(selection[0])+len(selection[1]) == 0 || len(selection[0])+len(selection[1]) > 256 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	arr := [2]AlgebraicFaceArrangement{a, b}
	used := [2]map[int]int{make(map[int]int), make(map[int]int)}
	for op, rows := range selection {
		seen := map[int]bool{}
		for _, x := range rows {
			if x.Face < 0 || x.Face >= len(arr[op].faces) || seen[x.Face] {
				return empty, geom.ErrInvalid
			}
			seen[x.Face] = true
			f := arr[op].faces[x.Face]
			for _, loop := range append([][]AlgebraicSubdivisionUse{f.Outer}, f.Holes...) {
				for _, u := range loop {
					used[op][u.Edge]++
				}
			}
		}
	}
	vertexIDs := map[[2]int]int{}
	var ids [][2]int
	for op, x := range arr {
		for i := range x.subdivision.vertices {
			vertexIDs[[2]int{op, i}] = len(ids)
			ids = append(ids, [2]int{op, i})
		}
	}
	parent := make([]int, len(ids))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	join := func(i, j int) {
		i, j = find(i), find(j)
		if i != j {
			parent[max(i, j)] = min(i, j)
		}
	}
	paired := [2]map[int]int{make(map[int]int), make(map[int]int)}
	for i, p := range pairs {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if p.ProjectionAxis < -1 || p.ProjectionAxis > 2 {
			return empty, geom.ErrInvalid
		}
		for k, id := range p.Edges {
			if id < 0 || id >= len(arr[k].edges) || used[k][id] != 1 {
				return empty, geom.ErrInvalid
			}
			if _, ok := paired[k][id]; ok {
				return empty, geom.ErrInvalid
			}
			paired[k][id] = i
		}
		ok, e := proveSewAlgebraicEdge(ctx, a, b, p, o)
		if e != nil {
			return empty, e
		}
		if !ok {
			return empty, geom.ErrConvergence
		}
		x, y := a.edges[p.Edges[0]], b.edges[p.Edges[1]]
		l, r := y.Start, y.End
		if p.Reverse {
			l, r = r, l
		}
		join(vertexIDs[[2]int{0, x.Start}], vertexIDs[[2]int{1, l}])
		join(vertexIDs[[2]int{0, x.End}], vertexIDs[[2]int{1, r}])
	}
	out := AlgebraicSewResult{inputs: arr}
	for i, r := range selection {
		out.recipeSelection[i] = append([]AlgebraicSewFace(nil), r...)
	}
	out.recipePairs = append([]AlgebraicSewPair(nil), pairs...)
	modelVertex := map[int]int{}
	vertex := func(op, old int) int {
		i := find(vertexIDs[[2]int{op, old}])
		if j, ok := modelVertex[i]; ok {
			return j
		}
		j := len(out.VertexSources)
		modelVertex[i] = j
		out.VertexSources = append(out.VertexSources, nil)
		return j
	}
	modelEdge := [2]map[int]int{make(map[int]int), make(map[int]int)}
	reverse := [2]map[int]bool{make(map[int]bool), make(map[int]bool)}
	addEdge := func(op, id int) int {
		q := arr[op].edges[id]
		n := len(out.Edges)
		out.Edges = append(out.Edges, [2]int{vertex(op, q.Start), vertex(op, q.End)})
		out.EdgeSources = append(out.EdgeSources, []AlgebraicSewEdgeSource{{op, id}})
		modelEdge[op][id] = n
		return n
	}
	for _, p := range pairs {
		i := addEdge(0, p.Edges[0])
		modelEdge[1][p.Edges[1]] = i
		reverse[1][p.Edges[1]] = p.Reverse
		out.EdgeSources[i] = append(out.EdgeSources[i], AlgebraicSewEdgeSource{1, p.Edges[1]})
	}
	for op, rows := range selection {
		for _, x := range rows {
			f := arr[op].faces[x.Face]
			q := AlgebraicArrangementFace{Original: f.Original, Fragment: f.Fragment, Reversed: f.Reversed != x.Reverse}
			copyLoop := func(loop []AlgebraicSubdivisionUse) []AlgebraicSubdivisionUse {
				r := make([]AlgebraicSubdivisionUse, len(loop))
				for i, u := range loop {
					id, ok := modelEdge[op][u.Edge]
					if !ok {
						id = addEdge(op, u.Edge)
					}
					r[i] = AlgebraicSubdivisionUse{Edge: id, OriginalUse: u.OriginalUse, Reversed: u.Reversed != reverse[op][u.Edge]}
				}
				return r
			}
			q.Outer = copyLoop(f.Outer)
			for _, h := range f.Holes {
				q.Holes = append(q.Holes, copyLoop(h))
			}
			out.Faces = append(out.Faces, q)
			out.FaceSources = append(out.FaceSources, AlgebraicSewFaceSource{op, x.Face, f.Original, f.Fragment})
		}
	}
	for i, s := range ids {
		if j, ok := modelVertex[find(i)]; ok {
			out.VertexSources[j] = append(out.VertexSources[j], s)
		}
	}
	if e := certifyAlgebraicSewLinks(ctx, &out); e != nil {
		return empty, e
	}
	return out, nil
}

func proveSewAlgebraicEdge(ctx context.Context, a, b AlgebraicFaceArrangement, p AlgebraicSewPair, o AlgebraicArrangementContactOptions) (bool, error) {
	x, y := a.edges[p.Edges[0]], b.edges[p.Edges[1]]
	if x.Boundary >= 0 && y.Boundary >= 0 && !p.Reverse {
		u, v := a.subdivision.edges[x.Boundary], b.subdivision.edges[y.Boundary]
		ca, cb := a.subdivision.source.edges[u.Original], b.subdivision.source.edges[v.Original]
		if m, ok := cb.Curve.(surfaceTrimCurve); ok {
			same, e := compareSplineRationalTrimUse(ctx, ca, m.surface, Use{Curve: m.curve, UVStart: m.a, UVEnd: m.b}, edgecheck.Options{
				MaxWork: o.Seams.Composition.MaxOperations, MaxBits: o.Seams.Composition.MaxCoefficientBits, MaxIntervals: o.Seams.Composition.MaxEvents,
			})
			if e != nil {
				return false, e
			}
			if same {
				for _, q := range [][2]*geom.ExactAlgebraicRoot{{u.Lower, v.Lower}, {u.Upper, v.Upper}} {
					if q[0] == nil || q[1] == nil {
						if q[0] != q[1] {
							return false, nil
						}
						continue
					}
					c, e := q[0].Compare(ctx, q[1], o.Roots)
					if e != nil {
						return false, e
					}
					if !c.Proven || c.Order != 0 {
						return false, nil
					}
				}
				return true, nil
			}
		}
	}
	return proveSewProjectedEdge(ctx, a, b, p, o)
}

func proveSewProjectedEdge(ctx context.Context, a, b AlgebraicFaceArrangement, p AlgebraicSewPair, o AlgebraicArrangementContactOptions) (bool, error) {
	x, e := a.ModelCurveCells(ctx, p.Edges[0], o.Seams)
	if e != nil {
		return false, e
	}
	y, e := b.ModelCurveCells(ctx, p.Edges[1], o.Seams)
	if e != nil {
		return false, e
	}
	if p.ProjectionAxis == -1 {
		lim := o.Roots
		if lim.MaxOperations == 0 {
			lim.MaxOperations = 8000000
		}
		for _, cells := range [][]*geom.ExactComposedCurveCell{x.Cells, y.Cells} {
			for i, c := range cells {
				if lim.MaxOperations < 1 {
					return false, geom.ErrConvergence
				}
				r, ops, e := c.ReduceHomogeneous(ctx, lim)
				lim.MaxOperations -= ops
				if e != nil {
					return false, e
				}
				cells[i] = r
			}
		}
		if lim.MaxOperations < 1 {
			return false, geom.ErrConvergence
		}
		r, e := geom.CompareComposedConicCollections(ctx, x, y, geom.ComposedCollectionComparisonOptions{ExactCurvePlaneEventOptions: lim, MaxPairs: o.MaxCellPairs})
		if e != nil {
			return false, e
		}
		return r.Complete && r.SameImage && (r.TraversalRelation < 0) == p.Reverse, nil
	}
	r, e := geom.CompareComposedCollectionsAffineProjection(ctx, x, y, p.ProjectionAxis,
		geom.ComposedCollectionComparisonOptions{ExactCurvePlaneEventOptions: o.Roots, MaxPairs: o.MaxCellPairs})
	if e != nil {
		return false, e
	}
	return r.Complete && r.SameImage && (r.ProjectionOrientation[0] != r.ProjectionOrientation[1]) == p.Reverse, nil
}

func certifyAlgebraicSewLinks(ctx context.Context, out *AlgebraicSewResult) error {
	counts := make([][2]int, len(out.Edges))
	fans := make([]map[int][]int, len(out.VertexSources))
	for _, f := range out.Faces {
		if e := ctx.Err(); e != nil {
			return e
		}
		out.EulerCharacteristic -= len(f.Holes)
		for _, loop := range append([][]AlgebraicSubdivisionUse{f.Outer}, f.Holes...) {
			if len(loop) < 2 {
				return geom.ErrConvergence
			}
			for i, u := range loop {
				v := loop[(i+1)%len(loop)]
				x, y := out.Edges[u.Edge], out.Edges[v.Edge]
				if x[0] == x[1] || y[0] == y[1] {
					return geom.ErrConvergence
				}
				if u.Reversed {
					x[0], x[1] = x[1], x[0]
				}
				if v.Reversed {
					y[0], y[1] = y[1], y[0]
				}
				if x[1] != y[0] {
					return geom.ErrConvergence
				}
				k := 0
				if u.Reversed != f.Reversed {
					k = 1
				}
				counts[u.Edge][k]++
				if fans[x[1]] == nil {
					fans[x[1]] = map[int][]int{}
				}
				fans[x[1]][u.Edge] = append(fans[x[1]][u.Edge], v.Edge)
				fans[x[1]][v.Edge] = append(fans[x[1]][v.Edge], u.Edge)
			}
		}
	}
	for _, c := range counts {
		if c != ([2]int{1, 1}) {
			return geom.ErrConvergence
		}
	}
	for _, fan := range fans {
		var stack []int
		for e, n := range fan {
			if len(n) != 2 {
				return geom.ErrConvergence
			}
			if len(stack) == 0 {
				stack = append(stack, e)
			}
		}
		seen := map[int]bool{}
		for len(stack) > 0 {
			e := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[e] {
				continue
			}
			seen[e] = true
			stack = append(stack, fan[e]...)
		}
		if len(seen) != len(fan) {
			return geom.ErrConvergence
		}
	}
	out.EulerCharacteristic += len(out.VertexSources) - len(out.Edges) + len(out.Faces)
	return nil
}
