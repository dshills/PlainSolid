package trimmed

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

var errCompactBoundaryLimit = fmt.Errorf("trimmed: compact boundary sampling limit")

// compactCylinderMesh specializes flat faces joined to rectangular cylinder
// patches. Curvature only needs angular subdivisions: straight edges and the
// axial direction need none. It retains shared model-edge samples and subjects
// every emitted triangle to the ordinary sampled deflection/normal checks.
func (s Solid) compactCylinderMesh(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, bool, error) {
	if o.AnalyticBound {
		return mesh.Mesh{}, false, nil
	}
	curved := make([]bool, len(s.edges))
	found := false
	for _, f := range s.faces {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, true, err
		}
		switch f.Surface.(type) {
		case geom.PlaneSurface:
		case geom.CylinderSurface:
			if len(f.Holes) != 0 || len(f.Outer) != 4 {
				return mesh.Mesh{}, false, nil
			}
			for _, u := range f.Outer {
				if u.Curve != nil {
					return mesh.Mesh{}, false, nil
				}
			}
			a, b, c, d := f.Outer[0].UVStart, f.Outer[1].UVStart, f.Outer[2].UVStart, f.Outer[3].UVStart
			if a.Y != b.Y || b.X != c.X || c.Y != d.Y || d.X != a.X || a.X == b.X || a.Y == d.Y || math.Abs(b.X-a.X) > math.Pi/2 {
				return mesh.Mesh{}, false, nil
			}
			curved[f.Outer[0].Edge] = true
			curved[f.Outer[2].Edge] = true
			found = true
		default:
			return mesh.Mesh{}, false, nil
		}
	}
	if !found {
		return mesh.Mesh{}, false, nil
	}
	for i, e := range s.edges {
		if !curved[i] {
			if _, ok := e.Curve.(geom.Line); !ok {
				return mesh.Mesh{}, false, nil
			}
		}
	}
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, true, err
		}
		m, err := s.compactCylinderCandidate(ctx, n, curved, o)
		if err == errCompactBoundaryLimit {
			return mesh.Mesh{}, false, nil
		}
		if err == errRefine {
			continue
		}
		return m, true, err
	}
	return mesh.Mesh{}, true, parametric.ErrBudget
}

func (s Solid) compactCylinderCandidate(ctx context.Context, n int, curved []bool, o parametric.TessellationOptions) (mesh.Mesh, error) {
	// Bound all face triangulations before allocating edge samples. A polygon
	// with h holes and v boundary vertices has v+2h-2 triangles; rectangular
	// cylinders use two triangles per angular strip.
	total := 0
	for _, f := range s.faces {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		vertices := 0
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				count := 1
				if curved[u.Edge] {
					count = n
				}
				vertices += count
			}
		}
		if vertices > brep.MaxHoledFaceVertices {
			return mesh.Mesh{}, errCompactBoundaryLimit
		}
		count := vertices + 2*len(f.Holes) - 2
		if _, ok := f.Surface.(geom.CylinderSurface); ok {
			count = 2 * n
		}
		if count > o.MaxTriangles-total {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		total += count
	}
	m := mesh.Mesh{Vertices: s.Vertices()}
	edgeIDs := make([][]int, len(s.edges))
	for i, e := range s.edges {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		count := 1
		if curved[i] {
			count = n
		}
		ids := make([]int, count+1)
		ids[0], ids[count] = e.Start, e.End
		for j := 1; j < count; j++ {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			p, err := s.onEdge(i, float64(j)/float64(count))
			if err != nil || !p.IsFinite() {
				return mesh.Mesh{}, fmt.Errorf("trimmed: invalid compact model edge")
			}
			ids[j] = len(m.Vertices)
			m.Vertices = append(m.Vertices, p)
		}
		edgeIDs[i] = ids
	}
	type boundarySide struct {
		side       trimSide
		start, end float64
	}
	for fi, f := range s.faces {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		loops := append([][]Use{f.Outer}, f.Holes...)
		rings := make([][]geom.Vec3, len(loops))
		indices := map[geom.Vec2]int{}
		sides := map[[2]geom.Vec2]boundarySide{}
		outer := make([][]geom.Vec2, len(f.Outer))
		for ri, loop := range loops {
			for ui, u := range loop {
				count := len(edgeIDs[u.Edge]) - 1
				if len(indices)+count > brep.MaxHoledFaceVertices {
					return mesh.Mesh{}, errCompactBoundaryLimit
				}
				uv := make([]geom.Vec2, count+1)
				for j := 0; j <= count; j++ {
					p, err := u.uv(float64(j) / float64(count))
					if err != nil {
						return mesh.Mesh{}, err
					}
					uv[j] = p
				}
				uv[0], uv[count] = u.UVStart, u.UVEnd
				if ri == 0 {
					outer[ui] = uv
				}
				for j := 0; j < count; j++ {
					p, q := uv[j], uv[j+1]
					k := j
					t, end := float64(j)/float64(count), float64(j+1)/float64(count)
					if u.Reversed {
						k = count - j
						t, end = 1-t, 1-end
					}
					id := edgeIDs[u.Edge][k]
					model, err := f.Surface.Eval(p.X, p.Y)
					if err != nil || !closePoint(model, m.Vertices[id], s.tolerance) {
						return mesh.Mesh{}, fmt.Errorf("trimmed: compact face %d disagrees with edge %d", fi, u.Edge)
					}
					if _, exists := indices[p]; exists {
						return mesh.Mesh{}, fmt.Errorf("trimmed: repeated compact UV vertex")
					}
					indices[p] = id
					rings[ri] = append(rings[ri], geom.Vec3{X: p.X, Y: p.Y})
					sides[[2]geom.Vec2{p, q}] = boundarySide{trimSide{u.Edge, u}, t, end}
				}
			}
		}
		var triangles [][3]geom.Vec2
		if _, ok := f.Surface.(geom.CylinderSurface); ok {
			if len(outer[0]) != n+1 || len(outer[2]) != n+1 || len(outer[1]) != 2 || len(outer[3]) != 2 {
				return mesh.Mesh{}, fmt.Errorf("trimmed: incompatible compact cylinder edges")
			}
			for i := 0; i < n; i++ {
				a, b, c, d := outer[0][i], outer[0][i+1], outer[2][n-i-1], outer[2][n-i]
				triangles = append(triangles, [3]geom.Vec2{a, b, c}, [3]geom.Vec2{a, c, d})
			}
		} else {
			var err error
			triangles, err = triangulateUV(rings)
			if err != nil {
				return mesh.Mesh{}, err
			}
		}
		if len(triangles) > o.MaxTriangles-len(m.Triangles) {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		for _, tri := range triangles {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			t := samples{uv: tri, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
			var p [3]geom.Vec3
			for j, uv := range tri {
				id, ok := indices[uv]
				if !ok {
					return mesh.Mesh{}, fmt.Errorf("trimmed: missing compact UV vertex")
				}
				t.ids[j] = id
				p[j] = m.Vertices[id]
				if side, ok := sides[[2]geom.Vec2{uv, tri[(j+1)%3]}]; ok {
					t.edge[j], t.start[j], t.end[j] = side.side, side.start, side.end
				}
			}
			if err := check(f.Surface, t, p, o.Deflection, o.Angle); err != nil {
				return mesh.Mesh{}, err
			}
			ids := t.ids
			if f.Reversed {
				ids[1], ids[2] = ids[2], ids[1]
			}
			m.Triangles = append(m.Triangles, ids)
		}
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}
