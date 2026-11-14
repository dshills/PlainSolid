package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ExactSewOptions controls model equality, trim checks and optional orientation
// repair. Independent faces may have separate edge and vertex slots. Geometry
// is preserved exactly. DisplayTolerance applies only to the ordinary native
// constructor's floating evaluation and mesh checks, and defaults to 1e-8.
type ExactSewOptions struct {
	MaxEdges, MaxFaces, MaxComparisons int
	DisplayTolerance                   float64
	RepairOrientation                  bool
	Curve                              geom.SurfacePatchOptions
	Agreement                          edgecheck.Options
	Trims                              trimvalidate.Options
}

// ExactSewnEdge maps an original model edge to its preserved output curve.
// Reversed says the original model edge traverses that curve in reverse.
type ExactSewnEdge struct {
	Edge     int
	Reversed bool
}

// ExactSewReport preserves every original vertex and edge slot. Face order is
// unchanged. FlippedFaces records changes to the physical face orientation.
type ExactSewReport struct {
	Vertices     []int
	Edges        []ExactSewnEdge
	FlippedFaces []int
	Comparisons  int
}

// SewExactTopology joins independent original faces into closed oriented
// trimmed topology. Finite model curves must have distinct exact endpoints
// and admit a rational spline descriptor. Matching uses whole-interval
// equality, allowing affine parameter changes and reversal. All original
// p-curves and UV loops are certified before sewing. Coincident disconnected
// vertices retain separate identities unless a matched edge actually joins
// them. Face orientation repair preserves the first face in each component.
// Global shell embedding and inward/outward cavity classification are separate
// from this topology operation. A failure returns no solid or partial mapping.
func SewExactTopology(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, o ExactSewOptions) (Solid, ExactSewReport, error) {
	input := Solid{vertices: vertices, edges: edges, faces: faces}
	o, err := exactSewSource(ctx, input, o)
	if err != nil {
		return Solid{}, ExactSewReport{}, err
	}
	return finishExactSew(ctx, input, o)
}

