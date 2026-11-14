package trimmed

import (
	"context"

	"plainsolid/geom"
)

// AlgebraicGraphShellUse is an oriented occurrence of a shared result edge.
// The ordered edge IDs around a face are kept even where its corners require
// algebraic model coordinates.
type AlgebraicGraphShellUse struct {
	Edge     int
	Reversed bool
}

type AlgebraicGraphShellEdge struct {
	Start, End  int
	StockEdge   int // original stock perimeter edge, or -1
	CapSeam     int // paired cap-to-cap seam, or -1
	Cap         int // cap-side attachment, or -1
	CapBoundary AlgebraicGraphCapSeamOwner
}
type AlgebraicGraphShellFace struct {
	StockFace int // -1 for a generated cap boundary group
	Cap       int // -1 for original stock
	Loops     [][]AlgebraicGraphShellUse
}

// AlgebraicGraphShellBoundary is the closed oriented topology of the selected
// halfspace intersection. All original geometry remains on its source charts.
type AlgebraicGraphShellBoundary struct {
	Faces               []AlgebraicGraphShellFace
	Edges               []AlgebraicGraphShellEdge
	Vertices            int
	BoundaryComponents  int
	EulerCharacteristic int
}

// ExactIndexedBoundary stitches the proved cap seams, exact original stock
// attachments, and inherited perimeter intervals by ownership, once both
// charts agree. It rechecks all directed links. The returned object is exact
// topology and original ancestry; it does not construct an ordinary Solid.
// Cap loops remain separate boundary components of their plane role until
// explicitly grouped by exact nesting. EulerCharacteristic is available
// only when every cap has one retained ring (zero otherwise).
func (c AlgebraicGraphCapPair) ExactIndexedBoundary(ctx context.Context, o RationalGraphShellSectionOptions, roots geom.ExactCurvePlaneEventOptions) (AlgebraicGraphShellBoundary, error) {
	var zero AlgebraicGraphShellBoundary
	stock, e := c.OriginalStockLoops(ctx, o, roots)
	if e != nil {
		return zero, e
	}
	type capKey struct{ cap, wire, use int }
	seamID := make(map[capKey]int)
	var out AlgebraicGraphShellBoundary
	for i, s := range c.seams {
		for j, a := range s.Owners {
			seamID[capKey{j, a.Wire, a.Use}] = len(out.Edges)
		}
		out.Edges = append(out.Edges, AlgebraicGraphShellEdge{StockEdge: -1, CapSeam: i, Cap: -1})
	}
	attachID := make(map[capKey]int)
	for i, cap := range c.caps {
		f := AlgebraicGraphShellFace{StockFace: -1, Cap: i}
		for wi, wire := range cap.wires {
			var row []AlgebraicGraphShellUse
			for ui, u := range wire {
				k := capKey{i, wi, ui}
				if u.Seam >= 0 {
					id, exists := seamID[k]
					if !exists {
						return zero, geom.ErrConvergence
					}
					row = append(row, AlgebraicGraphShellUse{id, i == 1})
					continue
				}
				id := len(out.Edges)
				attachID[k] = id
				out.Edges = append(out.Edges, AlgebraicGraphShellEdge{StockEdge: -1, CapSeam: -1, Cap: i, CapBoundary: AlgebraicGraphCapSeamOwner{wi, ui}})
				row = append(row, AlgebraicGraphShellUse{id, false})
			}
			f.Loops = append(f.Loops, row)
		}
		out.Faces = append(out.Faces, f)
	}
	type perimeter struct {
		edge         int
		lower, upper string
	}
	perimeterIDs := make(map[perimeter]int)
	for _, face := range stock {
		f := AlgebraicGraphShellFace{StockFace: face.OriginalFace, Cap: -1, Loops: make([][]AlgebraicGraphShellUse, 1)}
		for _, side := range face.Uses {
			id, reverse := -1, true
			if side.OriginalEdge >= 0 {
				k := perimeter{side.OriginalEdge, side.PerimeterParameters[0].RatString(), side.PerimeterParameters[1].RatString()}
				found, ok := perimeterIDs[k]
				if !ok {
					found = len(out.Edges)
					perimeterIDs[k] = found
					out.Edges = append(out.Edges, AlgebraicGraphShellEdge{StockEdge: side.OriginalEdge, CapSeam: -1, Cap: -1})
				}
				id, reverse = found, side.PerimeterReversed != face.Reversed
			} else {
				var ok bool
				id, ok = attachID[capKey{side.Cap, side.Owner.Wire, side.Owner.Use}]
				if !ok {
					return zero, geom.ErrConvergence
				}
			}
			f.Loops[0] = append(f.Loops[0], AlgebraicGraphShellUse{id, reverse})
		}
		if face.Reversed {
			row := f.Loops[0]
			for i, j := 0, len(row)-1; i < j; i, j = i+1, j-1 {
				row[i], row[j] = row[j], row[i]
			}
		}
		out.Faces = append(out.Faces, f)
	}
	return completeAlgebraicGraphShell(ctx, out)
}

