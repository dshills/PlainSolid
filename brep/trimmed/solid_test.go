package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type lift struct {
	plane     geom.PlaneSurface
	amplitude float64
}

func (l lift) Eval(u, v float64) (geom.Vec3, error) {
	p, e := l.plane.Eval(u, v)
	if e != nil {
		return p, e
	}
	x, y := p.X, p.Y
	return p.Add(geom.Vec3{Z: l.amplitude * x * (1 - x) * y * (1 - y)}), nil
}
func (l lift) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	p, e := l.plane.Eval(u, v)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	a, b, e := l.plane.Partials(u, v)
	dx, dy := l.amplitude*(1-2*p.X)*p.Y*(1-p.Y), l.amplitude*p.X*(1-p.X)*(1-2*p.Y)
	a.Z += a.X*dx + a.Y*dy
	b.Z += b.X*dx + b.Y*dy
	return a, b, e
}

func fromPlanar(t *testing.T, s brep.Solid, bump float64) Solid {
	t.Helper()
	v := s.Vertices()
	edges := make([]parametric.Edge, 0)
	for _, e := range s.Edges() {
		p, d := v[e.Start], v[e.End].Sub(v[e.Start])
		line, err := geom.NewLine(p, d)
		if err != nil {
			t.Fatal(err)
		}
		edges = append(edges, parametric.Edge{Start: e.Start, End: e.End, Curve: line, Range: parametric.Interval{Max: d.Length()}})
	}
	var faces []Face
	for i, f := range s.Faces() {
		p, err := s.FacePlane(i)
		if err != nil {
			t.Fatal(err)
		}
		frame, err := geom.FrameFromNormal(p.Origin(), p.Normal())
		if err != nil {
			t.Fatal(err)
		}
		surface, err := geom.NewPlaneSurface(frame)
		if err != nil {
			t.Fatal(err)
		}
		out := Face{Surface: surface}
		if p.Normal().Z > .99 && bump != 0 {
			out.Surface = lift{surface, bump}
		}
		convert := func(loop []brep.EdgeUse) []Use {
			var r []Use
			for _, u := range loop {
				e := edges[u.Edge]
				a, b := e.Start, e.End
				if u.Reversed {
					a, b = b, a
				}
				x, y := frame.ToLocal(v[a]), frame.ToLocal(v[b])
				r = append(r, Use{Edge: u.Edge, Reversed: u.Reversed, UVStart: geom.Vec2{X: x.X, Y: x.Y}, UVEnd: geom.Vec2{X: y.X, Y: y.Y}})
			}
			return r
		}
		out.Outer = convert(f.Boundary)
		for _, h := range f.Holes {
			out.Holes = append(out.Holes, convert(h))
		}
		faces = append(faces, out)
	}
	r, err := NewSolid(v, edges, faces, 1e-10)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCurvedTrim(t *testing.T) {
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s := fromPlanar(t, box, 2)
	m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .0005, Angle: .1})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	// Integral of x(1-x)y(1-y) over the top unit square is 1/36.
	if got := m.SignedVolume(); math.Abs(got-(1+2.0/36)) > .0005 {
		t.Fatalf("volume %g", got)
	}
	p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-(1+2.0/36)) > 1e-10 {
		t.Fatal(p, e)
	}
	if _, e = s.MassProperties(context.Background(), parametric.IntegrationOptions{MaxSamples: 1}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}

	if _, e = s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 1e-12, MaxTriangles: 12}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}
	f := s.Faces()
	f[0].Outer[0].UVStart.X += .1
	if _, e = NewSolid(s.Vertices(), s.Edges(), f, 1e-10); e == nil {
		t.Fatal("accepted open UV loop")
	}
}

func TestHoledTrim(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 3}, {X: 1, Y: 3}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	r := fromPlanar(t, s, 0)
	m, e := r.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .001, Angle: .01})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	p, e := r.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-28) > 1e-9 || math.Abs(p.Area-72) > 1e-9 {
		t.Fatal(p, e)
	}

	if math.Abs(m.SignedVolume()-28) > 1e-10 {
		t.Fatalf("volume %g", m.SignedVolume())
	}
}