func exactSewSource(ctx context.Context, s Solid, o ExactSewOptions) (ExactSewOptions, error) {
	if o.MaxEdges == 0 {
		o.MaxEdges = 4096
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 1024
	}
	if o.MaxComparisons == 0 {
		o.MaxComparisons = 8192
	}
	if o.DisplayTolerance == 0 {
		o.DisplayTolerance = 1e-8
	}
	if ctx == nil || o.MaxEdges < 1 || o.MaxEdges > 16384 || o.MaxFaces < 1 || o.MaxFaces > 4096 ||
		o.MaxComparisons < 1 || o.MaxComparisons > 262144 || !finite(o.DisplayTolerance) || o.DisplayTolerance <= 0 ||
		len(s.edges) == 0 || len(s.faces) == 0 || len(s.vertices) == 0 {
		return o, geom.ErrInvalid
	}
	if len(s.edges) > o.MaxEdges || len(s.faces) > o.MaxFaces || len(s.vertices) > 2*len(s.edges) {
		return o, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	return o, validateExactSewSource(ctx, s, o)
}

func finishExactSew(ctx context.Context, s Solid, o ExactSewOptions) (Solid, ExactSewReport, error) {
	var empty ExactSewReport
	result, report, err := sewExactModelEdges(ctx, s.vertices, s.edges, s.faces, o)
	if err != nil {
		return Solid{}, empty, err
	}
	if err = validateExactSewnVertexLinks(ctx, result); err != nil {
		return Solid{}, empty, err
	}
	return result, report, nil
}

func validateExactSewSource(ctx context.Context, s Solid, o ExactSewOptions) error {
	for _, p := range s.vertices {
		if !p.IsFinite() {
			return geom.ErrInvalid
		}
	}
	for _, e := range s.edges {
		if e.Start < 0 || e.End < 0 || e.Start >= len(s.vertices) || e.End >= len(s.vertices) ||
			e.Curve == nil || s.vertices[e.Start] == s.vertices[e.End] || e.Range.Min >= e.Range.Max || !finite(e.Range.Max-e.Range.Min) {
			return geom.ErrInvalid
		}
	}
	uses := make([]int, len(s.edges))
	for faceIndex, face := range s.faces {
		if err := ctx.Err(); err != nil {
			return err
		}
		if face.Surface == nil || len(face.Holes) > 2*len(s.edges)/3 {
			return geom.ErrInvalid
		}
		for _, loop := range append([][]Use{face.Outer}, face.Holes...) {
			if len(loop) < 3 || len(loop) > len(s.edges) {
				return geom.ErrInvalid
			}
			for i, use := range loop {
				if use.Edge < 0 || use.Edge >= len(s.edges) || !uvFinite(use.UVStart) || !uvFinite(use.UVEnd) || use.UVEnd != loop[(i+1)%len(loop)].UVStart {
					return geom.ErrInvalid
				}
				uses[use.Edge]++
				if uses[use.Edge] > 2 {
					return fmt.Errorf("trimmed: excessive source uses at edge %d: %w", use.Edge, geom.ErrConvergence)
				}
			}
		}
		report, err := ValidateFaceTrimLoops(ctx, face, o.Trims)
		if err != nil {
			return err
		}
		if report.Status != trimvalidate.Certified {
			return fmt.Errorf("trimmed: unresolved source trim loops at face %d: %w", faceIndex, geom.ErrConvergence)
		}
	}
	for _, count := range uses {
		if count == 0 {
			return geom.ErrInvalid
		}
	}
	// The zero value of tolerance on this source view requests exact continuous
	// agreement. Each original model curve is checked before canonicalization.
	agreement, err := s.ValidateEdgeAgreement(ctx, o.Agreement)
	if err != nil {
		return err
	}
	if !agreement.Complete {
		return fmt.Errorf("trimmed: unresolved exact original coedge agreement: %w", geom.ErrConvergence)
	}
	return nil
}

type exactSewModel struct {
	curve    *geom.ExactSplineCurve
	reversed bool
	edge     int
}

func exactSewCurve(ctx context.Context, edge parametric.Edge, endpoints [2]geom.Vec3, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	base, placement := parametric.CurveMapping(edge.Curve)
	var c *geom.ExactSplineCurve
	var reversed bool
	var err error
	switch model := base.(type) {
	case geom.Line:
		if placement != geom.Identity() {
			return nil, false, geom.ErrUnrepresentable
		}
		a, d := model.Origin(), model.Direction()
		for j, t := range [2]float64{edge.Range.Min, edge.Range.Max} {
			position := endpoints[j]
			for axis, origin := range [3]float64{a.X, a.Y, a.Z} {
				velocity := [3]float64{d.X, d.Y, d.Z}[axis]
				p := new(big.Rat).Add(ratInset(origin), new(big.Rat).Mul(ratInset(t), ratInset(velocity)))
				if p.Cmp(ratInset([3]float64{position.X, position.Y, position.Z}[axis])) != 0 {
					return nil, false, geom.ErrUnrepresentable
				}
			}
		}
		spline, e := geom.NewBSpline(1, endpoints[:], []float64{0, 0, 1, 1}, nil)
		if e != nil {
			return nil, false, e
		}
		c, err = geom.NewExactSplineCurve(spline, o)
	case *geom.ExactSplineCurve:
		c, err = model.Restrict(edge.Range.Min, edge.Range.Max, o)
		if err == nil && placement != geom.Identity() {
			c, err = c.Transform(placement, o)
		}
	default:
		c, reversed, err = exactEdgeSplineWithOptions(edge, o)
	}
	if err != nil {
		return nil, false, err
	}
	if c == nil {
		return nil, false, geom.ErrUnrepresentable
	}
	lo, hi := c.Domain()
	if reversed {
		lo, hi = hi, lo
	}
	for i, t := range [2]float64{lo, hi} {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		p, err := c.ExactPoint(t)
		if err != nil {
			return nil, false, err
		}
		if p != endpoints[i] {
			return nil, false, geom.ErrUnrepresentable
		}
	}
	return c, reversed, nil
}

func sewExactModelEdges(ctx context.Context, vertices []geom.Vec3, edges []parametric.Edge, faces []Face, o ExactSewOptions) (Solid, ExactSewReport, error) {
	var empty ExactSewReport
	var models []exactSewModel
	groups := make(map[[2]geom.Vec3][]int)
	report := ExactSewReport{Edges: make([]ExactSewnEdge, len(edges)), Vertices: make([]int, len(vertices))}
	weld := newExactVertexWeld(len(vertices))
	for i, edge := range edges {
		points := [2]geom.Vec3{vertices[edge.Start], vertices[edge.End]}
		curve, reversed, err := exactSewCurve(ctx, edge, points, o.Curve)
		if err != nil {
			return Solid{}, empty, fmt.Errorf("trimmed: source edge %d: %w", i, err)
		}
		key := points
		if exactSewPointLess(points[1], points[0]) {
			key[0], key[1] = key[1], key[0]
		}
		id, flip := -1, false
		for _, candidate := range groups[key] {
			if report.Comparisons >= o.MaxComparisons {
				return Solid{}, empty, geom.ErrConvergence
			}
			report.Comparisons++
			prior := models[candidate]
			other := edges[prior.edge]
			opposite := points[0] != vertices[other.Start]
			cmp, err := geom.CompareExactCurvesContext(ctx, prior.curve, curve, geom.ExactCurveComparisonOptions{
				NormalizeDomains: true, ReverseB: (prior.reversed != reversed) != opposite,
				MaxOperations: o.Curve.MaxOperations, MaxCoefficientBits: o.Curve.MaxCoefficientBits,
			})
			if err != nil {
				return Solid{}, empty, err
			}
			if cmp.Complete && cmp.Equal {
				if id >= 0 {
					return Solid{}, empty, geom.ErrConvergence
				}
				id, flip = candidate, opposite
			}
		}
		if id < 0 {
			id = len(models)
			groups[key] = append(groups[key], id)
			models = append(models, exactSewModel{curve, reversed, i})
		} else {
			other := edges[models[id].edge]
			if flip {
				other.Start, other.End = other.End, other.Start
			}
			weld.join(edge.Start, other.Start)
			weld.join(edge.End, other.End)
		}
		report.Edges[i] = ExactSewnEdge{id, flip}
	}
	usedVertices := make([]bool, len(vertices))
	for _, edge := range edges {
		usedVertices[edge.Start], usedVertices[edge.End] = true, true
	}
	var points []geom.Vec3
	canonical := make(map[int]int)
	for i, p := range vertices {
		if !usedVertices[i] {
			return Solid{}, empty, geom.ErrInvalid
		}
		key := weld.root(i)
		id, exists := canonical[key]
		if !exists {
			id = len(points)
			canonical[key] = id
			points = append(points, p)
		}
		report.Vertices[i] = id
	}
	shared := make([]parametric.Edge, len(models))
	for i, model := range models {
		original := edges[model.edge]
		original.Start = report.Vertices[original.Start]
		original.End = report.Vertices[original.End]
		shared[i] = original
	}
	wires, flipped, err := sewExactOrientation(faces, report.Edges, len(shared), o.RepairOrientation)
	if err != nil {
		return Solid{}, empty, err
	}
	report.FlippedFaces = flipped
	out, err := NewSolidContext(ctx, points, shared, wires, o.DisplayTolerance)
	if err != nil {
		return Solid{}, empty, err
	}
	return out, report, nil
}

func exactSewPointLess(a, b geom.Vec3) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.Z < b.Z
}

