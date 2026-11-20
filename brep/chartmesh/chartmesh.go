// Package chartmesh meshes a solid bounded by planes and circular
// cylinders, face by face in the parameters of each surface. It asks of a
// trim only that it evaluates, so it takes a face trimmed along the curve
// in which two cylinders of unequal radius or with skew axes meet
// (specs/general-boolean-quartic, phase 3), which the even refinement of
// brep/trimmed does not finish.
//
// Every edge is cut once, into equal steps of its parameter, and both
// faces of the edge take those points, so the mesh is closed whatever
// each face does inside its boundary. A planar face is triangulated on
// its boundary points alone. A cylindrical face is triangulated in the
// chart that unrolls it, and a triangle that spans too wide an angle
// about the axis, or whose normal leaves the surface normal by more than
// the facet angle, has an inner side halved until neither holds.
package chartmesh

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ErrUnsupported reports a solid outside what this package meshes: a
// surface that is neither a plane nor a circular cylinder, or an edge with
// no curve.
var ErrUnsupported = errors.New("chartmesh: unsupported solid")

// maxSteps bounds the steps one edge is cut into.
const maxSteps = 8192

// refinements bounds how often the triangles of one cylindrical face are
// looked over for sides to halve.
const refinements = 24

// face is one face in its chart. Chart coordinates are lengths: on a
// cylinder the first is the arc round the axis.
type face struct {
	surface geom.Surface
	radius  float64 // of a cylinder; zero for a plane
	limit   float64 // the widest chart span round the axis of one facet
}

// Tessellate meshes s. Deflection bounds the distance of a facet from its
// surface and of a chord from its edge. Angle, when positive, bounds the
// angle between a facet of a cylinder and the surface normal at its
// corners, as far as halving inner sides can reach it. MaxTriangles of
// zero selects a million.
func Tessellate(ctx context.Context, s trimmed.Solid, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if ctx == nil || !(o.Deflection > 0) || math.IsInf(o.Deflection, 0) || !(o.Angle >= 0) || o.Angle > math.Pi || o.MaxTriangles < 0 {
		return mesh.Mesh{}, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	faces, edges := s.Faces(), s.Edges()
	charts := make([]face, len(faces))
	for i, f := range faces {
		c, err := chartOf(f.Surface, o)
		if err != nil {
			return mesh.Mesh{}, fmt.Errorf("%w: face %d", err, i)
		}
		charts[i] = c
	}
	// The steps of each edge: enough for the chord to stay within the
	// deflection of the curve and for every use on a cylinder to stay
	// within the span of that face.
	type use struct {
		face int
		u    trimmed.Use
	}
	users := make([][]use, len(edges))
	for i, f := range faces {
		for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				users[u.Edge] = append(users[u.Edge], use{i, u})
			}
		}
	}
	steps := make([]int, len(edges))
	first := make([]int, len(edges)) // the vertex of the first inner point
	out := mesh.Mesh{Vertices: s.Vertices()}
	for i, e := range edges {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		if e.Curve == nil {
			return mesh.Mesh{}, fmt.Errorf("%w: edge %d has no curve", ErrUnsupported, i)
		}
		at := func(t float64) (geom.Vec3, error) { return e.Curve.Eval(e.Range.Min + (e.Range.Max-e.Range.Min)*t) }
		n := 1
		for _, u := range users[i] {
			if c := charts[u.face]; c.radius > 0 {
				// The whole way the use goes round the axis, forwards
				// and back.
				const parts = 16
				way, before := 0., 0.
				for k := 0; k <= parts; k++ {
					q, err := along(u.u, float64(k)/parts)
					if err != nil {
						return mesh.Mesh{}, err
					}
					if k > 0 {
						way += math.Abs(q.X - before)
					}
					before = q.X
				}
				n = max(n, int(math.Ceil(way*c.radius/c.limit)))
			}
		}
		for ; ; n += max(1, n/4) {
			if n > maxSteps {
				return mesh.Mesh{}, parametric.ErrBudget
			}
			fine := true
			from, err := at(0)
			if err != nil {
				return mesh.Mesh{}, err
			}
			for k := 1; k <= n && fine; k++ {
				middle, err := at((float64(k) - 0.5) / float64(n))
				if err != nil {
					return mesh.Mesh{}, err
				}
				to, err := at(float64(k) / float64(n))
				if err != nil {
					return mesh.Mesh{}, err
				}
				fine = middle.Distance(from.Add(to).Scale(0.5)) <= o.Deflection
				for _, u := range users[i] {
					c := charts[u.face]
					if c.radius == 0 || !fine {
						continue
					}
					p, err := along(u.u, float64(k-1)/float64(n))
					if err != nil {
						return mesh.Mesh{}, err
					}
					q, err := along(u.u, float64(k)/float64(n))
					if err != nil {
						return mesh.Mesh{}, err
					}
					fine = math.Abs(q.X-p.X)*c.radius <= c.limit*(1+1e-9)
				}
				from = to
			}
			if fine {
				break
			}
		}
		steps[i], first[i] = n, len(out.Vertices)
		for k := 1; k < n; k++ {
			p, err := at(float64(k) / float64(n))
			if err != nil {
				return mesh.Mesh{}, err
			}
			out.Vertices = append(out.Vertices, p)
		}
	}
	point := func(e, k int) int {
		switch k {
		case 0:
			return edges[e].Start
		case steps[e]:
			return edges[e].End
		}
		return first[e] + k - 1
	}
	for i, f := range faces {
		if err := ctx.Err(); err != nil {
			return mesh.Mesh{}, err
		}
		c := charts[i]
		stretch := 1.
		if c.radius > 0 {
			stretch = c.radius
		}
		var m patch
		for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			var ring []int
			for _, u := range loop {
				n := steps[u.Edge]
				for k := 0; k < n; k++ {
					q, err := along(u, float64(k)/float64(n))
					if err != nil {
						return mesh.Mesh{}, err
					}
					id := point(u.Edge, k)
					if u.Reversed {
						id = point(u.Edge, n-k)
					}
					ring = append(ring, len(m.at))
					m.at, m.ids = append(m.at, geom.Vec2{X: q.X * stretch, Y: q.Y}), append(m.ids, id)
				}
			}
			m.rings = append(m.rings, ring)
		}
		if err := m.triangulate(); err != nil {
			return mesh.Mesh{}, fmt.Errorf("chartmesh: face %d: %w", i, err)
		}
		if c.radius > 0 {
			if err := m.refine(c, o, &out); err != nil {
				return mesh.Mesh{}, fmt.Errorf("chartmesh: face %d: %w", i, err)
			}
		}
		if len(out.Triangles)+len(m.triangles) > o.MaxTriangles {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		for _, t := range m.triangles {
			tri := [3]int{m.ids[t[0]], m.ids[t[1]], m.ids[t[2]]}
			if f.Reversed {
				tri[1], tri[2] = tri[2], tri[1]
			}
			out.Triangles = append(out.Triangles, tri)
		}
	}
	if err := out.ValidateSolid(); err != nil {
		return mesh.Mesh{}, fmt.Errorf("chartmesh: %w", err)
	}
	return out, nil
}