func TestCurvedSharedTrim(t *testing.T) {
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s := fromPlanar(t, box, 0)
	faces := s.Faces()
	vertices, edges := s.Vertices(), s.Edges()
	top := -1
	for i, f := range faces {
		a, b, e := f.Surface.Partials(0, 0)
		if e == nil && a.Cross(b).Z > .99 {
			top = i
			break
		}
	}
	if top < 0 {
		t.Fatal("top not found")
	}
	f := faces[top]
	a, c := f.Outer[0].UVStart, f.Outer[2].UVStart
	center := mix(a, c, .5)
	center.X += .06
	p0, e := f.Surface.Eval(a.X, a.Y)
	if e != nil {
		t.Fatal(e)
	}
	p1, e := f.Surface.Eval(center.X, center.Y)
	if e != nil {
		t.Fatal(e)
	}
	p2, e := f.Surface.Eval(c.X, c.Y)
	if e != nil {
		t.Fatal(e)
	}
	curve, e := geom.NewBezier([]geom.Vec3{p0, p1, p2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	idx := func(u Use) int {
		e := edges[u.Edge]
		if u.Reversed {
			return e.End
		}
		return e.Start
	}
	edgeID := len(edges)
	edges = append(edges, parametric.Edge{Start: idx(f.Outer[0]), End: idx(f.Outer[2]), Curve: curve, Range: parametric.Interval{Max: 1}})
	one, two := f, f
	one.Outer = []Use{f.Outer[0], f.Outer[1], {Edge: edgeID, Reversed: true, UVStart: c, UVEnd: a, Curve: QuadraticUV{c, center, a}}}
	two.Outer = []Use{{Edge: edgeID, UVStart: a, UVEnd: c, Curve: QuadraticUV{a, center, c}}, f.Outer[2], f.Outer[3]}
	faces[top] = one
	faces = append(faces, two)
	s, e = NewSolid(vertices, edges, faces, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadJSON(bytes.NewReader(native.Bytes()), JSONLimits{MaxEntities: 1}); e == nil {
		t.Fatal("ignored entity limit")
	}
	s, e = ReadJSON(&native, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}

	props, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(props.Volume-1) > 1e-10 || math.Abs(props.Area-6) > 1e-10 {
		t.Fatal(props, e)
	}

	m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .00025})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-1) > 1e-10 {
		t.Fatalf("bad refined shape: %v", e)
	}
	mid, e := curve.Eval(.5)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, p := range m.Vertices {
		if p.Sub(mid).Length() < 1e-12 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("shared trim midpoint appears %d times", count)
	}
}

func TestPromoteAndTransform(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.Transformed(geom.Scaling(geom.Vec3{X: -2, Y: 1, Z: .5}))
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01})
	if e != nil || math.Abs(m.SignedVolume()-24) > 1e-10 {
		t.Fatalf("%g %v", m.SignedVolume(), e)
	}
}

func FuzzTrimmedJSON(f *testing.F) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		f.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		f.Fatal(e)
	}
	f.Add(buf.Bytes())
	f.Add([]byte(`{"version":1,"kind":"trimmed-brep"}`))
	// Reach the exact rational coefficient parser with a valid support;
	// the ordinary topology path still rejects this incomplete seed.
	frame, e := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		f.Fatal(e)
	}
	plane, e := geom.NewPlaneSurface(frame)
	if e != nil {
		f.Fatal(e)
	}
	support, e := parametric.EncodeSurface(plane)
	if e != nil {
		f.Fatal(e)
	}
	seed, e := json.Marshal(nativeSolid{Version: 4, Kind: "trimmed-brep", Tolerance: 1e-5,
		Vertices: []geom.Vec3{{}}, Edges: []nativeEdge{{Surface: support, Range: parametric.Interval{Max: 1},
			P: &nativeUse{ExactQuadratic: &[3][3]string{{"0", "0", "1"}, {"1/3", "1/3", "1/3"}, {"1", "0", "1"}}}}}})
	if e != nil {
		f.Fatal(e)
	}
	f.Add(seed)
	f.Fuzz(func(t *testing.T, b []byte) {
		s, e := ReadJSON(bytes.NewReader(b), JSONLimits{MaxBytes: 16000, MaxEntities: 100, MaxControlPoints: 32})
		if e != nil {
			return
		}
		if e = WriteJSON(&bytes.Buffer{}, s); e != nil {
			t.Fatal(e)
		}
	})
}

