package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func frameAt(t *testing.T, origin geom.Vec3) geom.Frame {
	t.Helper()
	f, e := geom.NewFrame(origin, geom.Vec3{X: 1, Y: 2}, geom.Vec3{X: -2, Y: 1, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	return f
}

func TestPrimitives(t *testing.T) {
	center := geom.Vec3{X: 2, Y: -3, Z: 4}
	f := frameAt(t, center)
	for _, tc := range []struct {
		name         string
		make         func() (Solid, error)
		volume, area float64
		euler        int
	}{
		{"sphere", func() (Solid, error) { return Sphere(center, 2) }, 32 * math.Pi / 3, 16 * math.Pi, 2},
		{"cylinder", func() (Solid, error) { return Cylinder(f, 2, 3) }, 12 * math.Pi, 20 * math.Pi, 2},
		{"cone", func() (Solid, error) { return Cone(f, math.Atan2(2, 3), 3) }, 4 * math.Pi, 2 * math.Pi * (2 + math.Sqrt(13)), 2},
		{"torus", func() (Solid, error) { return Torus(f, 3, 1) }, 6 * math.Pi * math.Pi, 12 * math.Pi * math.Pi, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := tc.make()
			if e != nil {
				t.Fatal(e)
			}
			m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .008, Angle: .2})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			p, e := m.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(p.Volume/tc.volume-1) > .015 || math.Abs(p.SurfaceArea/tc.area-1) > .015 {
				t.Fatalf("area/volume %#v; expected %g/%g", p, tc.area, tc.volume)
			}
			edges := map[[2]int]bool{}
			for _, tri := range m.Triangles {
				for j, a := range tri {
					b := tri[(j+1)%3]
					edges[[2]int{min(a, b), max(a, b)}] = true
				}
			}
			if chi := len(m.Vertices) - len(edges) + len(m.Triangles); chi != tc.euler {
				t.Fatalf("Euler characteristic %d", chi)
			}
		})
	}
}

