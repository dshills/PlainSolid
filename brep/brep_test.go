package brep

import (
	"math"
	"plainsolid/geom"
	"sync"
	"testing"
)

func near(t *testing.T, a, b float64) {
	t.Helper()
	if math.Abs(a-b) > 1e-11*math.Max(1, math.Abs(b)) {
		t.Fatalf("got %.17g want %.17g", a, b)
	}
}
func cube(t *testing.T) Solid {
	t.Helper()
	s, e := Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestBoxTopology(t *testing.T) {
	s := cube(t)
	top := s.Topology()
	if top != (Topology{Vertices: 8, Edges: 12, Faces: 6, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal(top)
	}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	m := s.Tessellate()
	if len(m.Triangles) != 12 {
		t.Fatal(len(m.Triangles))
	}
	if e := m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	props, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, props.Volume, 24)
	near(t, props.SurfaceArea, 52)
	for i := range s.Faces() {
		plane, e := s.FacePlane(i)
		if e != nil {
			t.Fatal(e)
		}
		if plane.SignedDistance(geom.Vec3{X: 1, Y: 1.5, Z: 2}) >= 0 {
			t.Fatal("normal points inward", i)
		}
	}
	bounds, e := s.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	if bounds.Max != (geom.Vec3{X: 2, Y: 3, Z: 4}) {
		t.Fatal(bounds)
	}
}
func TestPrism(t *testing.T) {
	// Start at a reflex vertex so a normal inferred from one triangle is wrong.
	p := []geom.Vec3{{X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}, {}, {X: 2}, {X: 2, Y: 1}}
	for _, offset := range []geom.Vec3{{Z: 3}, {X: 1, Y: 2, Z: -3}} {
		s, e := Prism(p, offset)
		if e != nil {
			t.Fatal(e)
		}
		top := s.Topology()
		if top.Vertices != 12 || top.Edges != 18 || top.Faces != 8 || top.EulerCharacteristic != 2 {
			t.Fatal(top)
		}
		m := s.Tessellate()
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		near(t, m.SignedVolume(), 9)
		bottom, e := s.FacePlane(0)
		if e != nil {
			t.Fatal(e)
		}
		if bottom.Normal().Dot(offset) >= 0 {
			t.Fatal("bottom normal points into prism")
		}
		topPlane, e := s.FacePlane(1)
		if e != nil {
			t.Fatal(e)
		}
		if topPlane.Normal().Dot(offset) <= 0 {
			t.Fatal("top normal points into prism")
		}
	}
}
func TestImmutable(t *testing.T) {
	original := cube(t)
	v, e, f := original.Vertices(), original.Edges(), original.Faces()
	copy, e2 := NewSolid(v, e, f)
	if e2 != nil {
		t.Fatal(e2)
	}
	v[0].X = 999
	e[0].Start = 999
	f[0].Boundary[0].Edge = 999
	if e2 = copy.Validate(); e2 != nil {
		t.Fatal(e2)
	}
	near(t, copy.Tessellate().SignedVolume(), 24)
	vv, ee, ff := copy.Vertices(), copy.Edges(), copy.Faces()
	vv[0].X = 999
	ee[0].End = 999
	ff[0].Boundary[0].Reversed = !ff[0].Boundary[0].Reversed
	m := copy.Tessellate()
	m.Vertices[0].X = 999
	m.Triangles[0][0] = 999
	if e2 = copy.Validate(); e2 != nil {
		t.Fatal(e2)
	}
	near(t, copy.Tessellate().SignedVolume(), 24)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = copy.Topology()
				_ = copy.Tessellate()
				_ = copy.Faces()
			}
		}()
	}
	wg.Wait()
}
func TestInvalidGraph(t *testing.T) {
	s := cube(t)
	cases := []struct {
		name   string
		mutate func(*[]geom.Vec3, *[]Edge, *[]Face)
	}{
		{"invalid vertex", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*e)[0].Start = -1 }},
		{"nonfinite vertex", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*v)[0].X = math.NaN() }},
		{"zero edge", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*e)[0].End = (*e)[0].Start }},
		{"coincident edge", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*v)[(*e)[0].End] = (*v)[(*e)[0].Start] }},
		{"unused vertex", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { *v = append(*v, geom.Vec3{X: 99}) }},
		{"duplicate edge", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { *e = append(*e, (*e)[0]) }},
		{"missing face", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { *f = (*f)[1:] }},
		{"edge index", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*f)[0].Boundary[0].Edge = 999 }},
		{"wire orientation", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) {
			(*f)[0].Boundary[0].Reversed = !(*f)[0].Boundary[0].Reversed
		}},
		{"short wire", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*f)[0].Boundary = (*f)[0].Boundary[:2] }},
		{"repeated wire", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) {
			(*f)[0].Boundary = append((*f)[0].Boundary, (*f)[0].Boundary...)
		}},
		{"nonplanar face", func(v *[]geom.Vec3, e *[]Edge, f *[]Face) { (*v)[0].Z = 0.1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, e, f := s.Vertices(), s.Edges(), s.Faces()
			tc.mutate(&v, &e, &f)
			if _, err := NewSolid(v, e, f); err == nil {
				t.Fatal("accepted invalid graph")
			}
		})
	}
}
func TestEmptyAndInvalidPrimitives(t *testing.T) {
	var s Solid
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	if s.Topology() != (Topology{}) {
		t.Fatal(s.Topology())
	}
	if _, e := s.Bounds(); e == nil {
		t.Fatal("empty bounds")
	}
	if _, e := s.FacePlane(0); e == nil {
		t.Fatal("bad face")
	}
	if _, e := Box(geom.Vec3{}, geom.Vec3{}); e == nil {
		t.Fatal("bad box")
	}
	if _, e := Prism(nil, geom.Vec3{Z: 1}); e == nil {
		t.Fatal("bad prism")
	}
}
func TestDisconnectedShells(t *testing.T) {
	a := cube(t)
	b, e := Box(geom.Vec3{X: 10}, geom.Vec3{X: 11, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	v, edges, faces := a.Vertices(), a.Edges(), a.Faces()
	nv, ne := len(v), len(edges)
	v = append(v, b.Vertices()...)
	for _, e := range b.Edges() {
		edges = append(edges, Edge{Start: e.Start + nv, End: e.End + nv})
	}
	for _, f := range b.Faces() {
		for i := range f.Boundary {
			f.Boundary[i].Edge += ne
		}
		faces = append(faces, f)
	}
	s, e := NewSolid(v, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	if s.Topology().Shells != 2 || s.Topology().EulerCharacteristic != 4 {
		t.Fatal(s.Topology())
	}
	near(t, s.Tessellate().SignedVolume(), 25)
}
func FuzzBox(f *testing.F) {
	f.Add(1.0, 2.0, 3.0)
	f.Fuzz(func(t *testing.T, x, y, z float64) {
		if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) || x < 0.01 || y < 0.01 || z < 0.01 || x > 100 || y > 100 || z > 100 {
			return
		}
		s, e := Box(geom.Vec3{}, geom.Vec3{X: x, Y: y, Z: z})
		if e != nil {
			t.Fatal(e)
		}
		near(t, s.Tessellate().SignedVolume(), x*y*z)
		if s.Topology().EulerCharacteristic != 2 {
			t.Fatal(s.Topology())
		}
	})
}

func TestTransforms(t *testing.T) {
	s := cube(t)
	r := s.Reversed()
	if e := r.Validate(); e != nil {
		t.Fatal(e)
	}
	near(t, r.Tessellate().SignedVolume(), -24)
	trans := geom.Translation(geom.Vec3{X: 10, Y: 20, Z: 30}).Mul(geom.Scaling(geom.Vec3{X: -2, Y: 3, Z: 4}))
	r, e := s.Transformed(trans)
	if e != nil {
		t.Fatal(e)
	}
	if r.Topology() != s.Topology() {
		t.Fatal(r.Topology())
	}
	near(t, r.Tessellate().SignedVolume(), 576)
	p, e := r.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Centroid.X, 8)
	near(t, p.Centroid.Y, 24.5)
	near(t, p.Centroid.Z, 38)
	if _, e = s.Transformed(geom.Affine{}); e == nil {
		t.Fatal("accepted singular transform")
	}
}