func TestPromotedSeamsAndPoles(t *testing.T) {
	ctx := context.Background()
	f, e := geom.FrameFromNormal(geom.Vec3{X: 2}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, build := range []func() (parametric.Solid, error){
		func() (parametric.Solid, error) { return parametric.Sphere(f.Origin(), 2) },
		func() (parametric.Solid, error) { return parametric.Torus(f, 3, 1) },
		func() (parametric.Solid, error) { return parametric.Cylinder(f, 1, 2) },
	} {
		s, e := build()
		if e != nil {
			t.Fatal(e)
		}
		r, e := FromParametric(s)
		if e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, r); e != nil {
			t.Fatal(e)
		}
		r, e = ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Tessellate(ctx, parametric.TessellationOptions{Deflection: .025, Angle: .3})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(e)
		}
		p, e := r.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Area-q.Area) > 1e-8 || math.Abs(p.Volume-q.Volume) > 1e-8 {
			t.Fatalf("%+v %+v %v", p, q, e)
		}
	}
}

func TestSplitFacePreservesGeometryAndHistory(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	a, c := f.Outer[0].UVStart, f.Outer[2].UVStart
	p := mix(a, c, .5)
	p.X += .05
	cut := QuadraticUV{a, p, c}
	r, h, e := s.SplitFace(0, 0, 2, cut)
	if e != nil || h.First != 0 || h.Second != len(s.Faces()) || h.Edge != len(s.Edges()) {
		t.Fatal(h, e)
	}
	if len(s.Faces()) != 6 {
		t.Fatal("input mutated")
	}
	r, e = r.Transformed(geom.Translation(geom.Vec3{Z: 2}))
	if e != nil {
		t.Fatal(e)
	}

	var buf bytes.Buffer
	if e = WriteJSON(&buf, r); e != nil {
		t.Fatal(e)
	}
	r, e = ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	m, e := r.Tessellate(ctx, parametric.TessellationOptions{Deflection: .005})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
	v, e := r.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(v.Volume-1) > 1e-10 {
		t.Fatal(v, e)
	}
	bad := cut
	bad.Control.X += 10
	if _, _, e = s.SplitFace(0, 0, 2, bad); e == nil {
		t.Fatal("accepted exterior split")
	}
}

func TestAnalyticBoundPromotedSphere(t *testing.T) {
	s, e := parametric.Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	r, e := FromParametric(s)
	if e != nil {
		t.Fatal(e)
	}
	o := parametric.TessellationOptions{Deflection: .01, AnalyticBound: true, MaxSubdivisions: 4}
	if _, e = r.Tessellate(context.Background(), o); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}
	o.MaxSubdivisions = 128
	m, e := r.Tessellate(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range m.Triangles {
		c := m.Vertices[x[0]].Scale(.2).Add(m.Vertices[x[1]].Scale(.35)).Add(m.Vertices[x[2]].Scale(.45))
		if 1-c.Length() > .01 {
			t.Fatal("sample exceeds bound")
		}
	}
}

func TestSplitSharedEdge(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	out, h, e := s.SplitEdge(0, .375)
	if e != nil {
		t.Fatal(e)
	}
	if h.Vertex != 8 || h.First != 0 || h.Second != 12 {
		t.Fatal(h)
	}
	uses := 0
	for _, f := range out.Faces() {
		for _, u := range f.Outer {
			if u.Edge == h.First || u.Edge == h.Second {
				uses++
			}
		}
	}
	if uses != 4 {
		t.Fatal(uses)
	}
	m, e := out.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-1) > 1e-12 {
		t.Fatal(e, m.SignedVolume())
	}
}