func TestTessellationConvergenceAndLimits(t *testing.T) {
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	coarse, e := s.Tessellate(ctx, TessellationOptions{Deflection: .25})
	if e != nil {
		t.Fatal(e)
	}
	fine, e := s.Tessellate(ctx, TessellationOptions{Deflection: .002})
	if e != nil {
		t.Fatal(e)
	}
	want := 4 * math.Pi / 3
	if len(fine.Triangles) <= len(coarse.Triangles) || math.Abs(fine.SignedVolume()-want) >= math.Abs(coarse.SignedVolume()-want) {
		t.Fatal("refinement did not improve volume")
	}
	for _, opts := range []TessellationOptions{{Deflection: 1e-10, MaxSubdivisions: 4}, {Deflection: .2, MaxTriangles: 31}} {
		if _, e = s.Tessellate(ctx, opts); !errors.Is(e, ErrBudget) {
			t.Fatalf("expected budget error, got %v", e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.Tessellate(canceled, TessellationOptions{Deflection: .1}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestInvalidTopology(t *testing.T) {
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	vs, es, fs := s.Vertices(), s.Edges(), s.Faces()
	bad := s.Faces()
	bad[0].Boundary[3].Reversed = false
	if _, e = NewSolid(vs, es, bad, s.Tolerance()); !errors.Is(e, ErrTopology) {
		t.Fatalf("accepted same orientation: %v", e)
	}
	bad = s.Faces()
	bad[0].Boundary[1].Edge = 99
	if _, e = NewSolid(vs, es, bad, s.Tolerance()); !errors.Is(e, ErrTopology) {
		t.Fatalf("accepted invalid edge reference: %v", e)
	}
	vs[0].X = 1
	if _, e = NewSolid(vs, es, fs, s.Tolerance()); !errors.Is(e, ErrTopology) {
		t.Fatalf("accepted inconsistent geometry: %v", e)
	}
	if s.Vertices()[0].X != 0 {
		t.Fatal("vertex alias")
	}
}

func TestDirectMassProperties(t *testing.T) {
	p := geom.Vec3{X: 123, Y: -456, Z: 99}
	f := frameAt(t, p)
	for _, tc := range []struct {
		name      string
		build     func() (Solid, error)
		area, vol float64
		center    geom.Vec3
	}{
		{"sphere", func() (Solid, error) { return Sphere(p, 2) }, 16 * math.Pi, 32 * math.Pi / 3, p},
		{"cylinder", func() (Solid, error) { return Cylinder(f, 2, 3) }, 20 * math.Pi, 12 * math.Pi, f.ToWorld(geom.Vec3{Z: 1.5})},
		{"cone", func() (Solid, error) { return Cone(f, math.Atan2(2, 3), 3) }, 2 * math.Pi * (2 + math.Sqrt(13)), 4 * math.Pi, f.ToWorld(geom.Vec3{Z: 2.25})},
		{"torus", func() (Solid, error) { return Torus(f, 3, 1) }, 12 * math.Pi * math.Pi, 6 * math.Pi * math.Pi, p},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := tc.build()
			if e != nil {
				t.Fatal(e)
			}
			m, e := s.MassProperties(context.Background(), IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(m.Area/tc.area-1) > 1e-10 || math.Abs(m.Volume/tc.vol-1) > 1e-10 || m.Centroid.Sub(tc.center).Length() > 1e-10 {
				t.Fatalf("%+v vs %g %g %+v", m, tc.area, tc.vol, tc.center)
			}
			if _, e = s.MassProperties(context.Background(), IntegrationOptions{MaxSubdivisions: 1}); !errors.Is(e, ErrBudget) {
				t.Fatalf("budget: %v", e)
			}
		})
	}
}

func TestRevolve(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: -5, Y: 3})
	for _, tc := range []struct {
		poly         []geom.Vec2
		area, volume float64
	}{
		{[]geom.Vec2{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 3}, {X: 0, Y: 3}}, 20 * math.Pi, 12 * math.Pi},
		{[]geom.Vec2{{X: 1, Y: 0}, {X: 1, Y: 3}, {X: 2, Y: 3}, {X: 2, Y: 0}}, 24 * math.Pi, 9 * math.Pi},
		{[]geom.Vec2{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 1, Y: 3}, {X: 0, Y: 3}}, math.Pi * (5 + 3*math.Sqrt(10)), 7 * math.Pi},
	} {
		s, e := Revolve(f, tc.poly)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Area-tc.area) > 1e-7 || math.Abs(p.Volume-tc.volume) > 1e-7 {
			t.Fatalf("%+v %v, expected area %g volume %g", p, e, tc.area, tc.volume)
		}
	}
	for _, bad := range [][]geom.Vec2{
		{{X: 1, Y: 0}, {X: 3, Y: 3}, {X: 1, Y: 3}, {X: 3, Y: 0}},
		{{X: 0, Y: 0}, {X: 2, Y: 1}, {X: 0, Y: 2}, {X: 1, Y: 1}},
		{{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: 2}, {X: 2, Y: 2}},
	} {
		if _, e := Revolve(f, bad); e == nil {
			t.Fatalf("accepted invalid profile %+v", bad)
		}
	}
}

func TestSphereBooleans(t *testing.T) {
	a, b := Ball{geom.Vec3{X: 1, Y: -2, Z: 4}, 1}, Ball{geom.Vec3{X: 1, Y: -1, Z: 4}, 1}
	for _, tc := range []struct {
		op           BooleanOp
		volume, area float64
	}{
		{Union, 9 * math.Pi / 4, 6 * math.Pi},
		{Intersection, 5 * math.Pi / 12, 2 * math.Pi},
		{Difference, 11 * math.Pi / 12, 4 * math.Pi},
	} {
		s, e := CombineSpheres(a, b, tc.op, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.volume) > 1e-8 || math.Abs(p.Area-tc.area) > 1e-8 {
			t.Fatalf("op %d: %+v %v", tc.op, p, e)
		}
		if len(s.Faces()) != 2 || len(s.Edges()) != 5 {
			t.Fatal("unexpected spherical topology")
		}
		m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatalf("op %d: bad mesh %v", tc.op, e)
		}
	}
	for _, tc := range []struct {
		a, b      Ball
		op        BooleanOp
		want      float64
		wantError bool
	}{
		{a, a, Difference, 0, false},
		{a, Ball{a.Center.Add(geom.Vec3{Z: 2}), 1}, Intersection, 0, false},
		{a, Ball{a.Center.Add(geom.Vec3{Z: 2}), 1}, Union, 0, true},
		{a, Ball{a.Center.Add(geom.Vec3{Z: 2 - 1e-11}), 1}, Intersection, 0, true},
		{a, Ball{a.Center.Add(geom.Vec3{Z: 4}), 1}, Union, 8 * math.Pi / 3, false},
		{a, Ball{a.Center.Add(geom.Vec3{Z: .125}), .5}, Difference, 7 * math.Pi / 6, false},
		{a, Ball{a.Center.Add(geom.Vec3{Z: .5}), .5}, Difference, 0, true},
	} {
		s, e := CombineSpheres(tc.a, tc.b, tc.op, 1e-10)
		if (e != nil) != tc.wantError {
			t.Fatalf("case %+v: %v", tc, e)
		}
		if e != nil {
			continue
		}
		if tc.want == 0 {
			if len(s.Faces()) != 0 {
				t.Fatal("expected empty result")
			}
			continue
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.want) > 1e-8 {
			t.Fatalf("case %+v: %+v %v", tc, p, e)
		}
	}
}

