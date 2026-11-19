package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/brep/parametric"

	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// triangulateUV removes exactly collinear subdivisions for the polygon
// triangulator, then restores them to the affected boundary triangles. This
// keeps every trim vertex and edge use in the conforming seed mesh.
func triangulateUV(rings [][]geom.Vec3) ([][3]geom.Vec2, error) {
	return triangulateUVBounded(context.Background(), rings, brep.MaxHoledFaceVertices)
}
func triangulateUVBounded(ctx context.Context, rings [][]geom.Vec3, maximum int) ([][3]geom.Vec2, error) {
	uv := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
	turn := func(a, b, c geom.Vec3) int { n, _ := geom.Orient2D(uv(a), uv(b), uv(c)); return n }
	clean := make([][]geom.Vec3, len(rings))
	type subdivision struct {
		a, b   geom.Vec2
		points []geom.Vec2
	}
	var splits []subdivision
	for i, r := range rings {
		// A repeated point must not disappear when both adjacent collinear
		// occurrences are removed; restoring it would create a zero-area face.
		seen := map[geom.Vec2]bool{}
		for _, point := range r {
			if !point.IsFinite() || seen[uv(point)] {
				return nil, geom.ErrInvalid
			}
			seen[uv(point)] = true
		}
		var retained []int
		for j, x := range r {
			a, b := r[(j+len(r)-1)%len(r)], r[(j+1)%len(r)]
			if turn(a, x, b) != 0 {
				retained = append(retained, j)
				continue
			}
			if x.X < min(a.X, b.X) || x.X > max(a.X, b.X) || x.Y < min(a.Y, b.Y) || x.Y > max(a.Y, b.Y) {
				return nil, fmt.Errorf("trimmed: overlapping collinear segments")
			}
		}
		if len(retained) < 3 {
			return nil, fmt.Errorf("trimmed: degenerate UV ring")
		}
		for j, a := range retained {
			b := retained[(j+1)%len(retained)]
			clean[i] = append(clean[i], r[a])
			s := subdivision{a: uv(r[a]), b: uv(r[b])}
			for k := (a + 1) % len(r); k != b; k = (k + 1) % len(r) {
				s.points = append(s.points, uv(r[k]))
			}
			if len(s.points) > 0 {
				splits = append(splits, s)
			}
		}
	}
	var out [][3]geom.Vec2
	if maximum > brep.MaxHoledFaceVertices {
		ids, e := brep.TriangulatePlanarRings(ctx, clean[0], clean[1:], brep.PlanarTriangulationOptions{MaxVertices: maximum, MaxOperations: 64000000})
		if errors.Is(e, brep.ErrTriangulationLimit) {
			return nil, parametric.ErrBudget
		}
		if e != nil {
			return nil, e
		}
		var points []geom.Vec3
		for _, ring := range clean {
			points = append(points, ring...)
		}
		for _, tri := range ids {
			a, b, c := uv(points[tri[0]]), uv(points[tri[1]]), uv(points[tri[2]])
			sign, e := geom.Orient2D(a, b, c)
			if e != nil || sign == 0 {
				return nil, geom.ErrDegenerate
			}
			if sign < 0 {
				b, c = c, b
			}
			out = append(out, [3]geom.Vec2{a, b, c})
		}
	} else {
		z := 0.
		for _, r := range clean {
			for _, p := range r {
				z = math.Max(z, math.Max(math.Abs(p.X-clean[0][0].X), math.Abs(p.Y-clean[0][0].Y)))
			}
		}
		prism, e := brep.PrismWithHoles(clean[0], clean[1:], geom.Vec3{Z: z})
		if e != nil {
			// The legacy prism path can reject nonzero represented turns.
			// Independently prove complete exact coverage; never snap a corner.
			// Prove the original rings, including every collinear subdivision,
			// so no restoration or boundary atom can escape certification.
			return triangulateExactUVFallback(ctx, rings, maximum)
		} else {
			m := prism.Tessellate()

			for _, tri := range m.Triangles {
				a, b, c := m.Vertices[tri[0]], m.Vertices[tri[1]], m.Vertices[tri[2]]
				if a.Z == 0 && b.Z == 0 && c.Z == 0 {
					out = append(out, [3]geom.Vec2{uv(a), uv(c), uv(b)})
				}
			}
		}
	}

	for _, s := range splits {
		found := false
		for i, t := range out {
			for j, a := range t {
				if a != s.a || t[(j+1)%3] != s.b {
					continue
				}
				if found {
					return nil, fmt.Errorf("trimmed: duplicate UV boundary edge")
				}
				found = true
				c := t[(j+2)%3]
				out[i] = [3]geom.Vec2{a, s.points[0], c}
				for k, p := range s.points {
					q := s.b
					if k+1 < len(s.points) {
						q = s.points[k+1]
					}
					out = append(out, [3]geom.Vec2{p, q, c})
				}
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("trimmed: missing UV boundary subdivision")
		}
	}
	return out, nil
}

// flipDegenerateTriangles removes, where it can, the triangles of a chart
// triangulation that mesh.Mesh.Validate would reject because their corners
// span no area in space. model gives the point in space of a chart vertex.
// The result covers the same chart region with the same vertices.
//
// A vertex where a third edge meets a straight edge is a corner of the faces
// along that edge, and lies on one line with its neighbors. Chart coordinates
// are rounded, so the corner can be convex in the chart, and the chart
// triangulation can then cut off the triangle of the three corners. The side
// of that triangle opposite its middle corner is interior to the face. If the
// triangle and its neighbor across that side form a convex quadrilateral in
// the chart, the other diagonal covers the same region with two triangles,
// and it is taken when both of them have area in space. Each exchange
// leaves fewer triangles without area, so the passes end.
//
// A triangle that cannot be replaced this way is returned as it was and is
// left to the mesh check. That is the case for a face without area, for two
// corners at one point, and for a corner on a boundary edge that it does
// not subdivide.
func flipDegenerateTriangles(triangles [][3]geom.Vec2, model func(geom.Vec2) geom.Vec3) [][3]geom.Vec2 {
	sound := func(t [3]geom.Vec2) bool {
		turn, err := geom.Orient2D(t[0], t[1], t[2])
		return err == nil && turn == 1 && !mesh.Degenerate(model(t[0]), model(t[1]), model(t[2]))
	}
	for changed := true; changed; {
		changed = false
	next:
		for i, t := range triangles {
			if !mesh.Degenerate(model(t[0]), model(t[1]), model(t[2])) {
				continue
			}
			// The middle corner is opposite the longest side in space.
			middle, longest := 0, -1.
			for j := range t {
				if d := model(t[(j+1)%3]).Sub(model(t[(j+2)%3])).Length(); d > longest {
					middle, longest = j, d
				}
			}
			x, b, a := t[middle], t[(middle+1)%3], t[(middle+2)%3]
			for n, u := range triangles {
				for j, p := range u {
					if p != a || u[(j+1)%3] != b {
						continue
					}
					first, second := [3]geom.Vec2{a, x, u[(j+2)%3]}, [3]geom.Vec2{x, b, u[(j+2)%3]}
					if sound(first) && sound(second) {
						triangles[i], triangles[n] = first, second
						changed = true
					}
					continue next
				}
			}
		}
	}
	return triangles
}

// seatCurvedTrims exchanges interior sides of a chart triangulation so that,
// where it can, each curved trim stays inside the triangle on its chord.
//
// Refinement puts the midpoint of a boundary side on its curve and the
// midpoint of an interior side on its chord. Next to a corner the triangle
// that is cut off keeps the direction of the interior side that leaves the
// corner, so it folds as soon as the curve is on the far side of that side:
// a trim that leaves its corner outside the triangle folds at some level
// whatever happens elsewhere. The curve is therefore sampled at the
// parameters refinement uses next to each end, 2^-k and 1-2^-k, and a sample
// that is not strictly inside the other two sides of its triangle counts
// against that triangle. An interior side of such a triangle is exchanged
// for the other diagonal of its two triangles when both new triangles are
// positively wound and have area in space and they have fewer such samples
// between them, so the passes end. Where no exchange helps, the triangles
// stay as they were and refinement decides.
func seatCurvedTrims(triangles [][3]geom.Vec2, sides map[[2]geom.Vec2]trimSide, model func(geom.Vec2) geom.Vec3) ([][3]geom.Vec2, error) {
	const depth = 10
	samples := map[[2]geom.Vec2][]geom.Vec2{}
	for key, side := range sides {
		if side.use.Curve == nil {
			continue
		}
		for k := 1; k <= depth; k++ {
			for _, t := range [2]float64{math.Ldexp(1, -k), 1 - math.Ldexp(1, -k)} {
				p, err := side.use.uv(t)
				if err != nil {
					return nil, err
				}
				samples[key] = append(samples[key], p)
				if k == 1 {
					break // the middle once
				}
			}
		}
	}
	if len(samples) == 0 {
		return triangles, nil
	}
	// outside counts the samples of the curved sides of t that are not
	// strictly inside its other two sides.
	outside := func(t [3]geom.Vec2) (count int) {
		for j := range t {
			for _, p := range samples[[2]geom.Vec2{t[j], t[(j+1)%3]}] {
				for _, k := range [2]int{(j + 1) % 3, (j + 2) % 3} {
					if turn, err := geom.Orient2D(t[k], t[(k+1)%3], p); err != nil || turn != 1 {
						count++
						break
					}
				}
			}
		}
		return count
	}
	sound := func(t [3]geom.Vec2) bool {
		turn, err := geom.Orient2D(t[0], t[1], t[2])
		return err == nil && turn == 1 && !mesh.Degenerate(model(t[0]), model(t[1]), model(t[2]))
	}
	for changed := true; changed; {
		changed = false
		for i, t := range triangles {
			count := outside(t)
			if count == 0 {
				continue
			}
		sides:
			for j := range t {
				a, b, opposite := t[j], t[(j+1)%3], t[(j+2)%3]
				if _, trim := sides[[2]geom.Vec2{a, b}]; trim {
					continue
				}
				for n, u := range triangles {
					for k, p := range u {
						if p != b || u[(k+1)%3] != a {
							continue
						}
						first, second := [3]geom.Vec2{opposite, a, u[(k+2)%3]}, [3]geom.Vec2{opposite, u[(k+2)%3], b}
						if !sound(first) || !sound(second) {
							continue sides
						}
						if outside(first)+outside(second) < count+outside(u) {
							triangles[i], triangles[n] = first, second
							changed = true
							break sides
						}
						continue sides
					}
				}
			}
		}
	}
	return triangles, nil
}