func TestSplitPeriodicEdge(t *testing.T) {
	s, e := parametric.Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	t0, e := FromParametric(s)
	if e != nil {
		t.Fatal(e)
	}
	x, h, e := t0.SplitEdge(1, .5)
	if e != nil {
		t.Fatal(e)
	}
	if h.Vertex != 2 {
		t.Fatal(h)
	}
	m, e := x.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .02})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestSplitCurvedEdgeRoundTrip(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	a, c := f.Outer[0].UVStart, f.Outer[2].UVStart
	p := mix(a, c, .5)
	p.X += .05
	r, h, e := s.SplitFace(0, 0, 2, QuadraticUV{a, p, c})
	if e != nil {
		t.Fatal(e)
	}
	r, _, e = r.SplitEdge(h.Edge, .4)
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, r); e != nil {
		t.Fatal(e)
	}
	r, e = ReadJSON(&buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .005})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestClosedTrimFaceSplit(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	lo, hi := f.Outer[0].UVStart, f.Outer[0].UVStart
	for _, u := range f.Outer {
		p := u.UVStart
		lo.X, lo.Y = min(lo.X, p.X), min(lo.Y, p.Y)
		hi.X, hi.Y = max(hi.X, p.X), max(hi.Y, p.Y)
	}
	a, c := mix(lo, hi, .25), mix(lo, hi, .75)
	loop := []geom.Vec2{a, {X: c.X, Y: a.Y}, c, {X: a.X, Y: c.Y}}
	s, h, e := s.SplitFaceLoop(0, loop)
	if e != nil || len(h.Edges) != 4 || len(h.Vertices) != 4 || h.Inner != 6 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-1) > 1e-10 || math.Abs(p.Area-6) > 1e-10 {
		t.Fatal(p, e)
	}
	m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestClosedCurvedLoop(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	f := s.Faces()[0]
	lo, hi := f.Outer[0].UVStart, f.Outer[0].UVStart
	for _, u := range f.Outer {
		p := u.UVStart
		lo.X, lo.Y = min(lo.X, p.X), min(lo.Y, p.Y)
		hi.X, hi.Y = max(hi.X, p.X), max(hi.Y, p.Y)
	}
	c := mix(lo, hi, .5)
	r := .15
	points := []geom.Vec2{{X: c.X + r, Y: c.Y}, {X: c.X, Y: c.Y + r}, {X: c.X - r, Y: c.Y}, {X: c.X, Y: c.Y - r}}
	curves := make([]PCurve, 4)
	for i, a := range points {
		b := points[(i+1)%4]
		d := sub2(add2(a, b), c)
		q, e := geom.NewBezier([]geom.Vec3{{X: a.X, Y: a.Y}, {X: d.X, Y: d.Y}, {X: b.X, Y: b.Y}}, []float64{1, math.Sqrt(.5), 1})
		if e != nil {
			t.Fatal(e)
		}
		curves[i] = SplineUV{Curve: q, Range: UVInterval{Max: 1}}
	}
	s, h, e := s.SplitFaceCurvedLoop(0, curves)
	if e != nil || len(h.Edges) != 4 {
		t.Fatal(h, e)
	}
	p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-1) > 1e-8 || math.Abs(p.Area-6) > 1e-8 {
		t.Fatal(p, e)
	}
	m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .002})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestTrimLoopProofReporting(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromPlanar(b, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	r, e := s.ValidateTrimLoops(ctx, trimvalidate.Options{})
	if e != nil || !r.Complete || len(r.Faces) != 6 {
		t.Fatal(r, e)
	}
	s, _, e = BoreCylinder(b, 0, 1, geom.Vec3{X: 2, Y: 2}, .5, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.ValidateTrimLoops(ctx, trimvalidate.Options{})
	if e != nil || r.Complete || r.Faces[0].Status != trimvalidate.Unresolved || r.Faces[2].Status != trimvalidate.Certified {
		t.Fatal(r, e)
	}
}
