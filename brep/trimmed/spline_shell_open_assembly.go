package trimmed

import (
	"context"
	"fmt"
	"sort"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type openWirePiece struct {
	segment intersect.TubeSegment
	id      int
	reverse bool
}

type openWireFacePath struct {
	anchors [2]EdgeAnchor
	pieces  []openWirePiece
}

func (w openSplineWire) facePaths(operand int) (map[int][]openWireFacePath, map[int][]openWireFacePath, error) {
	paths := map[int][]openWireFacePath{}
	cycles := map[int][]openWireFacePath{}
	seen := make([]bool, len(w.branches))
	for i, branch := range w.branches {
		for end, p := range branch.ends {
			anchor, ok := p.anchorOn(operand)
			if !ok || seen[i] {
				continue
			}
			face := branch.faces[operand]
			path := openWireFacePath{}
			path.anchors[0] = anchor
			next := 2*i + end
			for {
				if next < 0 || next/2 >= len(w.branches) || seen[next/2] {
					return nil, nil, geom.ErrConvergence
				}
				b := w.branches[next/2]
				if b.faces[operand] != face {
					return nil, nil, geom.ErrConvergence
				}
				seen[next/2] = true
				reverse := next%2 == 1
				for k := range b.segments {
					j := k
					if reverse {
						j = len(b.segments) - 1 - k
					}
					path.pieces = append(path.pieces, openWirePiece{b.segments[j], b.offset + j, reverse})
				}
				last := (next % 2) ^ 1
				if anchor, ok := b.ends[last].anchorOn(operand); ok {
					path.anchors[1] = anchor
					break
				}
				next = w.joins[next/2][last]
			}
			if path.anchors[0] == path.anchors[1] || len(path.pieces) > 64 {
				return nil, nil, geom.ErrConvergence
			}
			paths[face] = append(paths[face], path)
		}
	}
	for i := range w.branches {
		if seen[i] {
			continue
		}
		face := w.branches[i].faces[operand]
		path := openWireFacePath{}
		for next := 2 * i; ; {
			if next < 0 || next/2 >= len(w.branches) || seen[next/2] {
				return nil, nil, geom.ErrConvergence
			}
			b := w.branches[next/2]
			_, startBoundary := b.ends[0].anchorOn(operand)
			_, endBoundary := b.ends[1].anchorOn(operand)
			if b.faces[operand] != face || startBoundary || endBoundary {
				return nil, nil, geom.ErrConvergence
			}
			seen[next/2] = true
			reverse := next%2 == 1
			for k := range b.segments {
				j := k
				if reverse {
					j = len(b.segments) - 1 - k
				}
				path.pieces = append(path.pieces, openWirePiece{b.segments[j], b.offset + j, reverse})
			}
			next = w.joins[next/2][(next%2)^1]
			if next == 2*i {
				break
			}
		}
		if len(path.pieces) < 3 || len(path.pieces) > 64 {
			return nil, nil, geom.ErrConvergence
		}
		cycles[face] = append(cycles[face], path)
	}
	return paths, cycles, nil
}

type splineSectionPartition struct {
	solid   Solid
	faces   []int   // fragment to original face
	edges   [][]int // original edge to parameter-ordered fragments
	section map[int]int
}

func (w openSplineWire) splitShell(ctx context.Context, s Solid, operand int, model [][2]geom.Vec3, o geom.TubeEndpointEdgeOptions) (splineSectionPartition, error) {
	paths, cycles, e := w.facePaths(operand)
	if e != nil {
		return splineSectionPartition{}, e
	}
	for i := range s.faces {
		for _, path := range paths[i] {
			if e := verifyTubeFaceCutDomain(s, i, path, operand); e != nil {
				return splineSectionPartition{}, e
			}
		}
	}
	anchors := make([][]float64, len(s.edges))
	vertices := map[EdgeAnchor]int{}
	for _, b := range w.branches {
		for _, p := range b.ends {
			x, ok := p.anchorOn(operand)
			if !ok {
				continue
			}
			if _, ok := vertices[x]; ok {
				continue
			}
			vertices[x] = -1
			anchors[x.Edge] = append(anchors[x.Edge], x.Parameter)
		}
	}
	edgeFragments := make([][]int, len(s.edges))
	for id, parameters := range anchors {
		sort.Float64s(parameters)
		current := id
		for _, p := range parameters {
			var h EdgeSplit
			s, h, e = s.SplitSplineEdgeExact(ctx, current, p, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
			if e != nil {
				return splineSectionPartition{}, e
			}
			vertices[EdgeAnchor{id, p}] = h.Vertex
			edgeFragments[id] = append(edgeFragments[id], current)
			current = h.Second
		}
		edgeFragments[id] = append(edgeFragments[id], current)
	}
	original := make([]int, len(s.faces))
	section := map[int]int{}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for i := range original {
		original[i] = i
		if len(paths[i]) == 0 {
			continue
		}
		var regions []Face
		v, edges, regions, e = s.openSplineFaceRegions(ctx, i, operand, paths[i], vertices, v, edges, section)
		if e != nil {
			return splineSectionPartition{}, fmt.Errorf("trimmed: arrange face %d: %w", i, e)
		}
		faces[i] = regions[0]
		faces = append(faces, regions[1:]...)
		for range regions[1:] {
			original = append(original, i)
		}
	}
	v, edges, faces, original, e = w.insertClosedSplineRings(ctx, operand, cycles, model, v, edges, faces, original, section)
	if e != nil {
		return splineSectionPartition{}, e
	}
	s, e = NewSolidContext(ctx, v, edges, faces, s.tolerance)
	return splineSectionPartition{s, original, edgeFragments, section}, e
}

// Each path is an embedded arc in the original rectangle. Paths on this
// chart cannot cross: within a face pair the branch proof rules out duplicate
// roots, and across pairs the embedding of the other shell restricts contact
// to its shared boundary, where the exact endpoint joins are already matched.
// Walk the original counterclockwise boundary and turn onto the section at
// each anchor. Every original use is consumed once and every section twice.
// This constructs all fragments at once, including arcs returning to the
// same original side. NewSolidContext also validates the represented loops
// and rejects cases whose vertex chords do not admit the seed mesh.
func (s Solid) openSplineFaceRegions(ctx context.Context, face, operand int, paths []openWireFacePath, vertices map[EdgeAnchor]int,
	v []geom.Vec3, edges []parametric.Edge, section map[int]int) ([]geom.Vec3, []parametric.Edge, []Face, error) {
	f := s.faces[face]
	type turn struct {
		uses []Use
		end  int
	}
	turns := make(map[int]turn, 2*len(paths))
	for _, p := range paths {
		if e := ctx.Err(); e != nil {
			return nil, nil, nil, e
		}
		var indices [2]int
		var ids [2]int
		for j, a := range p.anchors {
			id, ok := vertices[a]
			if !ok {
				return nil, nil, nil, geom.ErrInvalid
			}
			i := s.faceVertexUse(face, id)
			if i < 0 || turns[i].uses != nil {
				return nil, nil, nil, geom.ErrConvergence
			}
			ids[j], indices[j] = id, i
		}
		if ids[0] == ids[1] {
			return nil, nil, nil, geom.ErrConvergence
		}
		from, start := ids[0], f.Outer[indices[0]].UVStart
		forward, backward := make([]Use, len(p.pieces)), make([]Use, len(p.pieces))
		for i, piece := range p.pieces {
			trim := piece.segment.TrimA()
			if operand == 1 {
				trim = piece.segment.TrimB()
			}
			var c PCurve = trim
			if piece.reverse {
				c = reversePCurve(c)
			}
			a, e := c.EvalUV(0)
			if e != nil {
				return nil, nil, nil, e
			}
			b, e := c.EvalUV(1)
			if e != nil {
				return nil, nil, nil, e
			}
			if a != start || a == b {
				return nil, nil, nil, geom.ErrUnrepresentable
			}
			to := ids[1]
			if i+1 < len(p.pieces) {
				point, e := f.Surface.Eval(b.X, b.Y)
				if e != nil {
					return nil, nil, nil, e
				}
				to = len(v)
				v = append(v, point)
			} else if b != f.Outer[indices[1]].UVStart {
				return nil, nil, nil, geom.ErrUnrepresentable
			}
			id := len(edges)
			edges = append(edges, parametric.Edge{Start: from, End: to, Curve: surfaceTrimCurve{f.Surface, a, b, c}, Range: parametric.Interval{Max: 1}})
			section[id] = piece.id
			forward[i] = Use{Edge: id, UVStart: a, UVEnd: b, Curve: c}
			backward[len(p.pieces)-1-i] = Use{Edge: id, Reversed: true, UVStart: b, UVEnd: a, Curve: reversePCurve(c)}
			from, start = to, b
		}
		turns[indices[0]], turns[indices[1]] = turn{forward, indices[1]}, turn{backward, indices[0]}
	}
	used := make([]bool, len(f.Outer))
	var regions []Face
	for i := range f.Outer {
		if used[i] {
			continue
		}
		var loop []Use
		for j := i; ; {
			used[j] = true
			loop = append(loop, f.Outer[j])
			j = (j + 1) % len(f.Outer)
			if t, ok := turns[j]; ok {
				loop = append(loop, t.uses...)
				j = t.end
			}
			if j == i {
				break
			}
			if used[j] {
				return nil, nil, nil, geom.ErrConvergence
			}
		}
		r := f
		r.Outer = loop
		regions = append(regions, r)
	}
	if len(regions) != len(paths)+1 {
		return nil, nil, nil, geom.ErrConvergence
	}
	return v, edges, regions, nil
}

func (w openSplineWire) modelEndpoints() ([][2]geom.Vec3, error) {
	// Every section segment uses one canonical represented endpoint pair.
	// At crossings the explicit exact hint is used; at internal tube joins
	// the two charts share an independently proved model enclosure.
	var model [][2]geom.Vec3
	for _, branch := range w.branches {
		for i, s := range branch.segments {
			var points [2]geom.Vec3
			for end := range 2 {
				q, e := s.Evaluate(float64(end))
				if e != nil {
					return nil, e
				}
				points[end] = q.Point
				if end == 0 && i == 0 || end == 1 && i == len(branch.segments)-1 {
					points[end], e = geom.RepresentedExactPoint(branch.ends[end].point)
					if e != nil {
						return nil, e
					}
				}
			}
			if points[0] == points[1] {
				return nil, geom.ErrUnrepresentable
			}
			model = append(model, points)
		}
	}
	for _, r := range w.rings {
		for i, p := range r.ring.vertices {
			model = append(model, [2]geom.Vec3{p, r.ring.vertices[(i+1)%len(r.ring.vertices)]})
		}
	}
	return model, nil
}

func assembleOpenSplineWire(ctx context.Context, a, b Solid, ap, bp []splineEmbeddingFace, w openSplineWire,
	op parametric.BooleanOp, h SplineShellOpenHistory, o SplineShellOpenOptions) (Solid, SplineShellOpenHistory, error) {
	model, e := w.modelEndpoints()
	if e != nil {
		return Solid{}, h, e
	}
	if e := w.checkDistinctWireVertices(model, a, b); e != nil {
		return Solid{}, h, e
	}
	h.FacesA, h.FacesB = make([][]int, len(a.faces)), make([][]int, len(b.faces))
	h.EdgesA, h.EdgesB = make([][]int, len(a.edges)), make([][]int, len(b.edges))
	var parts []SewFace
	section := map[[2]int]int{}
	var retained [2]map[int][2]int
	var edgeFragments [2][][]int
	ab, bb := splineShellPatchBounds(ap), splineShellPatchBounds(bp)
	for operand, p := range []struct {
		s, other Solid
		bounds   geom.Box
		history  [][]int
	}{{a, b, bb, h.FacesA}, {b, a, ab, h.FacesB}} {
		partition, e := w.splitShell(ctx, p.s, operand, model, o.Endpoints)
		if e != nil {
			return Solid{}, h, fmt.Errorf("trimmed: split operand %d: %w", operand, e)
		}
		s := partition.solid
		edgeFragments[operand] = partition.edges
		retained[operand] = make(map[int][2]int)
		keep, e := classifyOpenShellFragments(ctx, s, p.s, p.other, p.bounds, partition.section, op, operand, o.Shells)
		if e != nil {
			return Solid{}, h, fmt.Errorf("trimmed: classify operand %d: %w", operand, e)
		}
		vertices := map[int]geom.Vec3{}
		for _, f := range s.faces {
			for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
				for _, u := range loop {
					id, ok := partition.section[u.Edge]
					if !ok {
						continue
					}
					_, rev, ok := trimTube(u.Curve)
					if !ok {
						return Solid{}, h, geom.ErrInvalid
					}
					x := s.edges[u.Edge]
					start, end := x.Start, x.End
					if rev != u.Reversed {
						start, end = end, start
					}
					for i, v := range [2]int{start, end} {
						if prev, ok := vertices[v]; ok && prev != model[id][i] {
							return Solid{}, h, geom.ErrConvergence
						}
						vertices[v] = model[id][i]
					}
				}
			}
		}
		for i, f := range s.faces {
			if !keep[i] {
				continue
			}
			if op == parametric.Difference && operand == 1 {
				f.Reversed = !f.Reversed
			}
			origin := partition.faces[i]
			p.history[origin] = append(p.history[origin], len(parts))
			part, ids := s.splineSewFaceMapped(f, vertices)
			for j, edge := range ids {
				if id, ok := partition.section[edge]; ok {
					section[[2]int{len(parts), j}] = id
				} else {
					retained[operand][edge] = [2]int{len(parts), j}
				}
			}
			parts = append(parts, part)
		}
	}
	s, sewing, sectionEdges, e := sewSplineSectionFragments(ctx, parts, section, len(model), max(a.tolerance, b.tolerance), o.Sew)
	if e != nil {
		return Solid{}, h, e
	}
	h.SectionEdges = sectionEdges
	for operand, rows := range [2][][]int{h.EdgesA, h.EdgesB} {
		for i, fragments := range edgeFragments[operand] {
			for _, id := range fragments {
				if use, ok := retained[operand][id]; ok {
					rows[i] = append(rows[i], sewing.Edges[use[0]][use[1]])
				}
			}
		}
	}
	return s, h, nil
}

func (w openSplineWire) checkDistinctWireVertices(model [][2]geom.Vec3, a, b Solid) error {
	original := map[geom.Vec3]bool{}
	for _, s := range []Solid{a, b} {
		for _, p := range s.vertices {
			original[p] = true
		}
	}
	seen := map[geom.Vec3]bool{}
	for i, branch := range w.branches {
		for j := range branch.segments {
			id := branch.offset + j
			if j > 0 && model[id-1][1] != model[id][0] {
				return geom.ErrConvergence
			}
			if j > 0 || 2*i < w.joins[i][0] {
				p := model[id][0]
				if original[p] || seen[p] {
					return geom.ErrUnrepresentable
				}
				seen[p] = true
			}
		}
		if 2*i+1 < w.joins[i][1] {
			p := model[branch.offset+len(branch.segments)-1][1]
			if original[p] || seen[p] {
				return geom.ErrUnrepresentable
			}
			seen[p] = true
		}
	}
	for _, r := range w.rings {
		for _, p := range r.ring.vertices {
			if original[p] || seen[p] {
				return geom.ErrUnrepresentable
			}
			seen[p] = true
		}
	}
	return nil
}

func classifyOpenShellFragments(ctx context.Context, s, original, other Solid, bounds geom.Box, section map[int]int,
	op parametric.BooleanOp, operand int, o SplineShellBooleanOptions) ([]bool, error) {
	// Every section edge toggles material membership. Every uncut shared
	// edge preserves it. Use this topology to propagate one certified
	// witness rather than doing a ray test on each fragment.
	inc := make([][]int, len(s.edges))
	for i, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				inc[u.Edge] = append(inc[u.Edge], i)
			}
		}
	}
	type neighbor struct {
		face  int
		cross bool
	}
	adj := make([][]neighbor, len(s.faces))
	for id, f := range inc {
		if len(f) != 2 || f[0] == f[1] {
			return nil, geom.ErrConvergence
		}
		_, cross := section[id]
		adj[f[0]] = append(adj[f[0]], neighbor{f[1], cross})
		adj[f[1]] = append(adj[f[1]], neighbor{f[0], cross})
	}
	state := make([]int8, len(s.faces))
	seed := -1
	for i, f := range s.faces {
		for _, u := range f.Outer {
			v := s.edges[u.Edge].Start
			if u.Reversed {
				v = s.edges[u.Edge].End
			}
			if v >= len(original.vertices) {
				continue
			}
			inside, e := classifySeparatedSplineWitness(ctx, other, bounds, original.vertices[v], o)
			if e != nil {
				return nil, e
			}
			state[i] = -1
			if inside {
				state[i] = 1
			}
			seed = i
			break
		}
		if seed >= 0 {
			break
		}
	}
	if seed < 0 {
		return nil, geom.ErrConvergence
	}
	queue := []int{seed}
	for k := 0; k < len(queue); k++ {
		i := queue[k]
		for _, n := range adj[i] {
			value := state[i]
			if n.cross {
				value = -value
			}
			if state[n.face] == 0 {
				state[n.face] = value
				queue = append(queue, n.face)
			} else if state[n.face] != value {
				return nil, geom.ErrConvergence
			}
		}
	}
	keep := make([]bool, len(s.faces))
	for i, x := range state {
		if x == 0 {
			return nil, geom.ErrConvergence
		}
		keep[i] = x < 0
		if op == parametric.Intersection || op == parametric.Difference && operand == 1 {
			keep[i] = x > 0
		}
	}
	return keep, nil
}

