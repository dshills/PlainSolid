package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicGraphMaterialOptions bounds preparation independently of each query.
// Nil Projection selects XY; exact projected graph proofs also support YZ/XZ.
// Native limits default to16 loops and2048 total segments (at most128 per loop).
type AlgebraicGraphMaterialOptions struct {
	Projection            *[2]int
	Exact                 geom.ExactCurvePlaneEventOptions
	MaxLoops, MaxSegments int
}
type graphMaterialRegion struct {
	outer int
	holes []int
	faces [2]int
	upper int
}

// AlgebraicGraphMaterialQuery is an immutable prepared query over the retained
// fragments of a graph-only common. It never treats an unselected nested band
// as material. No display mesh or rounded model point participates in proof.
type AlgebraicGraphMaterialQuery struct {
	axes            [2]int
	residual        int
	graphs          [2]*geom.AffineProjectedGraph
	patches         [2]*geom.SurfacePatch
	faces           [2]Face
	loops           [][]AlgebraicTubeFaceCutPiece
	edges           [][]int
	regions         []graphMaterialRegion
	operations      int
	windowTubeBoxes [][][]geom.ParameterRectangle // Immutable full-segment covering boxes.
}

func (q *AlgebraicGraphMaterialQuery) ProjectionAxes() [2]int {
	if q == nil {
		return [2]int{-1, -1}
	}
	return q.axes
}
func (q *AlgebraicGraphMaterialQuery) PreparationOperations() int {
	if q == nil {
		return 0
	}
	return q.operations
}

type AlgebraicGraphMaterialPoint struct {
	Location        parametric.Location
	Complete        bool
	Faces           [2]int // Native bracketing Boolean faces, original operand order; -1 when absent.
	BoundaryFaces   []int
	RimEdges        []int    // Canonical native edge IDs; two at a shared native vertex.
	Lower, Upper    *big.Rat // Exact residual-coordinate fiber endpoints when present.
	ExactOperations int
}

