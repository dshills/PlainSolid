package trimmed

import (
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SphereCutHistory identifies the original planar face, its circular rim,
// and the generated cavity wall faces. All source graph IDs are retained.
type SphereCutHistory struct {
	Opening   int
	RimEdges  [4]int
	WallFaces [4]int
}

// CutSphereFromBox subtracts a ball from an axis-aligned box. Supported cases
// are strict disjointness, strict containment (a closed cavity), and one
// transverse circular opening through a box face. Balls that reach a second
// face, touch an edge or approach tangency within tolerance are rejected.
func CutSphereFromBox(minimum, maximum geom.Vec3, ball parametric.Ball, tolerance float64) (Solid, SphereCutHistory, error) {
	fail := func(msg string) (Solid, SphereCutHistory, error) {
		return Solid{}, SphereCutHistory{}, fmt.Errorf("trimmed: sphere cut: %s", msg)
	}
	if !ball.Center.IsFinite() || !finite(ball.Radius) || !finite(tolerance) || tolerance <= 0 || ball.Radius <= tolerance {
		return fail("invalid sphere or tolerance")
	}
	b, e := brep.Box(minimum, maximum)
	if e != nil {
		return Solid{}, SphereCutHistory{}, e
	}
	s, e := FromPlanar(b, tolerance)
	if e != nil {
		return Solid{}, SphereCutHistory{}, e
	}
	history := SphereCutHistory{Opening: -1, RimEdges: [4]int{-1, -1, -1, -1}, WallFaces: [4]int{-1, -1, -1, -1}}
	c := ball.Center
	q := geom.Vec3{X: max(minimum.X, min(maximum.X, c.X)), Y: max(minimum.Y, min(maximum.Y, c.Y)), Z: max(minimum.Z, min(maximum.Z, c.Z))}
	sep := c.Distance(q)
	if !finite(sep) {
		return fail("separation outside float64 range")
	}
	if sep > ball.Radius+tolerance {
		return s, history, nil
	}
	var d float64
	for i, f := range s.faces {
		frame := f.Surface.(geom.PlaneSurface).Frame()
		distance := c.Sub(frame.Origin()).Dot(frame.ZAxis())
		if !finite(distance) {
			return fail("nonfinite face distance")
		}
		if distance > ball.Radius+tolerance {
			return s, history, nil
		}
		if distance < -ball.Radius-tolerance {
			continue
		}
		if history.Opening >= 0 || math.Abs(distance) >= ball.Radius-tolerance {
			return fail("sphere reaches more than one face or is nearly tangent")
		}
		history.Opening, d = i, distance
	}
	if history.Opening < 0 {
		b, e := parametric.Sphere(c, ball.Radius)
		if e != nil {
			return Solid{}, SphereCutHistory{}, e
		}
		t, e := FromParametric(b)
		if e != nil {
			return Solid{}, SphereCutHistory{}, e
		}
		v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
		nv, ne := len(v), len(edges)
		v = append(v, t.vertices...)
		for _, x := range t.edges {
			x.Start += nv
			x.End += nv
			edges = append(edges, x)
		}
		for _, f := range t.Faces() {
			f.Reversed = !f.Reversed
			for i := range f.Outer {
				f.Outer[i].Edge += ne
			}
			faces = append(faces, f)
		}
		x, e := NewSolid(v, edges, faces, max(tolerance, b.Tolerance()))
		return x, history, e
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	f := faces[history.Opening].Surface.(geom.PlaneSurface).Frame()
	sf, e := geom.NewFrame(c, f.XAxis(), f.ZAxis())
	if e != nil {
		return Solid{}, SphereCutHistory{}, e
	}
	wall, e := geom.NewSphereSurface(sf, ball.Radius)
	if e != nil {
		return Solid{}, SphereCutHistory{}, e
	}
	latitude := math.Asin(-d / ball.Radius)
	radius := math.Sqrt(ball.Radius-math.Abs(d)) * math.Sqrt(ball.Radius+math.Abs(d))
	center := f.ToLocal(c)
	uvCenter := geom.Vec2{X: center.X, Y: center.Y}
	arc := func(a, b float64) EllipseUV {
		return EllipseUV{Center: uvCenter, Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: radius}, From: a, To: b}
	}
	var rim [4]int
	for i := range 4 {
		a := float64(i) * math.Pi / 2
		p, e := wall.Eval(a, latitude)
		if e != nil {
			return Solid{}, SphereCutHistory{}, e
		}
		rim[i] = len(v)
		v = append(v, p)
	}
	pole := len(v)
	v = append(v, c.Sub(sf.ZAxis().Scale(ball.Radius)))
	var seams, poles [4]int
	hole := make([]Use, 4)
	for i := range 4 {
		j := (i + 1) % 4
		a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		re := arc(a, b)
		id := len(edges)
		history.RimEdges[i] = id
		edges = append(edges, parametric.Edge{Start: rim[i], End: rim[j],
			Curve: surfaceTrimCurve{surface: faces[history.Opening].Surface, curve: re}, Range: parametric.Interval{Max: 1}})
		start, e := re.EvalUV(0)
		if e != nil {
			return Solid{}, SphereCutHistory{}, e
		}
		end, e := re.EvalUV(1)
		if e != nil {
			return Solid{}, SphereCutHistory{}, e
		}
		hole[3-i] = Use{Edge: id, Reversed: true, UVStart: end, UVEnd: start, Curve: arc(b, a)}
		seams[i] = len(edges)
		edges = append(edges, parametric.Edge{Start: pole, End: rim[i], Range: parametric.Interval{Max: 1},
			Curve: surfaceTrimCurve{surface: wall, a: geom.Vec2{X: a, Y: -math.Pi / 2}, b: geom.Vec2{X: a, Y: latitude}}})
		poles[i] = len(edges)
		edges = append(edges, parametric.Edge{Start: pole, End: pole})
	}
	faces[history.Opening].Holes = append(faces[history.Opening].Holes, hole)
	for i := range 4 {
		j := (i + 1) % 4
		a, b := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		points := [4]geom.Vec2{{X: a, Y: -math.Pi / 2}, {X: b, Y: -math.Pi / 2}, {X: b, Y: latitude}, {X: a, Y: latitude}}
		ids := [4]int{poles[i], seams[j], history.RimEdges[i], seams[i]}
		l := make([]Use, 4)
		for k, id := range ids {
			l[k] = Use{Edge: id, Reversed: k >= 2, UVStart: points[k], UVEnd: points[(k+1)%4]}
		}
		history.WallFaces[i] = len(faces)
		faces = append(faces, Face{Surface: wall, Outer: l, Reversed: true})
	}
	out, e := NewSolid(v, edges, faces, tolerance)
	if e != nil {
		return Solid{}, SphereCutHistory{}, e
	}
	return out, history, nil
}