// chartOf reads the chart of a face from its surface, which may be
// placed by a rigid motion or a mirror.
func chartOf(s geom.Surface, o parametric.TessellationOptions) (face, error) {
	base, m := parametric.SurfaceMapping(s)
	switch b := base.(type) {
	case geom.PlaneSurface:
		if _, polar := parametric.PolarPlaneFrame(base); polar {
			return face{}, ErrUnsupported
		}
		return face{surface: s}, nil
	case geom.CylinderSurface:
		_, r := b.Parameters()
		r *= math.Cbrt(math.Abs(m.Determinant()))
		// A facet that spans the angle a about the axis leaves the
		// surface by r·(1 − cos(a/2)), and its normal leaves the surface
		// normal at its corners by a/2. The span is kept a tenth inside
		// the second bound, so that only a facet of poor shape fails it.
		span := math.Pi / 2
		if o.Deflection < r {
			span = math.Min(span, 2*math.Acos(1-o.Deflection/r))
		}
		if o.Angle > 0 {
			span = math.Min(span, 1.8*o.Angle)
		}
		return face{surface: s, radius: r, limit: span * r}, nil
	}
	return face{}, ErrUnsupported
}

// along gives the surface parameters of a use at a part t of its way.
func along(u trimmed.Use, t float64) (geom.Vec2, error) {
	if u.Curve == nil {
		return geom.Vec2{X: u.UVStart.X + (u.UVEnd.X-u.UVStart.X)*t, Y: u.UVStart.Y + (u.UVEnd.Y-u.UVStart.Y)*t}, nil
	}
	return u.Curve.EvalUV(t)
}