func TestClipSphere(t *testing.T) {
	center := geom.Vec3{X: -2, Y: 7, Z: 3}
	n, _ := (geom.Vec3{X: 2, Y: -1, Z: 3}).Unit()
	ball := Ball{Center: center, Radius: 2}
	for _, h := range []float64{-1.5, 0, .5, 1.5} {
		plane, e := geom.NewPlane(center.Add(n.Scale(h)), n)
		if e != nil {
			t.Fatal(e)
		}
		s, e := ClipSphere(ball, plane, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		height := h + 2
		wantV := math.Pi * height * height * (2 - height/3)
		wantA := 4*math.Pi*height + math.Pi*(4-h*h)
		if e != nil || math.Abs(p.Area-wantA) > 1e-8 || math.Abs(p.Volume-wantV) > 1e-8 {
			t.Fatalf("h %g: %+v %v, want area %g volume %g", h, p, e, wantA, wantV)
		}
		m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(e)
		}
		for _, v := range m.Vertices {
			if plane.SignedDistance(v) > 1e-10 {
				t.Fatalf("vertex above cut: %+v", v)
			}
		}
	}
}

func TestRoundedBox(t *testing.T) {
	a, b, c, r := 2., 3., 4., .75
	base := geom.Vec3{X: -3, Y: 5, Z: 10}
	s, e := RoundedBox(base, base.Add(geom.Vec3{X: a, Y: b, Z: c}), r)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Faces()) != 26 {
		t.Fatal("expected six planes, twelve cylinders and eight octants")
	}
	p, e := s.MassProperties(context.Background(), IntegrationOptions{})
	wA := 2*(a*b+a*c+b*c) + 2*math.Pi*r*(a+b+c) + 4*math.Pi*r*r
	wV := a*b*c + 2*r*(a*b+a*c+b*c) + math.Pi*r*r*(a+b+c) + 4*math.Pi*r*r*r/3
	wC := base.Add(geom.Vec3{X: a / 2, Y: b / 2, Z: c / 2})
	if e != nil || math.Abs(p.Area-wA) > 1e-8 || math.Abs(p.Volume-wV) > 1e-8 || p.Centroid.Sub(wC).Length() > 1e-10 {
		t.Fatalf("%+v %v; want %g %g %v", p, e, wA, wV, wC)
	}
	m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02, Angle: .2})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestTorusElbow(t *testing.T) {
	frame := frameAt(t, geom.Vec3{Y: 5})
	for _, a := range []float64{.3, math.Pi / 2, 3 * math.Pi / 2} {
		s, e := TorusElbow(frame, 3, 1, a)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Area-(6*math.Pi*a+2*math.Pi)) > 1e-8 || math.Abs(p.Volume-3*math.Pi*a) > 1e-8 {
			t.Fatalf("angle %g: %+v %v", a, p, e)
		}
		m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02, Angle: .25})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatalf("angle %g: %v", a, e)
		}
	}
}