// Complete indexed stitching identifies vertices only by adjacent uses in a
// verified loop, never by the distance between represented model points.
func completeAlgebraicGraphShell(ctx context.Context, out AlgebraicGraphShellBoundary) (AlgebraicGraphShellBoundary, error) {
	var zero AlgebraicGraphShellBoundary
	parent := make([]int, 2*len(out.Edges))
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
	var fans = make([]int, len(parent))
	for i := range fans {
		fans[i] = -1
	}
	count := make([][2]int, len(out.Edges))
	ends := func(u AlgebraicGraphShellUse) (a, b int) {
		a, b = 2*u.Edge, 2*u.Edge+1
		if u.Reversed {
			a, b = b, a
		}
		return a, b
	}
	for _, face := range out.Faces {
		for _, loop := range face.Loops {
			if len(loop) < 2 {
				return zero, geom.ErrConvergence
			}
			for i, u := range loop {
				if e := ctx.Err(); e != nil {
					return zero, e
				}
				d := 0
				if u.Reversed {
					d = 1
				}
				count[u.Edge][d]++
				_, b := ends(u)
				a, _ := ends(loop[(i+1)%len(loop)])
				if fans[b] >= 0 {
					return zero, geom.ErrConvergence
				}
				fans[b] = a
				parent[find(b)] = find(a)
			}
		}
	}
	vertices := make(map[int]int)
	for i, counts := range count {
		if counts != [2]int{1, 1} {
			return zero, geom.ErrConvergence
		}
		var ends [2]int
		for j := range ends {
			key := find(2*i + j)
			id, ok := vertices[key]
			if !ok {
				id = len(vertices)
				vertices[key] = id
			}
			ends[j] = id
		}
		if ends[0] == ends[1] {
			return zero, geom.ErrConvergence
		}
		out.Edges[i].Start, out.Edges[i].End = ends[0], ends[1]
	}
	// Each vertex link must be one cycle of incident edge ends.
	fanChecked := make(map[int]bool)
	seen := make([]bool, len(fans))
	for at := range fans {
		if seen[at] {
			continue
		}
		key := find(at)
		if fanChecked[key] {
			return zero, geom.ErrConvergence
		}
		fanChecked[key] = true
		for i := at; ; i = fans[i] {
			if seen[i] || find(i) != key || fans[i] < 0 {
				return zero, geom.ErrConvergence
			}
			seen[i] = true
			if fans[i] == at {
				break
			}
		}
	}
	out.Vertices = len(vertices)
	graph := make([][]int, len(vertices))
	for _, edge := range out.Edges {
		graph[edge.Start] = append(graph[edge.Start], edge.End)
		graph[edge.End] = append(graph[edge.End], edge.Start)
	}
	visited := make([]bool, len(graph))
	for start := range graph {
		if visited[start] {
			continue
		}
		out.BoundaryComponents++
		stack := []int{start}
		visited[start] = true
		for len(stack) > 0 {
			z := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, next := range graph[z] {
				if !visited[next] {
					visited[next] = true
					stack = append(stack, next)
				}
			}
		}
	}
	if len(out.Faces) >= 2 && len(out.Faces[0].Loops) == 1 && len(out.Faces[1].Loops) == 1 {
		out.EulerCharacteristic = out.Vertices - len(out.Edges) + len(out.Faces)
	}
	return out, nil
}
