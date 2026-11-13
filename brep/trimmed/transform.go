package trimmed

import (
	"context"
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FromPlanar promotes planar shared topology, including holes, into the
// trimmed representation. Curve geometry and p-curves are constructed from
// the indexed vertices; the caller controls the world-space agreement tolerance.
func FromPlanar(s brep.Solid, tolerance float64) (Solid, error) {
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	edges := make([]parametric.Edge, len(e))
	for i, x := range e {
		a, b := v[x.Start], v[x.End]
		l, err := geom.NewLine(a, b.Sub(a))
		if err != nil {
			return Solid{}, err
		}
		edges[i] = parametric.Edge{Start: x.Start, End: x.End, Curve: l, Range: parametric.Interval{Max: b.Sub(a).Length()}}
	}
	faces := make([]Face, len(f))
	for i, x := range f {
		p, err := s.FacePlane(i)
		if err != nil {
			return Solid{}, err
		}
		frame, err := geom.FrameFromNormal(p.Origin(), p.Normal())
		if err != nil {
			return Solid{}, err
		}
		surf, err := geom.NewPlaneSurface(frame)
		if err != nil {
			return Solid{}, err
		}
		convert := func(loop []brep.EdgeUse) []Use {
			uses := make([]Use, len(loop))
			for i, u := range loop {
				a, b := e[u.Edge].Start, e[u.Edge].End
				if u.Reversed {
					a, b = b, a
				}
				x, y := frame.ToLocal(v[a]), frame.ToLocal(v[b])
				uses[i] = Use{Edge: u.Edge, Reversed: u.Reversed, UVStart: geom.Vec2{X: x.X, Y: x.Y}, UVEnd: geom.Vec2{X: y.X, Y: y.Y}}
			}
			return uses
		}
		faces[i] = Face{Surface: surf, Outer: convert(x.Boundary)}
		for _, h := range x.Holes {
			faces[i].Holes = append(faces[i].Holes, convert(h))
		}
	}
	return NewSolid(v, edges, faces, tolerance)
}

// Transformed maps the original curves and surfaces, retaining topology and
// parameter-space trims. A reflection reverses all face orientations.
func (s Solid) Transformed(m geom.Affine) (Solid, error) {
	if _, e := m.Inverse(); e != nil {
		return Solid{}, e
	}
	d := m.Determinant()
	if !finite(d) || d == 0 {
		return Solid{}, fmt.Errorf("trimmed: determinant outside float64 range")
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	for i, p := range v {
		v[i] = m.Apply(p)
	}
	for i, x := range e {
		if c, ok := x.Curve.(surfaceTrimCurve); ok {
			e[i].Curve = c.transformed(m)
		} else {
			e[i].Curve = parametric.MapCurve(x.Curve, m)
		}
	}
	for i, x := range f {
		f[i].Surface = parametric.MapSurface(x.Surface, m)
		if d < 0 {
			f[i].Reversed = !f[i].Reversed
		}
	}
	if usesRationalEndpoints(s) {
		points, err := collectRationalVertices(context.Background(), v, e, false)
		if err != nil {
			return Solid{}, err
		}
		for _, p := range points {
			v[p.Vertex] = p.Display
		}
	}
	bound := 0.
	for _, r := range m {
		bound = math.Hypot(bound, math.Hypot(math.Hypot(r[0], r[1]), r[2]))
	}
	return NewSolid(v, e, f, s.tolerance*bound)
}

// FromParametric promotes rectangular patches into the general trim graph,
// preserving analytic surfaces, model-space curves, seam sharing and poles.
func FromParametric(s parametric.Solid) (Solid, error) {
	var faces []Face
	for _, f := range s.Faces() {
		p := [4]geom.Vec2{{X: f.U.Min, Y: f.V.Min}, {X: f.U.Max, Y: f.V.Min}, {X: f.U.Max, Y: f.V.Max}, {X: f.U.Min, Y: f.V.Max}}
		loop := make([]Use, 4)
		for i, u := range f.Boundary {
			loop[i] = Use{Edge: u.Edge, Reversed: u.Reversed, UVStart: p[i], UVEnd: p[(i+1)%4]}
		}
		faces = append(faces, Face{Surface: f.Surface, Outer: loop, Reversed: f.Reversed})
	}
	return NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
}