func TestEllipticLoft(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 8})
	// Cross sections have area pi*(1+2v)*(4-2v). Integrating that
	// quadratic and its first moment gives the following independent values.
	s, e := EllipticLoft(f, geom.Vec2{X: 1, Y: 4}, geom.Vec2{X: 3, Y: 2}, 3)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(context.Background(), IntegrationOptions{})
	wantV := 17 * math.Pi
	wantC := f.ToWorld(geom.Vec3{Z: 3 * 3 / (17.0 / 3)})
	if e != nil || math.Abs(p.Volume-wantV) > 1e-8 || p.Centroid.Sub(wantC).Length() > 1e-9 {
		t.Fatalf("%+v %v, want volume %g centroid %v", p, e, wantV, wantC)
	}
	m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02, Angle: .25})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}

func TestCentroidalInertia(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 5, Y: -2, Z: 3})
	s, e := Cylinder(f, 2, 3)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	axis := f.ZAxis()
	v := [3]float64{axis.X, axis.Y, axis.Z}
	transverse := p.Volume * (3*4 + 9) / 12
	axial := p.Volume * 4 / 2
	for i := range 3 {
		for j := range 3 {
			want := (axial - transverse) * v[i] * v[j]
			if i == j {
				want += transverse
			}
			if math.Abs(p.Inertia[i][j]-want) > 1e-8 {
				t.Fatalf("inertia[%d][%d]=%g, want %g", i, j, p.Inertia[i][j], want)
			}
		}
	}
}

func TestAffineReflectionAndPersistence(t *testing.T) {
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Translation(geom.Vec3{X: 2, Z: -5}).Mul(geom.Scaling(geom.Vec3{X: -2, Y: 3, Z: 4}))
	s, e = s.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&b, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-32*math.Pi) > 1e-7 || p.Centroid.Sub(m.Apply(geom.Vec3{})).Length() > 1e-9 {
		t.Fatalf("%+v %v", p, e)
	}
	tess, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .025})
	if e != nil || tess.SignedVolume() <= 0 {
		t.Fatal(e)
	}
	if _, e = s.Transformed(geom.Affine{}); e == nil {
		t.Fatal("accepted singular transform")
	}
}

func TestAnalyticDeflectionBound(t *testing.T) {
	s, e := Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.DeflectionBound(32)
	if e != nil || b <= 0 {
		t.Fatal(b, e)
	}
	c, e := s.DeflectionBound(64)
	if e != nil || c >= b/3 {
		t.Fatal(c, e)
	}
	o := TessellationOptions{Deflection: b * .9, MaxSubdivisions: 32, AnalyticBound: true}
	if _, e = s.Tessellate(context.Background(), o); !errors.Is(e, ErrBudget) {
		t.Fatal(e)
	}
	o.MaxSubdivisions = 64
	m, e := s.Tessellate(context.Background(), o)
	if e != nil || len(m.Triangles) != 2*64*63 {
		t.Fatalf("wrong refinement: %d %v", len(m.Triangles), e)
	}
	// For a sphere the radial error at any chord point is exact.
	for _, tr := range m.Triangles {
		p := m.Vertices[tr[0]].Scale(.25).Add(m.Vertices[tr[1]].Scale(.375)).Add(m.Vertices[tr[2]].Scale(.375))
		if 1-p.Length() > c {
			t.Fatal("chord error exceeded analytic bound")
		}
	}
}

func TestRationalEllipticExtrusion(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 2, Z: 3})
	for _, d := range []float64{-3, 3} {
		offset := f.XAxis().Scale(1.2).Add(f.ZAxis().Scale(d))
		s, e := ExtrudeEllipse(f, 2, 1, offset)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-6*math.Pi) > 1e-8 || p.Centroid.Sub(f.Origin().Add(offset.Scale(.5))).Length() > 1e-8 {
			t.Fatalf("%+v %v", p, e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .02})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(e)
		}
	}
	if _, e := ExtrudeEllipse(f, 2, 1, f.XAxis()); e == nil {
		t.Fatal("accepted in-plane extrusion")
	}
}

