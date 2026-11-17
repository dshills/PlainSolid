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

type sharedSample struct {
	edge int
	t    float64
}
type interiorSample struct {
	face int
	a, b geom.Vec2
}

func (s Solid) refine(ctx context.Context, level int, deflection, angle float64) (mesh.Mesh, error) {
	if s.sampledAnnularCaps || s.sampledAnalyticCaps {
		return s.refineFrom(ctx, level, level, deflection, angle)
	}
	m, e := s.refineFrom(ctx, level, 0, deflection, angle)
	if e != errTrimFold {
		return m, e
	}
	sawRefine := false
	for start := 1; start <= level; start++ {
		m, e = s.refineFrom(ctx, level, start, deflection, angle)
		if e == parametric.ErrBudget && sawRefine {
			return mesh.Mesh{}, errRefine
		}
		if e != errTrimFold && e != errRefine {
			return m, e
		}
		sawRefine = sawRefine || e == errRefine
	}
	if sawRefine {
		return mesh.Mesh{}, errRefine
	}
	return mesh.Mesh{}, errTrimFold
}

func (s Solid) refineFrom(ctx context.Context, level, startLevel int, deflection, angle float64) (mesh.Mesh, error) {
	m := mesh.Mesh{Vertices: s.Vertices()}
	pole := map[int]bool{}
	for _, edge := range s.edges {
		if edge.Curve == nil {
			pole[edge.Start] = true
		}
	}
	boundary := map[sharedSample]int{}
	interior := map[interiorSample]int{}
	needsRefine := false
	if err := ctx.Err(); err != nil {
		return mesh.Mesh{}, err
	}
	initial := s.seedTriangles()
	if startLevel > 0 {
		var e error
		initial, e = s.sampledSeeds(ctx, startLevel, &m, boundary)
		if e != nil {
			return mesh.Mesh{}, e
		}
	}
	for fi, triangles := range initial {
		f := s.faces[fi]
		f.Surface = numericalWrappedSupport(f.Surface)
		current := triangles
		end := level
		if s.splitSeeds[fi] {
			current = make([]samples, 0, 3*len(triangles))
			for _, t := range triangles {
				p := mix(mix(t.uv[0], t.uv[1], .5), t.uv[2], 1.0/3)
				world, e := f.Surface.Eval(p.X, p.Y)
				if e != nil || !world.IsFinite() {
					return mesh.Mesh{}, geom.ErrInvalid
				}
				id := len(m.Vertices)
				m.Vertices = append(m.Vertices, world)
				for i := range 3 {
					j := (i + 1) % 3
					current = append(current, samples{
						uv:    [3]geom.Vec2{t.uv[i], t.uv[j], p},
						ids:   [3]int{t.ids[i], t.ids[j], id},
						edge:  [3]trimSide{t.edge[i], internalSide(), internalSide()},
						start: [3]float64{t.start[i]}, end: [3]float64{t.end[i]},
					})
				}
			}
			// Sampled polygons on adjacent faces can select the same
			// diagonal between shared boundary vertices. Interior-only
			// subdivision gives each face its own midpoint on that chord.
			if startLevel > 0 && startLevel == end {
				end++
			}
		}
		for depth := startLevel; depth < end; depth++ {
			next := make([]samples, 0, 4*len(current))
			for _, t := range current {
				if err := ctx.Err(); err != nil {
					return mesh.Mesh{}, err
				}
				var p [3]geom.Vec2
				var id [3]int
				var mid [3]float64
				var split [3]bool
				kept := 0
				for i, a := range t.uv {
					if startLevel > 0 && t.edge[i].edge >= 0 {
						kept++
						continue
					}
					split[i] = true
					b := t.uv[(i+1)%3]
					p[i] = mix(a, b, .5)
					if side := t.edge[i]; side.edge >= 0 {
						mid[i] = (t.start[i] + t.end[i]) / 2
						uv, err := side.point(mid[i])
						if err != nil {
							return mesh.Mesh{}, err
						}
						p[i] = uv
					}
					eval, err := f.Surface.Eval(p[i].X, p[i].Y)
					if err != nil || !eval.IsFinite() {
						return mesh.Mesh{}, fmt.Errorf("trimmed: invalid midpoint evaluation on face %d (%T at %.17g, %.17g): %v", fi, f.Surface, p[i].X, p[i].Y, err)
					}
					if e := t.edge[i].edge; e >= 0 {
						if s.edges[e].Curve == nil {
							id[i] = s.edges[e].Start
							if !closePoint(eval, m.Vertices[id[i]], s.tolerance) {
								return mesh.Mesh{}, fmt.Errorf("trimmed: pole sample mismatch")
							}
							continue
						}
						key := sharedSample{e, mid[i]}
						j, ok := boundary[key]
						if !ok {
							x, err := s.onEdge(e, mid[i])
							if err != nil || !x.IsFinite() {
								return mesh.Mesh{}, fmt.Errorf("trimmed: invalid midpoint on edge %d", e)
							}
							j = len(m.Vertices)
							m.Vertices = append(m.Vertices, x)
							boundary[key] = j
						}
						if !closePoint(eval, m.Vertices[j], s.tolerance) {
							return mesh.Mesh{}, fmt.Errorf("trimmed: face %d disagrees with edge %d", fi, e)
						}
						id[i] = j
					} else {
						if a.X > b.X || a.X == b.X && a.Y > b.Y {
							a, b = b, a
						}
						key := interiorSample{fi, a, b}
						j, ok := interior[key]
						if !ok {
							j = len(m.Vertices)
							m.Vertices = append(m.Vertices, eval)
							interior[key] = j
						}
						id[i] = j
					}
				}
				if kept > 0 {
					next = appendFixedBoundary(next, t, p, id, split)
					continue
				}
				for i := range 3 {
					before := (i + 2) % 3
					next = append(next, samples{
						uv:    [3]geom.Vec2{t.uv[i], p[i], p[before]},
						ids:   [3]int{t.ids[i], id[i], id[before]},
						edge:  [3]trimSide{t.edge[i], internalSide(), t.edge[before]},
						start: [3]float64{t.start[i], 0, mid[before]},
						end:   [3]float64{mid[i], 0, t.end[before]},
					})
				}
				next = append(next, samples{uv: p, ids: id, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}})
			}
			current = next
		}
		for _, t := range current {
			if err := ctx.Err(); err != nil {
				return mesh.Mesh{}, err
			}
			collapsed := t.ids[0] == t.ids[1] || t.ids[1] == t.ids[2] || t.ids[0] == t.ids[2]
			if deflection > 0 {
				p := [3]geom.Vec3{m.Vertices[t.ids[0]], m.Vertices[t.ids[1]], m.Vertices[t.ids[2]]}
				a := angle
				if collapsed {
					a = 0
				}
				if err := check(f.Surface, t, p, deflection, a, [3]bool{pole[t.ids[0]], pole[t.ids[1]], pole[t.ids[2]]}); err != nil {
					if err != errRefine {
						return mesh.Mesh{}, err
					}
					// Look at the other triangles, too; any fold must
					// trigger retriangulation before refining this mesh.
					needsRefine = true
				}
			}
			turn, err := geom.Orient2D(t.uv[0], t.uv[1], t.uv[2])
			if err != nil || turn != 1 {
				return mesh.Mesh{}, errTrimFold
			}
			if collapsed {
				continue
			}
			tri := t.ids
			if f.Reversed {
				tri[1], tri[2] = tri[2], tri[1]
			}
			m.Triangles = append(m.Triangles, tri)
		}
	}
	if needsRefine {
		return mesh.Mesh{}, errRefine
	}
	if err := m.ValidateSolid(); err != nil {
		return mesh.Mesh{}, fmt.Errorf("trimmed: %w", err)
	}
	return m, nil
}