func graphMaterialLimits(o geom.ExactCurvePlaneEventOptions) (geom.ExactCurvePlaneEventOptions, error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 32000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 1000000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 || o.MaxSubdivisions < 0 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// PrepareGraphMaterialQuery accepts a qualified intersection consisting only
// of complete closed generated loops on one common original graph-face pair.
// Each retained face may have at most one hole. The original immutable Boolean
// supplies material selection and loop ownership; graph inversion and every
// point/loop relation are independently exact. Unsupported source edges,
// partial loops, multiple chart pairs and ambiguous ownership fail atomically.
func (s AlgebraicShellBoolean) PrepareGraphMaterialQuery(ctx context.Context, o AlgebraicGraphMaterialOptions) (*AlgebraicGraphMaterialQuery, error) {
	if ctx == nil || s.source == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	exact, e := graphMaterialLimits(o.Exact)
	if e != nil {
		return nil, e
	}
	if o.MaxLoops == 0 {
		o.MaxLoops = 16
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 2048
	}
	if o.MaxLoops < 1 || o.MaxLoops > 16 || o.MaxSegments < 1 || o.MaxSegments > 2048 {
		return nil, geom.ErrInvalid
	}
	n := len(s.source.branches)
	if s.op != parametric.Intersection || n < 1 || n > o.MaxLoops || len(s.faces) < 2 {
		return nil, geom.ErrConvergence
	}
	q := &AlgebraicGraphMaterialQuery{axes: [2]int{0, 1}, loops: make([][]AlgebraicTubeFaceCutPiece, n), edges: make([][]int, n)}
	if o.Projection != nil {
		q.axes = *o.Projection
	}
	if q.axes[0] < 0 || q.axes[0] > 2 || q.axes[1] < 0 || q.axes[1] > 2 || q.axes[0] == q.axes[1] {
		return nil, geom.ErrInvalid
	}
	q.residual = 3 - q.axes[0] - q.axes[1]
	remaining := func() geom.ExactCurvePlaneEventOptions { p := exact; p.MaxOperations -= q.operations; return p }
	pair := s.source.branches[0].Faces
	total := 0
	for i, b := range s.source.branches {
		if !b.Closed || b.Faces != pair || len(b.Segments) < 3 || len(b.Segments) > 128 {
			return nil, geom.ErrConvergence
		}
		total += len(b.Segments)
		if total > o.MaxSegments {
			return nil, geom.ErrConvergence
		}
		q.edges[i] = make([]int, len(b.Segments))
		for j := range q.edges[i] {
			q.edges[i][j] = -1
		}
	}
	if len(s.index) != total {
		return nil, geom.ErrConvergence
	}
	for id, edge := range s.index {
		if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 {
			return nil, geom.ErrConvergence
		}
		p := s.parts[0].Section[edge.SectionPiece]
		if p.Branch < 0 || p.Branch >= n || p.Index < 0 || p.Index >= len(q.edges[p.Branch]) || p.Reversed || q.edges[p.Branch][p.Index] >= 0 {
			return nil, geom.ErrConvergence
		}
		q.edges[p.Branch][p.Index] = id
	}
	ring := func(row []AlgebraicShellBooleanUse) (int, error) {
		if len(row) < 3 {
			return -1, geom.ErrConvergence
		}
		first := s.parts[0].Section[s.index[row[0].Edge].SectionPiece]
		branch := first.Branch
		n := len(q.edges[branch])
		if len(row) != n {
			return -1, geom.ErrConvergence
		}
		for j, u := range row {
			p := s.parts[0].Section[s.index[u.Edge].SectionPiece]
			want := (first.Index + j) % n
			if row[0].Reversed {
				want = (first.Index + n - j) % n
			}
			if p.Branch != branch || p.Index != want || u.Reversed != row[0].Reversed {
				return -1, geom.ErrConvergence
			}
		}
		return branch, nil
	}
	grouped := make([]graphMaterialRegion, n)
	seen := make([]bool, n)
	used := make([]bool, n)
	for i := range grouped {
		grouped[i].faces = [2]int{-1, -1}
	}
	firstFaces := [2]int{-1, -1}
	for id, f := range s.faces {
		if f.Operand < 0 || f.Operand > 1 || f.Original != pair[f.Operand] || len(f.Holes) > 1 {
			return nil, geom.ErrConvergence
		}
		outer, e := ring(f.Outer)
		if e != nil {
			return nil, e
		}
		used[outer] = true
		var holes []int
		for _, row := range f.Holes {
			h, e := ring(row)
			if e != nil || h == outer {
				return nil, geom.ErrConvergence
			}
			holes = append(holes, h)
			used[h] = true
		}
		r := &grouped[outer]
		if r.faces[f.Operand] >= 0 {
			return nil, geom.ErrConvergence
		}
		if seen[outer] {
			if len(r.holes) != len(holes) || len(holes) > 0 && r.holes[0] != holes[0] {
				return nil, geom.ErrConvergence
			}
		} else {
			r.outer = outer
			r.holes = holes
			seen[outer] = true
		}
		r.faces[f.Operand] = id
		firstFaces[f.Operand] = id
	}
	for chart, id := range firstFaces {
		if id < 0 {
			return nil, geom.ErrConvergence
		}
		f, e := s.OriginalFace(id)
		if e != nil {
			return nil, e
		}
		if !convexStraightTubeLensFace(f) {
			return nil, geom.ErrConvergence
		}
		q.faces[chart] = f
		p := remaining()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		patch, work, e := exactLensQueryPatch(f, geom.SurfacePatchOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
		q.operations += work
		if e != nil {
			return nil, e
		}
		q.patches[chart] = patch
		p = remaining()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		g, work, e := geom.NewAffineProjectedGraphWithWork(ctx, patch, q.axes, geom.SurfacePatchOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
		q.operations += work
		if e != nil {
			return nil, e
		}
		q.graphs[chart] = g
	}
	for i, r := range grouped {
		if !used[i] {
			return nil, geom.ErrConvergence
		}
		if !seen[i] {
			continue
		}
		if r.faces[0] < 0 || r.faces[1] < 0 {
			return nil, geom.ErrConvergence
		}
		found := -1
		for upper := 0; upper < 2; upper++ {
			ok := true
			for chart, id := range r.faces {
				rev := (chart != upper) == (projectedAxisParity(q.axes)*q.graphs[chart].Orientation() > 0)
				ok = ok && rev == s.faces[id].Reversed
			}
			if ok {
				if found >= 0 {
					return nil, geom.ErrConvergence
				}
				found = upper
			}
		}
		if found < 0 {
			return nil, geom.ErrConvergence
		}
		r.upper = found
		q.regions = append(q.regions, r)
	}
	for i, b := range s.source.branches {
		for _, seg := range b.Segments {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			_, d, rev, e := seg.ExactFreeDomain()
			if e != nil {
				return nil, e
			}
			a, z := d.Lower, d.Upper
			if rev {
				a, z = z, a
			}
			piece := AlgebraicTubeFaceCutPiece{Segment: seg}
			for j, v := range []*big.Rat{a, z} {
				p := remaining()
				if p.MaxOperations < 1 {
					return nil, geom.ErrConvergence
				}
				root, ops, e := geom.NewExactRationalRoot(ctx, v, p)
				q.operations += ops
				if e != nil {
					return nil, e
				}
				if j == 0 {
					piece.FreeStart = root
				} else {
					piece.FreeEnd = root
				}
			}
			q.loops[i] = append(q.loops[i], piece)
		}
	}
	return q, nil
}

// LocateRational classifies the exact supplied XYZ rationals. Both source graph
// heights and all retained outer/hole relations are proved without converting
// the point to float64. Query work is cumulative and separate from preparation.
// Errors publish no classification, fiber, or partial ancestry.
func (q *AlgebraicGraphMaterialQuery) LocateRational(ctx context.Context, point [3]*big.Rat, o geom.ExactCurvePlaneEventOptions) (out AlgebraicGraphMaterialPoint, err error) {
	return q.locateRationalGraphMaterial(ctx, point, o, false)
}

// fiberOnly requests the closed residual fiber, independently of the supplied
// residual point coordinate; its internal Location is not a point result.
func (q *AlgebraicGraphMaterialQuery) locateRationalGraphMaterial(ctx context.Context, point [3]*big.Rat, o geom.ExactCurvePlaneEventOptions, fiberOnly bool) (out AlgebraicGraphMaterialPoint, err error) {
	out.Faces = [2]int{-1, -1}
	defer func() {
		if err != nil {
			out = AlgebraicGraphMaterialPoint{Faces: [2]int{-1, -1}, ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o, err = graphMaterialLimits(o)
	if err != nil {
		return out, err
	}
	for _, x := range point {
		if x == nil {
			return out, geom.ErrInvalid
		}
		if x.Num().BitLen() > o.MaxCoefficientBits || x.Denom().BitLen() > o.MaxCoefficientBits {
			return out, geom.ErrConvergence
		}
	}
	exact := func() geom.ExactCurvePlaneEventOptions { p := o; p.MaxOperations -= out.ExactOperations; return p }
	eval := func() geom.SurfacePatchEvaluationOptions {
		p := exact()
		return geom.SurfacePatchEvaluationOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits}
	}
	xy := [2]*big.Rat{point[q.axes[0]], point[q.axes[1]]}
	var uv [2][2]*big.Rat
	for chart, g := range q.graphs {
		p := eval()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		v, e := g.InvertRational(ctx, xy, p)
		out.ExactOperations += v.Operations
		if e != nil {
			return out, e
		}
		if !v.Inside || !insideConvexTubeFace(q.faces[chart], v.UV) {
			out.Complete = true
			return out, nil
		}
		uv[chart] = v.UV
	}
	p := eval()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	h, e := q.graphs[0].CompareHeightAtRational(ctx, q.graphs[1], uv[0], p)
	out.ExactOperations += h.Operations
	if e != nil {
		return out, e
	}
	if !h.Complete || h.TargetOutside {
		return out, geom.ErrConvergence
	}
	z := [2]*big.Rat{h.SourcePoint[q.residual], h.TargetPoint[q.residual]}
	selected := -1
	if h.Sign == 0 {
		if !fiberOnly && point[q.residual].Cmp(z[0]) != 0 {
			out.Complete = true
			return out, nil
		}
		rim := -1
		for i, cut := range q.loops {
			for j, piece := range cut {
				p := exact()
				if p.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				on, used, e := pointOnTubeLoop(ctx, []AlgebraicTubeFaceCutPiece{piece}, uv, p, q.axes)
				out.ExactOperations += used
				if e != nil {
					return out, e
				}
				if on {
					if rim >= 0 && rim != i {
						return out, geom.ErrConvergence
					}
					rim = i
					out.RimEdges = append(out.RimEdges, q.edges[i][j])
				}
			}
		}
		if rim < 0 {
			return out, geom.ErrConvergence
		}
		for i, r := range q.regions {
			match := r.outer == rim
			for _, hole := range r.holes {
				match = match || hole == rim
			}
			if match {
				if selected >= 0 {
					return out, geom.ErrConvergence
				}
				selected = i
			}
		}
		if selected < 0 {
			return out, geom.ErrConvergence
		}
		out.Faces = q.regions[selected].faces
		out.BoundaryFaces = []int{out.Faces[0], out.Faces[1]}
		out.Lower = new(big.Rat).Set(z[0])
		out.Upper = new(big.Rat).Set(z[0])
		out.Location = parametric.Boundary
		out.Complete = true
		return out, nil
	}
	contains := make([]bool, len(q.loops))
	for i, cut := range q.loops {
		success := false
		for axis := 0; axis < 2; axis++ {
			p := exact()
			if p.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			root, used, e := geom.NewExactRationalRoot(ctx, uv[0][axis], p)
			out.ExactOperations += used
			if e != nil {
				return out, e
			}
			p = exact()
			if p.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			in, used, e := insideClosedTubeAtAxisRoot(ctx, 0, q.patches[0], cut, root, uv[0][1-axis], axis, p, q.axes)
			out.ExactOperations += used
			if errors.Is(e, geom.ErrConvergence) {
				continue
			}
			if e != nil {
				return out, e
			}
			contains[i] = in
			success = true
			break
		}
		if !success {
			return out, geom.ErrConvergence
		}
	}
	for i, r := range q.regions {
		inside := contains[r.outer]
		for _, hole := range r.holes {
			inside = inside && !contains[hole]
		}
		if inside {
			if selected >= 0 {
				return out, geom.ErrConvergence
			}
			selected = i
		}
	}
	if selected < 0 {
		out.Complete = true
		return out, nil
	}
	r := q.regions[selected]
	if (h.Sign > 0) != (r.upper == 0) {
		return out, geom.ErrConvergence
	}
	out.Faces = r.faces
	out.Lower = new(big.Rat).Set(z[r.upper^1])
	out.Upper = new(big.Rat).Set(z[r.upper])
	if fiberOnly {
		out.BoundaryFaces = []int{r.faces[r.upper^1], r.faces[r.upper]}
		out.Complete = true
		return out, nil
	}
	low, high := point[q.residual].Cmp(out.Lower), point[q.residual].Cmp(out.Upper)
	if low >= 0 && high <= 0 {
		out.Location = parametric.Inside
		if low == 0 {
			out.Location = parametric.Boundary
			out.BoundaryFaces = append(out.BoundaryFaces, r.faces[r.upper^1])
		}
		if high == 0 {
			out.Location = parametric.Boundary
			out.BoundaryFaces = append(out.BoundaryFaces, r.faces[r.upper])
		}
	}
	out.Complete = true
	return out, nil
}