func TestCurvedBounds(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: -4, Z: 5}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	torus, e := Torus(f, 3, 1)
	if e != nil {
		t.Fatal(e)
	}
	b, e := torus.Bounds()
	if e != nil || b.Min.Sub(geom.Vec3{X: -1, Y: -8, Z: 4}).Length() > 1e-10 || b.Max.Sub(geom.Vec3{X: 7, Y: 0, Z: 6}).Length() > 1e-10 {
		t.Fatalf("%+v %v", b, e)
	}
	r, e := geom.Rotation(geom.Vec3{X: 2, Y: -1, Z: 5}, .43)
	if e != nil {
		t.Fatal(e)
	}
	torus, e = torus.Transformed(r)
	if e != nil {
		t.Fatal(e)
	}
	b, e = torus.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for i := range 129 {
		for j := range 129 {
			p, e := torus.Faces()[0].point(float64(i)/128, float64(j)/128)
			if e != nil || !b.Contains(p) {
				t.Fatalf("missed %v, bounds %v", p, b)
			}
		}
	}
}

func TestBallQueries(t *testing.T) {
	b := Ball{geom.Vec3{X: 1, Y: 2, Z: 3}, 2}
	for _, tc := range []struct {
		offset geom.Vec3
		dist   float64
		loc    Location
		unique bool
	}{
		{geom.Vec3{}, -2, Inside, false},
		{geom.Vec3{X: 1}, -1, Inside, true},
		{geom.Vec3{Y: -2}, 0, Boundary, true},
		{geom.Vec3{Z: 5}, 3, Outside, true},
	} {
		p := b.Center.Add(tc.offset)
		r, e := b.Project(p)
		if e != nil || r.Unique != tc.unique || r.SignedDistance != tc.dist || r.Point.Sub(b.Center).Length() != 2 {
			t.Fatalf("%+v %v", r, e)
		}
		l, e := b.Classify(p, 1e-9)
		if e != nil || l != tc.loc {
			t.Fatal(l, e)
		}
	}
	d, e := b.DistanceTo(Ball{b.Center.Add(geom.Vec3{Y: 5}), .5})
	if e != nil || d != 2.5 {
		t.Fatal(d, e)
	}
}

func TestHollowAndSplitSphere(t *testing.T) {
	ctx := context.Background()
	s, e := HollowSphere(Ball{Radius: 2}, .25)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(ctx, IntegrationOptions{})
	w := 4 * math.Pi * (8 - math.Pow(1.75, 3)) / 3
	if e != nil || math.Abs(p.Volume-w) > 1e-9 {
		t.Fatal(p, e)
	}
	pl, e := geom.NewPlane(geom.Vec3{Z: .2}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	a, b, e := SplitSphere(Ball{Radius: 2}, pl, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	x, e := a.MassProperties(ctx, IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := b.MassProperties(ctx, IntegrationOptions{})
	if e != nil || math.Abs(x.Volume+y.Volume-32*math.Pi/3) > 1e-8 {
		t.Fatal(x, y, e)
	}
}

func TestParallelCylinderBooleans(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 2, Z: 3})
	a, b := DiskProfile{Radius: 1}, DiskProfile{Center: geom.Vec2{X: 1}, Radius: 1}
	lens := 2*math.Pi/3 - math.Sqrt(3)/2
	for _, tc := range []struct {
		op              BooleanOp
		area, perimeter float64
	}{
		{Intersection, lens, 4 * math.Pi / 3}, {Union, 2*math.Pi - lens, 8 * math.Pi / 3}, {Difference, math.Pi - lens, 2 * math.Pi},
	} {
		s, e := CombineParallelCylinders(f, a, b, 3, tc.op, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}

		p, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-3*tc.area) > 1e-8 || math.Abs(p.Area-(2*tc.area+3*tc.perimeter)) > 1e-8 {
			t.Fatalf("%+v %v", p, e)
		}
		m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .01, Angle: .25})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(e)
		}
	}
}

func TestCylinderCavitiesAndUnequalOverlap(t *testing.T) {
	f := frameAt(t, geom.Vec3{})
	ctx := context.Background()
	a := DiskProfile{Radius: 2}
	for _, tc := range []struct{ d, r float64 }{
		{.5, .75}, {0, .5}, {1.6, 1}, {1.2, 1}, {2.8, 1.25}, {1, 1.75},
	} {
		b := DiskProfile{Center: geom.Vec2{X: tc.d}, Radius: tc.r}
		s, e := CombineParallelCylinders(f, a, b, 2, Difference, 1e-10)
		if e != nil {
			t.Fatalf("%+v: %v", tc, e)
		}
		i, e := CombineParallelCylinders(f, a, b, 2, Intersection, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatalf("%+v: %v", tc, e)
		}
		q, e := i.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(p.Volume+q.Volume-8*math.Pi) > 1e-8 {
			t.Fatalf("%+v: wrong volumes %g %g %v", tc, p.Volume, q.Volume, e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .03, Angle: .4})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatalf("%+v: %v", tc, e)
		}
	}
}

