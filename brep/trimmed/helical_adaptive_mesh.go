package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

type adaptiveSideKey struct {
	face, edge int
	a, b       geom.Vec2
	start, end float64
}

func adaptiveKey(fi int, t samples, i int) adaptiveSideKey {
	if t.edge[i].edge >= 0 {
		return adaptiveSideKey{face: -1, edge: t.edge[i].edge, start: min(t.start[i], t.end[i]), end: max(t.start[i], t.end[i])}
	}
	a, b := t.uv[i], t.uv[(i+1)%3]
	if a.X > b.X || a.X == b.X && a.Y > b.Y {
		a, b = b, a
	}
	return adaptiveSideKey{face: fi, edge: -1, a: a, b: b}
}

// TessellateHelical uses conforming adaptive bisection for retained analytic
// helical charts. Shared model-edge intervals are refined by every owner.
// The existing sampled deflection/normal checks and closed-mesh validation are
// authoritative; this does not claim a continuous analytic error certificate.
// MaxSubdivisions bounds four bisection rounds per dyadic grid level, allowing
// directional refinement without constructing the full isotropic tensor grid.
func (s Solid) TessellateHelical(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return s.tessellateHelicalAllocation(ctx, o, true)
}
func (s Solid) tessellateHelicalAllocation(ctx context.Context, o parametric.TessellationOptions, preallocate bool) (mesh.Mesh, error) {
	if ctx == nil || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Angle) || o.Angle < 0 || o.Angle > math.Pi || o.MaxSubdivisions < 0 || o.MaxSubdivisions > 1024 || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	if e := ctx.Err(); e != nil {
		return mesh.Mesh{}, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 128
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	eligible := false
	for _, f := range s.faces {
		base, _ := parametric.SurfaceMapping(f.Surface)
		switch base.(type) {
		case geom.TwistedCylinderSurface, geom.HelicalProfileSurface:
			eligible = true
		}
	}
	if !eligible {
		return mesh.Mesh{}, geom.ErrDomain
	}
	return s.tessellateAnalyticAdaptiveAllocation(ctx, o, preallocate)
}

// tessellateAnalyticAdaptive is shared only by separately qualified analytic
// entry points. Callers validate options, normalize budgets and prove eligibility.
func (s Solid) tessellateAnalyticAdaptive(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	return s.tessellateAnalyticAdaptiveAllocation(ctx, o, true)
}

func (s Solid) tessellateAnalyticAdaptiveAllocation(ctx context.Context, o parametric.TessellationOptions, preallocate bool) (mesh.Mesh, error) {
	return s.refineAnalyticMeshAllocation(ctx, o, true, preallocate)
}

// refineAnalyticMesh can omit the diagonal flips and final cleanup. It always
// applies the same edge conformity, deflection, orientation and solid checks.
func (s Solid) refineAnalyticMesh(ctx context.Context, o parametric.TessellationOptions, improve bool) (mesh.Mesh, error) {
	return s.refineAnalyticMeshAllocation(ctx, o, improve, true)
}