// Refine only interior sides of a face triangulated from sampled trims.
// Keeping the boundary chords here avoids reintroducing the original fold.
// Each interior side is split by both incident triangles.
func appendFixedBoundary(dst []samples, t samples, p [3]geom.Vec2, id [3]int, split [3]bool) []samples {
	q := [6]geom.Vec2{t.uv[0], t.uv[1], t.uv[2], p[0], p[1], p[2]}
	ids := [6]int{t.ids[0], t.ids[1], t.ids[2], id[0], id[1], id[2]}
	emit := func(a, b, c int) {
		u := samples{uv: [3]geom.Vec2{q[a], q[b], q[c]}, ids: [3]int{ids[a], ids[b], ids[c]},
			edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
		list := [3]int{a, b, c}
		for j, k := range list {
			if k < 3 && list[(j+1)%3] == (k+1)%3 && !split[k] {
				u.edge[j], u.start[j], u.end[j] = t.edge[k], t.start[k], t.end[k]
			}
		}
		dst = append(dst, u)
	}
	for a := range 3 {
		b, c := (a+1)%3, (a+2)%3
		if !split[a] {
			continue
		}
		if split[b] {
			emit(3+a, b, 3+b)
			emit(a, 3+a, c)
			emit(3+a, 3+b, c)
			return dst
		}
		if !split[c] {
			emit(a, 3+a, c)
			emit(3+a, b, c)
			return dst
		}
	}
	return append(dst, t)
}

// At a higher-resolution fallback rebuild affected face triangulations from
// their sampled curves. Splitting only a coarse chord's adjacent triangle
// can fold when a valid curve bulges outside that triangle.
func (s Solid) sampledSeeds(ctx context.Context, level int, m *mesh.Mesh, boundary map[sharedSample]int) ([][]samples, error) {
	n := 1 << level
	var rim []bool
	if s.sampledAnnularCaps || s.sampledAnalyticCaps {
		rim = s.sampledAnnularEdges()
	}
	out := make([][]samples, len(s.faces))
	for fi, f := range s.faces {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		wall, wrapped := f.Surface.(*geom.WrappedSurfacePatch)
		wrapped = wrapped && rim != nil
		var wallOutline [][]geom.Vec2
		start := -1
		loops := append([][]Use{f.Outer}, f.Holes...)
		rings := make([][]geom.Vec3, len(loops))
		indices := map[geom.Vec2]int{}
		type sampledSide struct {
			trimSide
			start, end float64
		}
		sides := map[[2]geom.Vec2]sampledSide{}
		for ri, loop := range loops {
			if rim == nil && len(indices)+n*len(loop) > brep.MaxHoledFaceVertices {
				return nil, parametric.ErrBudget
			}
			for _, u := range loop {
				splits := n
				if rim != nil && !rim[u.Edge] {
					splits = 1
				}
				if rim != nil && !wrapped && len(indices)+splits > brep.MaxHoledFaceVertices {
					return nil, parametric.ErrBudget
				}
				edge := s.edges[u.Edge]
				p := u.UVStart
				var outline []geom.Vec2
				if wrapped {
					outline = []geom.Vec2{p}
					d := wall.Domain()
					if p == (geom.Vec2{X: d.UMin, Y: d.VMin}) && u.UVEnd.Y == d.VMin {
						start = len(wallOutline)
					}
				}
				for j := range splits {
					t, end := float64(j)/float64(splits), float64(j+1)/float64(splits)
					q, e := u.uv(end)
					if e != nil {
						return nil, e
					}
					if j == splits-1 {
						q = u.UVEnd
					}
					if u.Reversed {
						t, end = 1-t, 1-end
					}
					id := edge.Start
					if t == 1 {
						id = edge.End
					} else if t != 0 && edge.Curve != nil {
						key := sharedSample{u.Edge, t}
						var exists bool
						id, exists = boundary[key]
						if !exists {
							p3, e := s.onEdge(u.Edge, t)
							if e != nil || !p3.IsFinite() {
								return nil, fmt.Errorf("trimmed: invalid model-edge sample")
							}
							id = len(m.Vertices)
							m.Vertices = append(m.Vertices, p3)
							boundary[key] = id
						}
					}
					p3, e := f.Surface.Eval(p.X, p.Y)
					if e != nil || !closePoint(p3, m.Vertices[id], s.tolerance) {
						return nil, fmt.Errorf("trimmed: face %d disagrees with sampled edge %d", fi, u.Edge)
					}
					if _, exists := indices[p]; exists {
						return nil, fmt.Errorf("trimmed: repeated UV vertex in sampled face %d", fi)
					}
					indices[p] = id
					sides[[2]geom.Vec2{p, q}] = sampledSide{trimSide{u.Edge, u}, t, end}
					rings[ri] = append(rings[ri], geom.Vec3{X: p.X, Y: p.Y})
					if wrapped {
						outline = append(outline, q)
					}
					p = q
				}
				if wrapped {
					wallOutline = append(wallOutline, outline)
				}
			}
		}
		var triangles [][3]geom.Vec2
		var structured bool
		var e error
		if wrapped {
			if start < 0 {
				return nil, geom.ErrConvergence
			}
			wallOutline = append(wallOutline[start:], wallOutline[:start]...)
			if _, blend := circularBoreBlendEnd(wall, f); blend {
				surface := numericalWrappedSupport(f.Surface)
				triangles, e = sampledBoreFilletGrid(wall.Domain(), wallOutline, func(p geom.Vec2) error {
					if err := ctx.Err(); err != nil {
						return err
					}
					q, err := surface.Eval(p.X, p.Y)
					if err != nil || !q.IsFinite() {
						return geom.ErrInvalid
					}
					indices[p] = len(m.Vertices)
					m.Vertices = append(m.Vertices, q)
					return nil
				})
			} else {
				triangles, e = compactAxialTriangles(wallOutline)
			}
			structured = true
		} else {
			triangles, structured, e = sampledCylinderGrid(ctx, f, n, rings, indices, m)
		}
		if e != nil {
			return nil, e
		}
		if !structured {
			triangles, e = triangulateUV(rings)
		}
		if e != nil {
			return nil, fmt.Errorf("trimmed: face %d sampled domain: %w", fi, e)
		}
		for _, tri := range triangles {
			t := samples{uv: tri, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
			for j, p := range tri {
				t.ids[j] = indices[p]
				if side, ok := sides[[2]geom.Vec2{p, tri[(j+1)%3]}]; ok {
					t.edge[j], t.start[j], t.end[j] = side.trimSide, side.start, side.end
				}
			}
			out[fi] = append(out[fi], t)
		}
	}
	return out, nil
}

// The two four-edge toroid rims and the two uses of its shared curved seam
// already have model samples. Treat each rim as one grid side and keep those
// samples as the perimeter of the existing tensor grid.
func sampledBoreFilletGrid(d geom.ParameterRectangle, outline [][]geom.Vec2, vertex func(geom.Vec2) error) ([][3]geom.Vec2, error) {
	var sides [4][]geom.Vec2
	for _, edge := range outline {
		p, q := edge[0], edge[len(edge)-1]
		side := 3
		switch {
		case p.Y == q.Y && p.Y == d.VMin:
			side = 0
		case p.X == q.X && p.X == d.UMax:
			side = 1
		case p.Y == q.Y && p.Y == d.VMax:
			side = 2
		}
		if len(sides[side]) > 0 {
			sides[side] = sides[side][:len(sides[side])-1]
		}
		sides[side] = append(sides[side], edge...)
	}
	for _, side := range sides {
		if len(side) < 2 {
			return nil, geom.ErrConvergence
		}
	}
	return compactTorusGrid(sides[:], vertex)
}

func check(s geom.Surface, tri samples, p [3]geom.Vec3, deflection, angle float64, poles ...[3]bool) error {
	uv := tri.uv
	type normalSample struct {
		uv       geom.Vec2
		boundary bool
	}
	var normalSamples [7]normalSample
	for i := range 3 {
		knownPole := len(poles) > 0 && poles[0][i]
		normalSamples[i] = normalSample{uv[i], knownPole || tri.edge[i].edge >= 0 || tri.edge[(i+2)%3].edge >= 0}
	}
	for i := range 3 {
		j := (i + 1) % 3
		xy := mix(uv[i], uv[j], .5)
		if e := tri.edge[i]; e.edge >= 0 {
			var err error
			xy, err = e.point((tri.start[i] + tri.end[i]) / 2)
			if err != nil {
				return err
			}
		}
		normalSamples[i+3] = normalSample{xy, tri.edge[i].edge >= 0}
		q, e := s.Eval(xy.X, xy.Y)
		if e != nil || !q.IsFinite() {
			return fmt.Errorf("trimmed: invalid surface sample")
		}
		if !closePoint(q, p[i].Scale(.5).Add(p[j].Scale(.5)), deflection) {
			return errRefine
		}
	}
	a, b := mix(mix(uv[0], uv[1], .5), uv[2], 1.0/3), p[0].Scale(1.0/3).Add(p[1].Scale(1.0/3)).Add(p[2].Scale(1.0/3))
	q, e := s.Eval(a.X, a.Y)
	if e != nil || !q.IsFinite() {
		return fmt.Errorf("trimmed: invalid surface sample")
	}
	if !closePoint(q, b, deflection) {
		return errRefine
	}
	if angle > 0 {
		m, e := normal(p[1].Sub(p[0]), p[2].Sub(p[0]))
		if e != nil {
			return errRefine
		}
		normalSamples[6] = normalSample{a, false}
		// Centroid-only normals can accept a coarse curved facet whose ends
		// exceed the requested angle. Check the same boundary/interior samples
		// used for geometry, plus the vertices. At a singular boundary pole no
		// unique normal exists; regular samples still constrain the facet.
		for _, sample := range normalSamples {
			du, dv, e := s.Partials(sample.uv.X, sample.uv.Y)
			if e != nil {
				return e
			}
			if !du.IsFinite() || !dv.IsFinite() {
				return fmt.Errorf("trimmed: invalid surface partials")
			}
			n, e := normal(du, dv)
			if e != nil {
				if sample.boundary {
					continue
				}
				return fmt.Errorf("trimmed: singular interior normal")
			}
			if math.Atan2(n.Cross(m).Length(), n.Dot(m)) > angle {
				return errRefine
			}
		}
	}
	return nil
}
func normal(a, b geom.Vec3) (geom.Vec3, error) {
	u, e := a.Unit()
	if e != nil {
		return geom.Vec3{}, e
	}
	v, e := b.Unit()
	if e != nil {
		return geom.Vec3{}, e
	}
	return u.Cross(v).Unit()
}

// A rectangular cylinder wall has a regular UV parameterization. Retriangulating
// its collinear sampled perimeter as a polygon creates thin fans whose physical
// normals can remain poor under interior-only refinement. Use a uniform grid,
// preserving the shared perimeter samples and edge parameters assembled above.
func sampledCylinderGrid(ctx context.Context, f Face, n int, rings [][]geom.Vec3, indices map[geom.Vec2]int, m *mesh.Mesh) ([][3]geom.Vec2, bool, error) {
	if _, ok := f.Surface.(geom.CylinderSurface); !ok || len(f.Holes) != 0 || len(f.Outer) != 4 {
		return nil, false, nil
	}
	for _, use := range f.Outer {
		if use.Curve != nil {
			return nil, false, nil
		}
	}
	a, b, c, d := f.Outer[0].UVStart, f.Outer[1].UVStart, f.Outer[2].UVStart, f.Outer[3].UVStart
	if a.Y != b.Y || b.X != c.X || c.Y != d.Y || d.X != a.X || len(rings) != 1 || len(rings[0]) != 4*n {
		return nil, false, nil
	}
	points := make([]geom.Vec2, (n+1)*(n+1))
	for j := 0; j <= n; j++ {
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
		for i := 0; i <= n; i++ {
			boundary := -1
			switch {
			case j == 0:
				boundary = i
			case i == n:
				boundary = n + j
			case j == n:
				boundary = 3*n - i
			case i == 0:
				boundary = (4*n - j) % (4 * n)
			}
			var uv geom.Vec2
			if boundary >= 0 {
				p := rings[0][boundary%(4*n)]
				uv = geom.Vec2{X: p.X, Y: p.Y}
			} else {
				u, v := float64(i)/float64(n), float64(j)/float64(n)
				uv = mix(mix(a, b, u), mix(d, c, u), v)
				p, err := f.Surface.Eval(uv.X, uv.Y)
				if err != nil || !p.IsFinite() {
					return nil, true, fmt.Errorf("trimmed: invalid cylinder grid sample")
				}
				if _, exists := indices[uv]; exists {
					return nil, true, fmt.Errorf("trimmed: unresolved cylinder grid")
				}
				indices[uv] = len(m.Vertices)
				m.Vertices = append(m.Vertices, p)
			}
			points[j*(n+1)+i] = uv
		}
	}
	var triangles [][3]geom.Vec2
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			a, b, c, d := points[j*(n+1)+i], points[j*(n+1)+i+1], points[(j+1)*(n+1)+i+1], points[(j+1)*(n+1)+i]
			triangles = append(triangles, [3]geom.Vec2{a, b, c}, [3]geom.Vec2{a, c, d})
		}
	}
	return triangles, true, nil
}