func TestRoundedDistanceDoesNotProveTangency(t *testing.T) {
	a, b := Ball{Radius: 1}, Ball{Center: geom.Vec3{X: 2, Y: 1e-9}, Radius: 1}
	if b.Center.Length() != 2 {
		t.Fatal("fixture should round to 2")
	}
	if _, e := CombineSpheres(a, b, Intersection, 1e-10); e == nil {
		t.Fatal("rounded equality was treated as exact sphere tangency")
	}
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = CombineParallelCylinders(f, DiskProfile{Radius: 1}, DiskProfile{Center: geom.Vec2{X: 2, Y: 1e-9}, Radius: 1}, 1, Intersection, 1e-10); e == nil {
		t.Fatal("rounded equality was treated as exact cylinder tangency")
	}
}

func TestObliqueCylinderClip(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 2, Y: -3})
	center := f.ToWorld(geom.Vec3{Z: 3})
	n := f.XAxis().Scale(.4).Add(f.ZAxis())
	for _, sign := range []float64{1, -1} {
		p, e := geom.NewPlane(center, n.Scale(sign))
		if e != nil {
			t.Fatal(e)
		}
		s, e := ClipCylinder(f, 2, 5, p, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		h := 3.
		if sign < 0 {
			h = 2
		}
		wV := 4 * math.Pi * h
		wA := 4 * math.Pi * (1 + math.Hypot(1, .4) + h)
		m, e := s.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(m.Area-wA) > 1e-8 || math.Abs(m.Volume-wV) > 1e-8 {
			t.Fatalf("%+v %v, want %g %g", m, e, wA, wV)
		}
		tess, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .025})
		if e != nil || tess.ValidateSolid() != nil {
			t.Fatal(e)
		}
		for _, v := range tess.Vertices {
			if p.SignedDistance(v) > 1e-9 {
				t.Fatal("vertex outside retained halfspace")
			}
		}
	}
}

func TestRingQueries(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r := Ring{f, 3, 1}
	if p, e := r.Classify(geom.Vec3{}, 0); e != nil || p != Outside {
		t.Fatal(p, e)
	}
	if p, e := r.Classify(geom.Vec3{X: 3}, 0); e != nil || p != Inside {
		t.Fatal(p, e)
	}
	if p, e := r.Classify(geom.Vec3{X: 4}, 0); e != nil || p != Boundary {
		t.Fatal(p, e)
	}
	h, e := r.Ray(geom.Vec3{X: -5}, geom.Vec3{X: 2}, 9.5, 1e-9)
	if e != nil || len(h.Hits) != 4 {
		t.Fatal(h, e)
	}
	for i, w := range []float64{1, 3, 7, 9} {
		if math.Abs(h.Hits[i].Parameter-w) > 1e-9 || !h.Hits[i].Crossing {
			t.Fatal(h.Hits[i])
		}
	}
}

func TestCylinderBoundsIncludesCapRoundoff(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := CombineParallelCylinders(f, DiskProfile{Radius: 1}, DiskProfile{Radius: 1, Center: geom.Vec2{X: 1}}, .1, Union, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range s.Faces() {
		for i := range 11 {
			for j := range 11 {
				v, e := f.point(float64(i)/10, float64(j)/10)
				if e != nil || !b.Contains(v) {
					t.Fatalf("face point %+v not in %+v (%v)", v, b, e)
				}
			}
		}
	}
}

func TestThickTorusAndTube(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 3, Z: 2})
	torus, e := HollowTorus(f, 4, 1, .25)
	if e != nil {
		t.Fatal(e)
	}
	p, e := torus.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-8*math.Pi*math.Pi*(1-.75*.75)) > 1e-8 {
		t.Fatal(p, e)
	}
	tube, e := Tube(f, 3, 5, .5)
	if e != nil {
		t.Fatal(e)
	}
	p, e = tube.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-5*math.Pi*(9-6.25)) > 1e-8 {
		t.Fatal(p, e)
	}
}

