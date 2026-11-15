package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/brep/parametric"

	"plainsolid/brep"
	"plainsolid/geom"
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
