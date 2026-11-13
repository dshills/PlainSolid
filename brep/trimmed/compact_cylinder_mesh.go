package trimmed

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const maxCompactFaceVertices = 1024

var errCompactBoundaryLimit = fmt.Errorf("trimmed: compact boundary sampling limit")

// compactCylinderMesh specializes flat faces joined to rectangular cylinder
// patches, including mapped analytic supports. It subdivides the angular
// direction and leaves straight edges intact. All faces share model-edge
// samples and use the ordinary sampled deflection and normal checks.
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
		surface, _ := parametric.SurfaceMapping(f.Surface)
		switch surface.(type) {
		case geom.PlaneSurface:
		case geom.TorusSurface:
			if !compactTorus(f) {
				return mesh.Mesh{}, false, nil
			}
			for _, u := range f.Outer {
				curved[u.Edge] = true
			}
			found = true
		default:
			_, ok := compactAxialStart(f)
			if !ok {
				return mesh.Mesh{}, false, nil
			}
			for _, u := range f.Outer {
				if u.UVStart.Y == u.UVEnd.Y {
					curved[u.Edge] = true
				}
			}
			found = true
		}
	}
	if !found {
		return mesh.Mesh{}, false, nil
	}
	for i, e := range s.edges {
		if !curved[i] {
			if !compactStraightEdge(e.Curve) {
				return mesh.Mesh{}, false, nil
			}
		}
	}
	groups := compactEdgeGroups(s.faces, curved)
	levels := make([]int, len(curved))
	for i := range levels {
		levels[i] = 1
	}
	for iteration := 0; iteration <= len(s.edges)*11; iteration++ {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, true, err
		}
		counts := s.compactEdgeCounts(levels, curved)
		m, err := s.compactCandidateCounts(ctx, counts, o)
		if err == errCompactBoundaryLimit {
			return mesh.Mesh{}, false, nil
		}
		if refine, ok := err.(*compactRefinement); ok {
			roots := map[int]bool{}
			for edge := range refine.edges {
				roots[groups[edge]] = true
			}
			if len(roots) == 0 {
				return mesh.Mesh{}, true, parametric.ErrBudget
			}
			for i := range levels {
				if roots[groups[i]] && curved[i] {
					if levels[i] > o.MaxSubdivisions/2 {
						return mesh.Mesh{}, true, parametric.ErrBudget
					}
					levels[i] *= 2
				}
			}
			continue
		}
		return m, true, err
	}
	return mesh.Mesh{}, true, parametric.ErrBudget
}

// A rectangular cylindrical chart may start at either a rim or an axial seam.
// Start the strip triangulation on a rim while keeping the oriented loop.
func compactCylinderStart(f Face) (int, bool) {
	if len(f.Holes) != 0 || len(f.Outer) != 4 {
		return 0, false
	}
	for _, u := range f.Outer {
		if u.Curve != nil {
			return 0, false
		}
	}
	start := 0
	if f.Outer[0].UVStart.Y != f.Outer[0].UVEnd.Y {
		start = 1
	}
	a, b := f.Outer[start].UVStart, f.Outer[(start+1)%4].UVStart
	c, d := f.Outer[(start+2)%4].UVStart, f.Outer[(start+3)%4].UVStart
	return start, a.Y == b.Y && b.X == c.X && c.Y == d.Y && d.X == a.X &&
		a.X != b.X && a.Y != d.Y && math.Abs(b.X-a.X) <= math.Pi/2
}

func compactStraightEdge(curve geom.Curve) bool {
	base, _ := parametric.CurveMapping(curve)
	if parametric.IsRevolutionGenerator(base) {
		return true
	}
	switch c := base.(type) {
	case geom.Line:
		return true
	case surfaceTrimCurve:
		if c.curve != nil {
			return false
		}
		surface, _ := parametric.SurfaceMapping(c.surface)
		switch surface.(type) {
		case geom.PlaneSurface:
			return true
		case geom.CylinderSurface:
			return c.a.X == c.b.X
		}
	}
	return false
}

func (s Solid) compactCylinderCandidate(ctx context.Context, n int, curved []bool, o parametric.TessellationOptions) (mesh.Mesh, error) {

	levels := make([]int, len(curved))
	for i := range levels {
		levels[i] = n
	}
	m, err := s.compactCandidateCounts(ctx, s.compactEdgeCounts(levels, curved), o)
	if _, ok := err.(*compactRefinement); ok {
		return mesh.Mesh{}, errRefine
	}
	return m, err
}

func (s Solid) compactCandidateCounts(ctx context.Context, counts []int, o parametric.TessellationOptions) (mesh.Mesh, error) {
	failures := &compactRefinement{edges: map[int]bool{}}

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
				count := counts[u.Edge]
				vertices += count
			}
		}
		if vertices > maxCompactFaceVertices {
			return mesh.Mesh{}, errCompactBoundaryLimit
		}
		count := vertices + 2*len(f.Holes) - 2
		if compactTorus(f) {
			count = 2 * counts[f.Outer[0].Edge] * counts[f.Outer[1].Edge]
		}
		if _, ok := compactAxialStart(f); ok {
			count = 0
			for _, u := range f.Outer {
				if u.UVStart.Y == u.UVEnd.Y {
					count += counts[u.Edge]
				}
			}
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
		count := counts[i]
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
		start, cylinder := compactAxialStart(f)
		for ri, loop := range loops {
			for ui := range loop {
				u := loop[ui]
				if ri == 0 && start != 0 {
					u = loop[(ui+start)%len(loop)]
				}
				count := len(edgeIDs[u.Edge]) - 1
				if len(indices)+count > maxCompactFaceVertices {
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
		if compactTorus(f) {
			var err error
			triangles, err = compactTorusGrid(outer, func(p geom.Vec2) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				world, err := f.Surface.Eval(p.X, p.Y)
				if err != nil || !world.IsFinite() {
					return geom.ErrInvalid
				}
				indices[p] = len(m.Vertices)
				m.Vertices = append(m.Vertices, world)
				return nil
			})
			if err != nil {
				return mesh.Mesh{}, err
			}
		} else if cylinder {
			var err error
			triangles, err = compactAxialTriangles(outer)
			if err != nil {
				return mesh.Mesh{}, err
			}
		} else {
			var err error
			triangles, err = triangulateUVBounded(ctx, rings, maxCompactFaceVertices)
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
				if err != errRefine {
					return mesh.Mesh{}, err
				}
				base, _ := parametric.SurfaceMapping(f.Surface)
				_, planar := base.(geom.PlaneSurface)
				found := false
				if planar {
					for _, side := range t.edge {
						if side.edge >= 0 && counts[side.edge] > 0 {
							failures.edges[side.edge] = true
							found = true
						}
					}
				}
				if !planar || !found {
					for _, u := range f.Outer {
						failures.edges[u.Edge] = true
					}
				}
			}
			ids := t.ids
			if f.Reversed {
				ids[1], ids[2] = ids[2], ids[1]
			}
			m.Triangles = append(m.Triangles, ids)
		}
	}
	if len(failures.edges) > 0 {
		return mesh.Mesh{}, failures
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, err
	}
	return m, nil
}