func TestSphericalBore(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 3, Y: 2, Z: 1})
	ctx := context.Background()
	for _, tc := range []struct {
		op           BooleanOp
		area, volume float64
	}{
		{Difference, 24 * math.Pi, 32 * math.Pi / 3},
		{Intersection, 12 * math.Pi, 76 * math.Pi / 3},
	} {
		// R=3, bore radius sqrt(5), remaining band half-height h=2.
		r := math.Sqrt(5)
		a := tc.area + 8*math.Pi*r
		s, e := SphereCylinder(f, 3, r, tc.op, 1e-10)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-tc.volume) > 1e-8 || math.Abs(p.Area-a) > 1e-8 || p.Centroid.Sub(f.Origin()).Length() > 1e-9 {
			t.Fatalf("%+v %v, want %g %g", p, e, a, tc.volume)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .03, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(e)
		}
	}
}

func TestAnalyticSolidRay(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		s                 func() (Solid, error)
		origin, direction geom.Vec3
		max               float64
		want              []float64
	}{
		{func() (Solid, error) { return Sphere(geom.Vec3{}, 2) }, geom.Vec3{Z: -4}, geom.Vec3{Z: 1}, 10, []float64{2, 6}},
		{func() (Solid, error) { return Cylinder(f, 1, 3) }, geom.Vec3{X: .5, Z: -2}, geom.Vec3{Z: 1}, 10, []float64{2, 5}},
		{func() (Solid, error) { return Cylinder(f, 1, 3) }, geom.Vec3{X: -3, Z: 1}, geom.Vec3{X: 1}, 3, []float64{2}},
		{func() (Solid, error) { return Torus(f, 3, 1) }, geom.Vec3{X: -5}, geom.Vec3{X: 1}, 10, []float64{1, 3, 7, 9}},
	} {
		s, e := tc.s()
		if e != nil {
			t.Fatal(e)
		}
		h, e := s.Ray(tc.origin, tc.direction, tc.max, 1e-9)
		if e != nil || len(h) != len(tc.want) {
			t.Fatalf("got %+v, want %v (%v)", h, tc.want, e)
		}
		for i, w := range tc.want {
			if math.Abs(h[i].Distance-w) > 1e-9 {
				t.Fatal(h, w)
			}
		}
	}
}

func TestCylinderRegionProjection(t *testing.T) {
	f, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	c := CylinderRegion{Frame: f, Radius: 2, Height: 6}
	for _, tc := range []struct {
		p, q   geom.Vec3
		d      float64
		unique bool
	}{
		{geom.Vec3{Z: 3}, geom.Vec3{X: 2, Z: 3}, -2, false},
		{geom.Vec3{Z: .5}, geom.Vec3{}, -.5, true},
		{geom.Vec3{X: 3, Z: 7}, geom.Vec3{X: 2, Z: 6}, math.Sqrt2, true},
		{geom.Vec3{X: 2, Z: 2}, geom.Vec3{X: 2, Z: 2}, 0, true},
	} {
		q, e := c.Project(tc.p)
		if e != nil || q.Point != tc.q || q.Unique != tc.unique || math.Abs(q.SignedDistance-tc.d) > 1e-12 {
			t.Fatal(q, e)
		}
	}
}

func TestMatchedSplineLoft(t *testing.T) {
	f := frameAt(t, geom.Vec3{X: 3, Z: 5})
	hi := f.ToWorld(geom.Vec3{X: 1, Z: 3})
	t0, e := geom.NewFrame(hi, f.XAxis(), f.ZAxis())
	if e != nil {
		t.Fatal(e)
	}
	a, e := geom.NewEllipse(f, 1, 4)
	if e != nil {
		t.Fatal(e)
	}
	b, e := geom.NewEllipse(t0, 3, 2)
	if e != nil {
		t.Fatal(e)
	}
	x, e := a.RationalBSpline()
	if e != nil {
		t.Fatal(e)
	}
	y, e := b.RationalBSpline()
	if e != nil {
		t.Fatal(e)
	}
	s, e := LoftSplines(x, y, f.Origin(), hi, f.ZAxis(), 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-17*math.Pi) > 1e-8 {
		t.Fatal(p, e)
	}
	m, e := s.Tessellate(context.Background(), TessellationOptions{Deflection: .03})
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal(e)
	}
}