type exactVertexWeld []int

func newExactVertexWeld(n int) exactVertexWeld {
	out := make(exactVertexWeld, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func (s exactVertexWeld) root(v int) int {
	for s[v] != v {
		s[v] = s[s[v]]
		v = s[v]
	}
	return v
}

func (s exactVertexWeld) join(a, b int) {
	a, b = s.root(a), s.root(b)
	s[max(a, b)] = min(a, b)
}

func sewExactOrientation(faces []Face, mapping []ExactSewnEdge, count int, repair bool) ([]Face, []int, error) {
	type incidence struct {
		face int
		sign bool
	}
	uses := make([][]incidence, count)
	out := make([]Face, len(faces))
	for i, face := range faces {
		out[i] = cloneFace(face)
		for _, loop := range append([][]Use{out[i].Outer}, out[i].Holes...) {
			for j, original := range loop {
				m := mapping[original.Edge]
				loop[j].Edge = m.Edge
				loop[j].Reversed = original.Reversed != m.Reversed
				uses[m.Edge] = append(uses[m.Edge], incidence{i, loop[j].Reversed != face.Reversed})
			}
		}
	}
	type relation struct {
		face      int
		different bool
	}
	adjacent := make([][]relation, len(faces))
	for edgeID, edge := range uses {
		if len(edge) != 2 {
			return nil, nil, fmt.Errorf("trimmed: sewn edge %d has %d uses: %w", edgeID, len(edge), geom.ErrConvergence)
		}
		a, b := edge[0], edge[1]
		different := a.sign == b.sign
		adjacent[a.face] = append(adjacent[a.face], relation{b.face, different})
		adjacent[b.face] = append(adjacent[b.face], relation{a.face, different})
	}
	visited, flip := make([]bool, len(faces)), make([]bool, len(faces))
	for i := range faces {
		if visited[i] {
			continue
		}
		visited[i] = true
		queue := []int{i}
		for len(queue) > 0 {
			id := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			for _, link := range adjacent[id] {
				wanted := flip[id] != link.different
				if (!repair && wanted) || (visited[link.face] && flip[link.face] != wanted) {
					return nil, nil, fmt.Errorf("trimmed: inconsistent sewn orientation between faces %d and %d: %w", id, link.face, geom.ErrConvergence)
				}
				if !visited[link.face] {
					visited[link.face] = true
					flip[link.face] = wanted
					queue = append(queue, link.face)
				}
			}
		}
	}
	var flipped []int
	for i, value := range flip {
		if value {
			out[i].Reversed = !out[i].Reversed
			flipped = append(flipped, i)
		}
	}
	return out, flipped, nil
}

func validateExactSewnVertexLinks(ctx context.Context, s Solid) error {
	links := make([]map[int][]int, len(s.vertices))
	for i := range links {
		links[i] = make(map[int][]int)
	}
	for _, face := range s.faces {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, loop := range append([][]Use{face.Outer}, face.Holes...) {
			for i, current := range loop {
				prior := loop[(i+len(loop)-1)%len(loop)]
				edge := s.edges[current.Edge]
				id := edge.Start
				if current.Reversed {
					id = edge.End
				}
				adjacent := links[id]
				adjacent[prior.Edge] = append(adjacent[prior.Edge], current.Edge)
				adjacent[current.Edge] = append(adjacent[current.Edge], prior.Edge)
			}
		}
	}
	for i, adjacent := range links {
		if err := ctx.Err(); err != nil {
			return err
		}
		visited := make(map[int]bool, len(adjacent))
		for first := range adjacent {
			queue := []int{first}
			visited[first] = true
			for len(queue) > 0 {
				id := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				if len(adjacent[id]) != 2 {
					return fmt.Errorf("trimmed: nonmanifold link at vertex %d: %w", i, geom.ErrConvergence)
				}
				for _, neighbor := range adjacent[id] {
					if !visited[neighbor] {
						visited[neighbor] = true
						queue = append(queue, neighbor)
					}
				}
			}
			break
		}
		if len(visited) != len(adjacent) {
			return fmt.Errorf("trimmed: disconnected link at vertex %d: %w", i, geom.ErrConvergence)
		}
	}
	return nil
}