func verifyTubeFaceCutDomain(s Solid, face int, p openWireFacePath, operand int) error {
	d, e := faceUVBounds(s.faces[face])
	if e != nil {
		return e
	}
	for i, x := range p.pieces {
		trim := x.segment.TrimA()
		if operand == 1 {
			trim = x.segment.TrimB()
		}
		box, e := trim.BoundsUV()
		if e != nil {
			return e
		}
		if box.Min.X < d.UMin || box.Max.X > d.UMax || box.Min.Y < d.VMin || box.Max.Y > d.VMax {
			return geom.ErrConvergence
		}
		// Dependent coordinates are strictly interior to every certified
		// tube away from any verified boundary anchor. Roots on original
		// face boundaries may occur only at the path endpoints.
		for end := range 2 {
			q, e := trim.EvaluateBounds(float64(end))
			if e != nil {
				return e
			}
			local := end
			if x.reverse {
				local ^= 1
			}
			outerEnd := i == 0 && local == 0 || i == len(p.pieces)-1 && local == 1
			if !outerEnd && (q.Min.X <= d.UMin || q.Max.X >= d.UMax || q.Min.Y <= d.VMin || q.Max.Y >= d.VMax) {
				return fmt.Errorf("trimmed: internal wire endpoint not proved inside face %d: %w", face, geom.ErrConvergence)
			}
		}
	}
	return nil
}
