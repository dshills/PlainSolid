package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type WrappedFaceRegionSelection uint8

const (
	WrappedFaceRegionUnion WrappedFaceRegionSelection = iota + 1
	WrappedFaceRegionIntersection
	WrappedFaceRegionDifference
)

// WrappedRegionCellAncestry keeps the original arrangement cell and all lifted
// owner corners. A cell belonging to both inputs appears once, with Owners=3.
type WrappedRegionCellAncestry struct {
	SourceCell int
	Owners     uint8
	OwnerUV    [2][4][2]*big.Rat
}

// WrappedFaceRegionTopology is an immutable embedded, oriented open region made
// of exact cofacial rectangular cells. It is not a Solid, material union, or
// volume certificate. Faces use A's original wrapped support and Reversed flag.
// Original B's orientation is retained separately and never silently reconciled.
// Exported edges are unique; each has one boundary or two opposite face uses.
// The exterior and any hole are directed consistently with the chosen surface.
// A successful empty Difference has no cells, edges or boundary loops.
type WrappedFaceRegionTopology struct {
	vertices   []geom.Vec3
	edges      []parametric.Edge
	faces      []Face
	boundaries [][]Use
	exterior   []Use
	cells      []WrappedRegionCellAncestry
	ancestry   [][]WrappedFaceFragmentOwner
	reversed   [2]bool
	operations int
}

func (r *WrappedFaceRegionTopology) Vertices() []geom.Vec3 {
	if r == nil {
		return nil
	}
	return append([]geom.Vec3(nil), r.vertices...)
}
func (r *WrappedFaceRegionTopology) Edges() []parametric.Edge {
	if r == nil {
		return nil
	}
	return append([]parametric.Edge(nil), r.edges...)
}
func (r *WrappedFaceRegionTopology) Faces() []Face {
	if r == nil {
		return nil
	}
	out := make([]Face, len(r.faces))
	for i, f := range r.faces {
		out[i] = cloneFace(f)
	}
	return out
}

// Exterior returns a single closed cycle using global edge indices. UVStart /
// UVEnd and Reversed both reflect A's physical face orientation; unlike a Face
// outer loop, this traversal is clockwise in UV when A.Reversed is true.
func (r *WrappedFaceRegionTopology) Exterior() []Use {
	if r == nil {
		return nil
	}
	return append([]Use(nil), r.exterior...)
}
func (r *WrappedFaceRegionTopology) SourceOrientations() [2]bool {
	if r == nil {
		return [2]bool{}
	}
	return r.reversed
}

// ExactOperations reports the bounded source geometry/materialization proof.
// Incidence validation is additionally bounded by nine faces and 36 edges.
func (r *WrappedFaceRegionTopology) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}
func (r *WrappedFaceRegionTopology) CellAncestry() []WrappedRegionCellAncestry {
	if r == nil {
		return nil
	}
	out := make([]WrappedRegionCellAncestry, len(r.cells))
	for i, c := range r.cells {
		out[i] = c
		for owner := range c.OwnerUV {
			for corner := range c.OwnerUV[owner] {
				for axis, q := range c.OwnerUV[owner][corner] {
					if q != nil {
						out[i].OwnerUV[owner][corner][axis] = new(big.Rat).Set(q)
					}
				}
			}
		}
	}
	return out
}
func copyWrappedFragmentOwner(p WrappedFaceFragmentOwner) WrappedFaceFragmentOwner {
	for i := 0; i < 2; i++ {
		p.Parameters[i] = new(big.Rat).Set(p.Parameters[i])
		for j := 0; j < 2; j++ {
			p.UV[i][j] = new(big.Rat).Set(p.UV[i][j])
		}
	}
	return p
}

// EdgeAncestry follows canonical stored edge direction, not individual use
// direction. Internal subdivision edges may have no original boundary owner.
func (r *WrappedFaceRegionTopology) EdgeAncestry() [][]WrappedFaceFragmentOwner {
	if r == nil {
		return nil
	}
	out := make([][]WrappedFaceFragmentOwner, len(r.ancestry))
	for i, list := range r.ancestry {
		for _, p := range list {
			out[i] = append(out[i], copyWrappedFragmentOwner(p))
		}
	}
	return out
}