func (s Solid) refineAnalyticMeshAllocation(ctx context.Context, o parametric.TessellationOptions, improve, preallocate bool) (mesh.Mesh, error) {
	m := mesh.Mesh{Vertices: s.Vertices()}
	initial := s.seedTriangles()
	var e error
	current := initial
	pole := map[int]bool{}
	for _, edge := range s.edges {
		if edge.Curve == nil {
			pole[edge.Start] = true
		}
	}
	midpointIDs := map[adaptiveSideKey]int{}
	maxRounds := 4 * (bits.Len(uint(o.MaxSubdivisions)) - 1)
	if maxRounds < 2 {
		return mesh.Mesh{}, parametric.ErrBudget
	}
	optimize := improve
	lastMarked, lastCount := 0, 0
	for round := 0; round <= maxRounds; round++ {
		for fi := range current {
			if optimize {
				current[fi], e = adaptiveImprove(ctx, numericalWrappedSupport(s.faces[fi].Surface), current[fi])
				if e != nil {
					return mesh.Mesh{}, e
				}
			}
			if round >= 5 {
				current[fi], e = adaptiveCoarsen(ctx, numericalWrappedSupport(s.faces[fi].Surface), current[fi], m.Vertices, o, pole)
				if e != nil {
					return mesh.Mesh{}, e
				}
			}
			if e != nil {
				return mesh.Mesh{}, e
			}
		}
		marked := map[adaptiveSideKey]bool{}
		count := 0
		for fi, triangles := range current {
			surface := numericalWrappedSupport(s.faces[fi].Surface)
			for _, t := range triangles {
				if e := ctx.Err(); e != nil {
					return mesh.Mesh{}, e
				}
				count++
				if count > o.MaxTriangles {
					return mesh.Mesh{}, parametric.ErrBudget
				}
				collapsed := t.ids[0] == t.ids[1] || t.ids[1] == t.ids[2] || t.ids[2] == t.ids[0]
				angle := o.Angle
				if collapsed {
					angle = 0
				}
				points := [3]geom.Vec3{m.Vertices[t.ids[0]], m.Vertices[t.ids[1]], m.Vertices[t.ids[2]]}
				e := check(surface, t, points, o.Deflection, angle, [3]bool{pole[t.ids[0]], pole[t.ids[1]], pole[t.ids[2]]})
				if round < 2 {
					for i := range 3 {
						marked[adaptiveKey(fi, t, i)] = true
					}
					continue
				}
				phaseRefine := false
				base, _ := parametric.SurfaceMapping(surface)
				for i := range 3 {
					a, b := t.uv[i], t.uv[(i+1)%3]
					phase := 0.
					switch x := base.(type) {
					case geom.TwistedCylinderSurface:
						_, _, tw, _ := x.Parameters()
						angular := b.X - a.X
						if x.AngularReversed() {
							angular = -angular
						}
						phase = math.Abs(angular + tw*(b.Y-a.Y))
					case geom.HelicalProfileSurface:
						phase = math.Abs(b.X - a.X)
					}
					if phase > .25 {
						phaseRefine = true
					}
				}
				if e == nil && !phaseRefine {
					continue
				}
				if e == nil {
					e = errRefine
				}
				if e != errRefine {
					return mesh.Mesh{}, e
				}
				best, score := 0, -1.
				for i := range 3 {
					a, b := t.uv[i], t.uv[(i+1)%3]
					mid := mix(a, b, .5)
					if t.edge[i].edge >= 0 {
						var err error
						mid, err = t.edge[i].point((t.start[i] + t.end[i]) / 2)
						if err != nil {
							return mesh.Mesh{}, err
						}
					}
					du, dv, derivativeErr := surface.Partials(mid.X, mid.Y)
					if derivativeErr != nil {
						return mesh.Mesh{}, derivativeErr
					}
					value := du.Scale(b.X - a.X).Add(dv.Scale(b.Y - a.Y)).Length()
					if value > score {
						best, score = i, value
					}
				}
				if round > 5 && lastMarked < 128 {
					for i := range 3 {
						marked[adaptiveKey(fi, t, i)] = true
					}
				} else {
					marked[adaptiveKey(fi, t, best)] = true
				}
			}
		}
		if len(marked) == 0 {
			// Refinement can leave a chain of thin interior triangles adjacent
			// to a periodic trim seam. Finish monotone collapses to a fixed
			// point; every replacement satisfies the same error/phase checks.
			for fi := range current {
				for pass := 0; improve && pass < 32; pass++ {
					before := len(current[fi])
					current[fi], e = adaptiveCoarsen(ctx, numericalWrappedSupport(s.faces[fi].Surface), current[fi], m.Vertices, o, pole)
					if e != nil {
						return mesh.Mesh{}, e
					}
					if len(current[fi]) == before {
						break
					}
				}
			}
			for fi, triangles := range current {
				for _, t := range triangles {
					turn, e := geom.Orient2D(t.uv[0], t.uv[1], t.uv[2])
					if e != nil || turn != 1 {
						return mesh.Mesh{}, errTrimFold
					}
					if t.ids[0] == t.ids[1] || t.ids[1] == t.ids[2] || t.ids[2] == t.ids[0] {
						continue
					}
					ids := t.ids
					if s.faces[fi].Reversed {
						ids[1], ids[2] = ids[2], ids[1]
					}
					m.Triangles = append(m.Triangles, ids)
				}
			}
			used := map[int]int{}
			compact := make([]geom.Vec3, 0, len(m.Vertices))
			for i, tri := range m.Triangles {
				for j, id := range tri {
					next, ok := used[id]
					if !ok {
						next = len(compact)
						used[id] = next
						compact = append(compact, m.Vertices[id])
					}
					m.Triangles[i][j] = next
				}
			}
			m.Vertices = compact
			if e := m.ValidateSolid(); e != nil {
				return mesh.Mesh{}, fmt.Errorf("adaptive helical: %w", e)
			}
			return m, nil
		}
		// Freeze diagonal optimization once the remaining refinement front is
		// small or stops shrinking while the mesh size stalls. Continuing flips
		// can cycle at a stricter normal tolerance. Subsequent coarsening is
		// monotone: all replacements already pass the requested checks.
		if round >= 5 && (len(marked)*20 < count || lastMarked > 0 && len(marked) >= lastMarked && count*20 <= lastCount*21) {
			optimize = false
		}
		lastMarked, lastCount = len(marked), count
		if round == maxRounds {
			return mesh.Mesh{}, parametric.ErrBudget
		}

		next := make([][]samples, len(current))
		// Splitting k distinct sides emits exactly k+1 triangles, including
		// k=0. Reserve exact output sizes instead of repeatedly growing and
		// copying the large samples records. No sampling/refinement decisions
		// change; the private switch retains a reference path for equality tests.
		if preallocate {
			planned := 0
			for fi, triangles := range current {
				count := len(triangles)
				for _, t := range triangles {
					if e := ctx.Err(); e != nil {
						return mesh.Mesh{}, e
					}
					for i := range 3 {
						if marked[adaptiveKey(fi, t, i)] {
							count++
						}
					}
				}
				if count > o.MaxTriangles-planned {
					return mesh.Mesh{}, parametric.ErrBudget
				}
				planned += count
				next[fi] = make([]samples, 0, count)
			}
		}
		total := 0
		for fi, triangles := range current {
			surface := numericalWrappedSupport(s.faces[fi].Surface)
			for _, t := range triangles {
				if e := ctx.Err(); e != nil {
					return mesh.Mesh{}, e
				}
				var split [3]bool
				var uv [3]geom.Vec2
				var ids [3]int
				var mid [3]float64
				for i := range 3 {
					key := adaptiveKey(fi, t, i)
					if !marked[key] {
						continue
					}
					split[i] = true
					uv[i] = mix(t.uv[i], t.uv[(i+1)%3], .5)
					mid[i] = (t.start[i] + t.end[i]) / 2
					side := t.edge[i]
					if side.edge >= 0 {
						uv[i], e = side.point(mid[i])
						if e != nil {
							return mesh.Mesh{}, e
						}
					}
					p, err := surface.Eval(uv[i].X, uv[i].Y)
					if err != nil || !p.IsFinite() {
						return mesh.Mesh{}, geom.ErrInvalid
					}
					evaluated := p
					id, ok := midpointIDs[key]
					if !ok {
						if side.edge >= 0 {
							if s.edges[side.edge].Curve == nil {
								id = s.edges[side.edge].Start
							} else {
								p, err = s.onEdge(side.edge, mid[i])
								if err != nil {
									return mesh.Mesh{}, err
								}
								id = len(m.Vertices)
								m.Vertices = append(m.Vertices, p)
							}
						} else {
							id = len(m.Vertices)
							m.Vertices = append(m.Vertices, p)
						}
						midpointIDs[key] = id
					}
					if !closePoint(evaluated, m.Vertices[id], s.tolerance) {
						return mesh.Mesh{}, geom.ErrUnrepresentable
					}
					ids[i] = id
				}
				old := len(next[fi])
				next[fi] = appendAdaptiveTriangle(next[fi], t, uv, ids, mid, split)
				total += len(next[fi]) - old
				if total > o.MaxTriangles {
					return mesh.Mesh{}, parametric.ErrBudget
				}
			}
		}
		current = next
	}
	return mesh.Mesh{}, parametric.ErrBudget
}
func appendAdaptiveTriangle(dst []samples, t samples, p [3]geom.Vec2, id [3]int, mid [3]float64, split [3]bool) []samples {
	q := [6]geom.Vec2{t.uv[0], t.uv[1], t.uv[2], p[0], p[1], p[2]}
	ids := [6]int{t.ids[0], t.ids[1], t.ids[2], id[0], id[1], id[2]}
	emit := func(a, b, c int) {
		u := samples{uv: [3]geom.Vec2{q[a], q[b], q[c]}, ids: [3]int{ids[a], ids[b], ids[c]}, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
		list := [3]int{a, b, c}
		for j, x := range list {
			y := list[(j+1)%3]
			for k := range 3 {
				end := (k + 1) % 3
				if x == k && y == end && !split[k] {
					u.edge[j], u.start[j], u.end[j] = t.edge[k], t.start[k], t.end[k]
				}
				if split[k] && x == k && y == 3+k {
					u.edge[j], u.start[j], u.end[j] = t.edge[k], t.start[k], mid[k]
				}
				if split[k] && x == 3+k && y == end {
					u.edge[j], u.start[j], u.end[j] = t.edge[k], mid[k], t.end[k]
				}
			}
		}
		dst = append(dst, u)
	}
	if split[0] && split[1] && split[2] {
		emit(0, 3, 5)
		emit(1, 4, 3)
		emit(2, 5, 4)
		emit(3, 4, 5)
		return dst
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

// Improve only unconstrained chart diagonals. A twisted cylinder's natural UV
// metric is sheared; triangulating in raw UV creates physical slivers whose
// normal error survives many purely longest-edge subdivisions.
func adaptiveImprove(ctx context.Context, surface geom.Surface, triangles []samples) ([]samples, error) {
	for pass := 0; pass < 4; pass++ {
		type owner struct{ tri, side int }
		owners := map[adaptiveSideKey]owner{}
		changed := make([]bool, len(triangles))
		flips := 0
		for ti, t := range triangles {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			if changed[ti] {
				continue
			}
			for si := range 3 {
				if t.edge[si].edge >= 0 {
					continue
				}
				key := adaptiveKey(0, t, si)
				other, ok := owners[key]
				if !ok {
					owners[key] = owner{ti, si}
					continue
				}
				if changed[other.tri] {
					continue
				}
				u := triangles[other.tri]
				a, b, c := t.uv[si], t.uv[(si+1)%3], t.uv[(si+2)%3]
				d := u.uv[(other.side+2)%3]
				if !adaptivePositiveFilter(c, a, d) || !adaptivePositiveFilter(c, d, b) {
					continue
				}
				turn1, e := geom.Orient2D(c, a, d)
				if e != nil || turn1 != 1 {
					continue
				}
				turn2, e := geom.Orient2D(c, d, b)
				if e != nil || turn2 != 1 {
					continue
				}
				mid := mix(mix(a, b, .5), mix(c, d, .5), .5)
				du, dv, e := surface.Partials(mid.X, mid.Y)
				if e != nil {
					return nil, e
				}
				angleAt := func(x, y, z geom.Vec2) float64 {
					v := du.Scale(y.X - x.X).Add(dv.Scale(y.Y - x.Y))
					w := du.Scale(z.X - x.X).Add(dv.Scale(z.Y - x.Y))
					return math.Atan2(v.Cross(w).Length(), v.Dot(w))
				}
				if angleAt(c, a, b)+angleAt(d, a, b) <= math.Pi+1e-12 {
					continue
				}
				type side struct {
					edge       trimSide
					start, end float64
				}
				boundary := map[[2]geom.Vec2]side{}
				ids := map[geom.Vec2]int{}
				for _, v := range []samples{t, u} {
					for i := range 3 {
						ids[v.uv[i]] = v.ids[i]
						boundary[[2]geom.Vec2{v.uv[i], v.uv[(i+1)%3]}] = side{v.edge[i], v.start[i], v.end[i]}
					}
				}
				makeTri := func(x, y, z geom.Vec2) samples {
					r := samples{uv: [3]geom.Vec2{x, y, z}, ids: [3]int{ids[x], ids[y], ids[z]}, edge: [3]trimSide{internalSide(), internalSide(), internalSide()}}
					for i := range 3 {
						if v, ok := boundary[[2]geom.Vec2{r.uv[i], r.uv[(i+1)%3]}]; ok {
							r.edge[i], r.start[i], r.end[i] = v.edge, v.start, v.end
						}
					}
					return r
				}
				triangles[ti], triangles[other.tri] = makeTri(c, a, d), makeTri(c, d, b)
				changed[ti], changed[other.tri] = true, true
				flips++
				break
			}
		}
		if flips == 0 {
			break
		}
	}
	return triangles, nil
}

// Coarsen only unconstrained interior vertices, and only when every replacement
// triangle already satisfies the requested surface checks. The link condition
// preserves a manifold chart triangulation and all boundary samples stay fixed.
func adaptiveCoarsen(ctx context.Context, surface geom.Surface, triangles []samples, vertices []geom.Vec3, o parametric.TessellationOptions, poles map[int]bool) ([]samples, error) {
	base, _ := parametric.SurfaceMapping(surface)
	cylinder, isCylinder := base.(geom.TwistedCylinderSurface)
	collapseToBoundary := isCylinder && !cylinder.AngularReversed()
	boundary := map[int]bool{}
	incident := map[int][]int{}
	uvs := map[int]geom.Vec2{}
	// Retain directed boundary ownership when an interior vertex collapses
	// onto a trim vertex: a deleted triangle may transfer its model edge
	// to a replacement triangle, but no boundary sample is moved or removed.
	type boundarySide struct {
		side       trimSide
		start, end float64
	}
	boundarySides := map[[2]int]boundarySide{}
	candidates := []int{}
	for ti, t := range triangles {
		if ti%256 == 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
		}
		for j, id := range t.ids {
			if _, ok := uvs[id]; !ok {
				candidates = append(candidates, id)
			}
			uvs[id] = t.uv[j]
			found := false
			for _, x := range incident[id] {
				found = found || x == ti
			}
			if !found {
				incident[id] = append(incident[id], ti)
			}
			if t.edge[j].edge >= 0 {
				boundarySides[[2]int{id, t.ids[(j+1)%3]}] = boundarySide{t.edge[j], t.start[j], t.end[j]}
				boundary[id] = true
				boundary[t.ids[(j+1)%3]] = true
			}
		}
	}
	removed := make([]bool, len(triangles))
	unlink := func(id, ti int) {
		list := incident[id]
		for i, x := range list {
			if x == ti {
				incident[id] = append(list[:i], list[i+1:]...)
				return
			}
		}
	}
	contains := func(t samples, id int) bool { return t.ids[0] == id || t.ids[1] == id || t.ids[2] == id }
	type replacement struct {
		index int
		tri   samples
	}
	for _, v := range candidates {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if boundary[v] || poles[v] || len(incident[v]) == 0 || len(incident[v]) > 32 {
			continue
		}
		var neighbors [64]int
		n := 0
		for _, ti := range incident[v] {
			for _, id := range triangles[ti].ids {
				if id == v {
					continue
				}
				found := false
				for _, x := range neighbors[:n] {
					found = found || x == id
				}
				if !found {
					neighbors[n] = id
					n++
				}
			}
		}
		order := neighbors[:n]
		sort.Slice(order, func(i, j int) bool {
			a, b := vertices[order[i]].Distance(vertices[v]), vertices[order[j]].Distance(vertices[v])
			if a == b {
				return order[i] < order[j]
			}
			return a < b
		})
		for _, u := range order {
			if boundary[u] && !collapseToBoundary || poles[u] || len(incident[u]) == 0 {
				continue
			}
			shared := 0
			// A periodic seam vertex can own different UV coordinates. Use the
			// coordinate belonging to this collapse fan, never a global choice.
			targetUV := uvs[u]
			compatibleUV := true
			for _, ti := range incident[v] {
				if contains(triangles[ti], u) {
					for j, id := range triangles[ti].ids {
						if id == u {
							if shared != 0 && targetUV != triangles[ti].uv[j] {
								compatibleUV = false
							}
							targetUV = triangles[ti].uv[j]
						}
					}
					shared++
				}
			}
			if shared != 2 || !compatibleUV {
				continue
			}
			common := 0
			for _, id := range neighbors[:n] {
				if id == u {
					continue
				}
				found := false
				for _, ti := range incident[u] {
					if contains(triangles[ti], id) {
						found = true
						break
					}
				}
				if found {
					common++
				}
			}
			if common != 2 {
				continue
			}
			var replacements [32]replacement
			nr := 0
			var deletes [2]int
			nd := 0
			valid := true
			for _, ti := range incident[v] {
				t := triangles[ti]
				if contains(t, u) {
					deletes[nd] = ti
					nd++
					continue
				}
				for j, id := range t.ids {
					if id == v {
						t.ids[j] = u
						t.uv[j] = targetUV
					}
				}
				for j, id := range t.ids {
					if side, ok := boundarySides[[2]int{id, t.ids[(j+1)%3]}]; ok {
						t.edge[j], t.start[j], t.end[j] = side.side, side.start, side.end
					}
				}
				if !adaptivePositiveFilter(t.uv[0], t.uv[1], t.uv[2]) {
					valid = false
					break
				}
				turn, e := geom.Orient2D(t.uv[0], t.uv[1], t.uv[2])
				if e != nil || turn != 1 {
					valid = false
					break
				}
				points := [3]geom.Vec3{vertices[t.ids[0]], vertices[t.ids[1]], vertices[t.ids[2]]}
				if !adaptivePhaseSafe(surface, t) || check(surface, t, points, o.Deflection, o.Angle, [3]bool{poles[t.ids[0]], poles[t.ids[1]], poles[t.ids[2]]}) != nil {
					valid = false
					break
				}
				replacements[nr] = replacement{ti, t}
				nr++
			}
			if !valid {
				continue
			}
			for _, ti := range deletes[:nd] {
				removed[ti] = true
				for _, id := range triangles[ti].ids {
					unlink(id, ti)
				}
			}
			for _, r := range replacements[:nr] {
				for _, id := range triangles[r.index].ids {
					unlink(id, r.index)
				}
				triangles[r.index] = r.tri
				for _, id := range r.tri.ids {
					incident[id] = append(incident[id], r.index)
				}
			}
			break
		}
	}
	out := make([]samples, 0, len(triangles))
	for i, t := range triangles {
		if i%256 == 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
		}
		if !removed[i] {
			out = append(out, t)
		}
	}
	return out, nil
}

// Optional optimization declines nearly-collinear proposals before exact
// predicates; declining a flip/collapse never changes the retained geometry.
func adaptivePositiveFilter(a, b, c geom.Vec2) bool {
	x, y := b.X-a.X, b.Y-a.Y
	u, v := c.X-a.X, c.Y-a.Y
	left, right := x*v, y*u
	det := left - right
	return finite(det) && det > 1e-14*(math.Abs(left)+math.Abs(right))
}

// Coarsening must preserve the same anti-alias phase bound as refinement.
func adaptivePhaseSafe(surface geom.Surface, t samples) bool {
	base, _ := parametric.SurfaceMapping(surface)
	for i := range 3 {
		a, b := t.uv[i], t.uv[(i+1)%3]
		phase := 0.
		switch x := base.(type) {
		case geom.TwistedCylinderSurface:
			_, _, tw, _ := x.Parameters()
			du := b.X - a.X
			if x.AngularReversed() {
				du = -du
			}
			phase = math.Abs(du + tw*(b.Y-a.Y))
		case geom.HelicalProfileSurface:
			phase = math.Abs(b.X - a.X)
		}
		if phase > .25 {
			return false
		}
	}
	return true
}