// NewWrappedFaceRegionTopology revalidates source geometry and model-edge mates
// rather than accepting a caller-editable Complete/Cells report. The fixed
// scope is at most nine cells. Edge reuse additionally checks exact affine UV
// identity and the certified reverse cell mate. Vertex fans, global incidence
// and simple boundary cycles are checked independently before publication.
// Difference is directed A minus B, retaining the closure of its 2D cells.
// It supports a connected disk or disk with one hole; disconnected or pinched
// results remain unresolved. Empty() distinguishes a proved empty Difference.
// The embedding guard still rejects hulls crossing a fundamental seam;
// different whole-period lifted owner charts remain supported.
func NewWrappedFaceRegionTopology(ctx context.Context, a, b SewFace, selection WrappedFaceRegionSelection, o WrappedRectangleFaceOverlapOptions) (*WrappedFaceRegionTopology, error) {
	if selection != WrappedFaceRegionUnion && selection != WrappedFaceRegionIntersection && selection != WrappedFaceRegionDifference {
		return nil, geom.ErrInvalid
	}
	material, e := MaterializeWrappedRectangleFaceCells(ctx, a, b, o)
	if e != nil {
		return nil, e
	}
	result := &WrappedFaceRegionTopology{reversed: material.FaceReversed, operations: material.ExactOperations}
	type occurrence struct{ cell, side, edge int }
	selected := map[int]bool{}
	for i, c := range material.Cells {
		if selection == WrappedFaceRegionUnion || (selection == WrappedFaceRegionIntersection && c.Owners == 3) || (selection == WrappedFaceRegionDifference && c.Owners == 1) {
			selected[i] = true
		}
	}
	if len(selected) == 0 {
		if selection != WrappedFaceRegionDifference {
			return nil, geom.ErrConvergence
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		return result, nil
	}
	vertexIDs := map[geom.Vec2]int{}
	modelIDs := map[geom.Vec3]geom.Vec2{}
	edgeIDs := map[[2]int]occurrence{}
	for ci, c := range material.Cells {
		if !selected[ci] {
			continue
		}
		f := Face{Surface: c.Face.Face.Surface, Reversed: c.Face.Face.Reversed}
		ids := [4]int{}
		for i, u := range c.Face.Face.Outer {
			uv := u.UVStart
			point := c.Face.Vertices[i]
			if old, ok := modelIDs[point]; ok && old != uv {
				return nil, geom.ErrConvergence
			}
			modelIDs[point] = uv
			id, ok := vertexIDs[uv]
			if !ok {
				id = len(result.vertices)
				vertexIDs[uv] = id
				result.vertices = append(result.vertices, point)
			} else if result.vertices[id] != point {
				return nil, geom.ErrConvergence
			}
			ids[i] = id
		}
		for i, use := range c.Face.Face.Outer {
			j := (i + 1) % 4
			start, end := ids[i], ids[j]
			key := [2]int{start, end}
			if end < start {
				key = [2]int{end, start}
			}
			old, reuse := edgeIDs[key]
			id := len(result.edges)
			reverse := false
			if reuse {
				side := c.Sides[i]
				mate := material.Cells[old.cell].Sides[old.side]
				if side.MateCell != old.cell || side.MateSide != old.side || mate.MateCell != ci || mate.MateSide != i {
					return nil, geom.ErrConvergence
				}
				id = old.edge
				prior := result.edges[id]
				if prior.Start != end || prior.End != start {
					return nil, geom.ErrConvergence
				}
				reverse = true
				x, ok := analyticModelLine(prior)
				y, valid := analyticModelLine(c.Face.Edges[i])
				if !ok || !valid || x.surface != y.surface {
					return nil, geom.ErrConvergence
				}
				for axis := 0; axis < 2; axis++ {
					if x.uv[0][axis].Cmp(y.uv[1][axis]) != 0 || x.uv[1][axis].Cmp(y.uv[0][axis]) != 0 {
						return nil, geom.ErrConvergence
					}
				}
			} else {
				edge := c.Face.Edges[i]
				edge.Start = start
				edge.End = end
				result.edges = append(result.edges, edge)
				result.ancestry = append(result.ancestry, nil)
				edgeIDs[key] = occurrence{ci, i, id}
			}
			use.Edge = id
			use.Reversed = reverse
			f.Outer = append(f.Outer, use)
			originals := append([]WrappedFaceFragmentOwner(nil), c.Sides[i].Original...)
			if selection == WrappedFaceRegionDifference {
				side := c.Sides[i]
				if side.MateCell >= 0 && !selected[side.MateCell] {
					mate := material.Cells[side.MateCell].Sides[side.MateSide]
					if mate.MateCell != ci || mate.MateSide != i {
						return nil, geom.ErrConvergence
					}
					for _, source := range mate.Original {
						if source.Face == 1 {
							p := copyWrappedFragmentOwner(source)
							p.Parameters[0], p.Parameters[1] = p.Parameters[1], p.Parameters[0]
							p.UV[0], p.UV[1] = p.UV[1], p.UV[0]
							originals = append(originals, p)
						}
					}
				}
			}
			for _, source := range originals {
				p := copyWrappedFragmentOwner(source)
				if reverse {
					p.Parameters[0], p.Parameters[1] = p.Parameters[1], p.Parameters[0]
					p.UV[0], p.UV[1] = p.UV[1], p.UV[0]
				}
				duplicate := false
				for _, q := range result.ancestry[id] {
					if p.Face != q.Face || p.Use != q.Use || p.Edge != q.Edge {
						continue
					}
					duplicate = true
					for end := 0; end < 2; end++ {
						if p.Parameters[end].Cmp(q.Parameters[end]) != 0 {
							return nil, geom.ErrConvergence
						}
						for axis := 0; axis < 2; axis++ {
							if p.UV[end][axis].Cmp(q.UV[end][axis]) != 0 {
								return nil, geom.ErrConvergence
							}
						}
					}
				}
				if !duplicate {
					result.ancestry[id] = append(result.ancestry[id], p)
				}
			}
		}
		result.faces = append(result.faces, f)
		result.cells = append(result.cells, WrappedRegionCellAncestry{SourceCell: ci, Owners: c.Owners, OwnerUV: c.OwnerUV})
	}
	maxCycles := 1
	if selection == WrappedFaceRegionDifference {
		maxCycles = 2
	}
	cycles, e := validateWrappedOpenRegionCycles(ctx, len(result.vertices), result.edges, result.faces, maxCycles)
	if e != nil {
		return nil, e
	}
	for _, cycle := range cycles {
		for _, use := range cycle {
			if len(result.ancestry[use.Edge]) == 0 {
				return nil, geom.ErrConvergence
			}
		}
	}
	result.boundaries = cycles
	result.exterior = cycles[0]
	return result, nil
}

// Pure bounded topological validation. Geometry identity is established by the
// constructor above; endpoint incidence alone is never used as a curve proof.
func validateWrappedOpenRegionIncidence(ctx context.Context, nv int, edges []parametric.Edge, faces []Face) ([]Use, error) {
	cycles, e := validateWrappedOpenRegionCycles(ctx, nv, edges, faces, 1)
	if e != nil {
		return nil, e
	}
	return cycles[0], nil
}
func validateWrappedOpenRegionCycles(ctx context.Context, nv int, edges []parametric.Edge, faces []Face, maxCycles int) ([][]Use, error) {
	if maxCycles < 1 || maxCycles > 2 || ctx == nil || nv < 4 || nv > 36 || len(edges) < 4 || len(edges) > 36 || len(faces) < 1 || len(faces) > 9 {
		return nil, geom.ErrInvalid
	}
	type incidence struct {
		face, start, end int
		use              Use
	}
	incidents := make([][]incidence, len(edges))
	vertexFaces := make([]map[int]bool, nv)
	vertexUV := make(map[int]geom.Vec2)
	for fi, f := range faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if len(f.Outer) != 4 || len(f.Holes) != 0 || f.Reversed != faces[0].Reversed {
			return nil, geom.ErrConvergence
		}
		seen := map[int]bool{}
		previous, first := -1, -1
		for _, u := range f.Outer {
			if u.Edge < 0 || u.Edge >= len(edges) || seen[u.Edge] {
				return nil, geom.ErrConvergence
			}
			seen[u.Edge] = true
			edge := edges[u.Edge]
			start, end := edge.Start, edge.End
			if u.Reversed {
				start, end = end, start
			}
			if start < 0 || end < 0 || start >= nv || end >= nv || start == end {
				return nil, geom.ErrConvergence
			}
			if u.Curve != nil || !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) {
				return nil, geom.ErrConvergence
			}
			for j, v := range [2]int{start, end} {
				uv := u.UVStart
				if j == 1 {
					uv = u.UVEnd
				}
				if old, ok := vertexUV[v]; ok && old != uv {
					return nil, geom.ErrConvergence
				}
				vertexUV[v] = uv
			}
			if first < 0 {
				first = start
			} else if previous != start {
				return nil, geom.ErrConvergence
			}
			previous = end
			if vertexFaces[start] == nil {
				vertexFaces[start] = map[int]bool{}
			}
			vertexFaces[start][fi] = true
			if f.Reversed {
				start, end = end, start
				u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
				u.Reversed = !u.Reversed
			}
			incidents[u.Edge] = append(incidents[u.Edge], incidence{fi, start, end, u})
		}
		if previous != first || len(seen) != 4 {
			return nil, geom.ErrConvergence
		}
	}
	adj := make([]map[int]bool, len(faces))
	boundaryAt := make([]int, nv)
	var boundary []incidence
	for _, list := range incidents {
		switch len(list) {
		case 1:
			p := list[0]
			boundary = append(boundary, p)
			boundaryAt[p.start]++
			boundaryAt[p.end]++
		case 2:
			p, q := list[0], list[1]
			if p.face == q.face || p.start != q.end || p.end != q.start {
				return nil, geom.ErrConvergence
			}
			if adj[p.face] == nil {
				adj[p.face] = map[int]bool{}
			}
			if adj[q.face] == nil {
				adj[q.face] = map[int]bool{}
			}
			adj[p.face][q.face] = true
			adj[q.face][p.face] = true
		default:
			return nil, geom.ErrConvergence
		}
	}
	connected := func(nodes map[int]bool, neighbors func(int, int) bool) bool {
		start := -1
		for n := range nodes {
			start = n
			break
		}
		if start < 0 {
			return false
		}
		seen := map[int]bool{start: true}
		queue := []int{start}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for q := range nodes {
				if !seen[q] && neighbors(n, q) {
					seen[q] = true
					queue = append(queue, q)
				}
			}
		}
		return len(seen) == len(nodes)
	}
	all := map[int]bool{}
	for i := range faces {
		all[i] = true
	}
	if !connected(all, func(a, b int) bool { return adj[a][b] }) {
		return nil, geom.ErrConvergence
	}
	for v, nodes := range vertexFaces {
		if boundaryAt[v] != 0 && boundaryAt[v] != 2 {
			return nil, geom.ErrConvergence
		}
		if !connected(nodes, func(a, b int) bool {
			for _, list := range incidents {
				if len(list) == 2 && (list[0].start == v || list[0].end == v) && ((list[0].face == a && list[1].face == b) || (list[0].face == b && list[1].face == a)) {
					return true
				}
			}
			return false
		}) {
			return nil, geom.ErrConvergence
		}
	}
	if len(boundary) < 4 {
		return nil, geom.ErrConvergence
	}
	byStart := map[int]incidence{}
	for _, p := range boundary {
		if _, exists := byStart[p.start]; exists {
			return nil, geom.ErrConvergence
		}
		byStart[p.start] = p
	}
	var cycles [][]Use
	for len(byStart) > 0 {
		if len(cycles) >= maxCycles {
			return nil, geom.ErrConvergence
		}
		start := nv
		for v := range byStart {
			if v < start {
				start = v
			}
		}
		at := start
		var cycle []Use
		for step := 0; step < len(boundary); step++ {
			p, ok := byStart[at]
			if !ok {
				return nil, geom.ErrConvergence
			}
			cycle = append(cycle, p.use)
			delete(byStart, at)
			at = p.end
			if at == start {
				break
			}
		}
		if at != start || len(cycle) < 4 {
			return nil, geom.ErrConvergence
		}
		cycles = append(cycles, cycle)
	}
	if nv-len(edges)+len(faces) != 2-len(cycles) {
		return nil, geom.ErrConvergence
	}
	cycles, e := validateWrappedBoundaryCycles(cycles, faces[0].Reversed)
	if e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return cycles, nil
}
