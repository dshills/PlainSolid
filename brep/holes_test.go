package brep

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func rectangle(x0, y0, x1, y1 float64) []geom.Vec3 {
	return []geom.Vec3{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}
func TestHoledPrismAnalyticProperties(t *testing.T) {
	s, e := PrismWithHoles(rectangle(0, 0, 4, 4), [][]geom.Vec3{rectangle(1, 1, 3, 3)}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	near(t, p.Volume, 36)
	near(t, p.SurfaceArea, 96)
	near(t, p.Centroid.X, 2)
	near(t, p.Centroid.Y, 2)
	near(t, p.Centroid.Z, 1.5)
	top := s.Topology()
	if top != (Topology{Vertices: 16, Edges: 24, Faces: 10, Shells: 1, EulerCharacteristic: 0}) {
		t.Fatal(top)
	}
	if len(s.Tessellate().Triangles) != 32 {
		t.Fatal(len(s.Tessellate().Triangles))
	}
	if e = s.Tessellate().ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if len(s.Faces()[0].Holes) != 1 || len(s.Faces()[1].Holes) != 1 {
		t.Fatal("lost holes")
	}
	reversed := s.Reversed()
	if e = reversed.Validate(); e != nil {
		t.Fatal(e)
	}
	near(t, reversed.Tessellate().SignedVolume(), -36)
	reflected, e := s.Transformed(geom.Scaling(geom.Vec3{X: -2, Y: 1, Z: 1}))
	if e != nil {
		t.Fatal(e)
	}
	near(t, reflected.Tessellate().SignedVolume(), 72)
	if reflected.Topology() != top {
		t.Fatal(reflected.Topology())
	}
}
func TestMultipleHolesAndWinding(t *testing.T) {
	outer := rectangle(0, 0, 10, 8)
	holes := [][]geom.Vec3{rectangle(1, 1, 3, 3), rectangle(6, 2, 9, 6)}
	for _, offset := range []geom.Vec3{{Z: 2}, {X: 1, Y: 2, Z: -2}} {
		s, e := PrismWithHoles(outer, holes, offset)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		near(t, p.Volume, 128)
		near(t, p.Centroid.X, (80*5-4*2-12*7.5)/64+offset.X/2)
		near(t, p.Centroid.Y, (80*4-4*2-12*4)/64.0+offset.Y/2)
		near(t, p.Centroid.Z, offset.Z/2)
		if s.Topology().EulerCharacteristic != -2 {
			t.Fatal(s.Topology())
		}
	}
	for i, j := 0, len(outer)-1; i < j; i, j = i+1, j-1 {
		outer[i], outer[j] = outer[j], outer[i]
	}
	s, e := PrismWithHoles(outer, holes, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 128)
}
func TestConcaveOuterAndHole(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 6}, {X: 6, Y: 2}, {X: 4, Y: 2}, {X: 4, Y: 6}, {Y: 6}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 4}, {X: 1, Y: 4}}
	s, e := PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 48)
	if e = s.Validate(); e != nil {
		t.Fatal(e)
	}
	// Outer area is 28; L-shaped hole area is 4.
}
func TestHoledFaceDeepCopy(t *testing.T) {
	s, e := PrismWithHoles(rectangle(0, 0, 4, 4), [][]geom.Vec3{rectangle(1, 1, 3, 3)}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	copy, e := NewSolid(v, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	faces[0].Holes[0][0].Edge = 999
	accessor := copy.Faces()
	accessor[0].Holes[0][0].Edge = 999
	if e = copy.Validate(); e != nil {
		t.Fatal(e)
	}
	near(t, copy.Tessellate().SignedVolume(), 12)
}
func TestInvalidHoles(t *testing.T) {
	outer := rectangle(0, 0, 4, 4)
	cases := map[string][][]geom.Vec3{
		"outside": {rectangle(5, 1, 6, 2)}, "touch": {rectangle(0, 1, 2, 2)}, "cross": {rectangle(3, 1, 5, 2)}, "overlap": {rectangle(1, 1, 3, 3), rectangle(2, 2, 3.5, 3.5)}, "nested": {rectangle(0.5, 0.5, 3.5, 3.5), rectangle(1, 1, 2, 2)}, "empty": {nil}, "short": {{{X: 1, Y: 1}, {X: 2, Y: 1}}}, "self intersect": {{{X: 1, Y: 1}, {X: 3, Y: 3}, {X: 1, Y: 3}, {X: 3, Y: 1}}},
	}
	nonplanar := rectangle(1, 1, 3, 3)
	nonplanar[0].Z = 0.1
	cases["nonplanar"] = [][]geom.Vec3{nonplanar}
	for name, holes := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := PrismWithHoles(outer, holes, geom.Vec3{Z: 1}); e == nil {
				t.Fatal("accepted invalid holes")
			}
		})
	}
	// NewSolid requires explicit opposite winding rather than normalizing it.
	s, e := PrismWithHoles(outer, [][]geom.Vec3{rectangle(1, 1, 3, 3)}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	faces := s.Faces()
	hole := faces[0].Holes[0]
	for i, j := 0, len(hole)-1; i < j; i, j = i+1, j-1 {
		hole[i], hole[j] = hole[j], hole[i]
	}
	for i := range hole {
		hole[i].Reversed = !hole[i].Reversed
	}
	if _, e = NewSolid(s.Vertices(), s.Edges(), faces); e == nil {
		t.Fatal("accepted same winding")
	}
}
func TestHoledWorkLimitAndEmptyHoles(t *testing.T) {
	p := make([]geom.Vec3, MaxHoledFaceVertices)
	for i := range p {
		a := 2 * math.Pi * float64(i) / float64(len(p))
		p[i] = geom.Vec3{X: math.Cos(a), Y: math.Sin(a)}
	}
	if _, e := PrismWithHoles(p, [][]geom.Vec3{rectangle(-0.1, -0.1, 0.1, 0.1)}, geom.Vec3{Z: 1}); e == nil {
		t.Fatal("work limit")
	}
	s, e := PrismWithHoles(rectangle(0, 0, 2, 2), nil, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 4)
}
func TestHoledObliquePlaneAndScale(t *testing.T) {
	for _, scale := range []float64{1e-6, 1, 1e6} {
		outer := rectangle(0, 0, 4, 4)
		hole := rectangle(1, 1, 3, 3)
		for _, ring := range [][]geom.Vec3{outer, hole} {
			for i, p := range ring {
				ring[i] = geom.Vec3{X: p.X * scale, Y: p.Y * scale, Z: p.X * scale}
			}
		}
		s, e := PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2 * scale})
		if e != nil {
			t.Fatal(e)
		}
		expected := 24 * scale * scale * scale
		if math.Abs(s.Tessellate().SignedVolume()-expected) > math.Abs(expected)*1e-10 {
			t.Fatal(s.Tessellate().SignedVolume(), expected)
		}
	}
}
func FuzzRectangularHole(f *testing.F) {
	f.Add(1.0, 2.0, 3.0)
	f.Fuzz(func(t *testing.T, x, y, height float64) {
		if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(height) || x < 0.1 || x > 3.9 || y < 0.1 || y > 3.9 || height < 0.1 || height > 10 {
			return
		}
		s, e := PrismWithHoles(rectangle(0, 0, 5, 5), [][]geom.Vec3{rectangle(x, y, x+0.5, y+0.5)}, geom.Vec3{Z: height})
		if e != nil {
			t.Fatal(e)
		}
		near(t, s.Tessellate().SignedVolume(), 24.75*height)
		if s.Topology().EulerCharacteristic != 0 {
			t.Fatal(s.Topology())
		}
	})
}

func polygonAreaXY(p []geom.Vec3) float64 {
	a := 0.0
	for i, v := range p {
		w := p[(i+1)%len(p)]
		a += v.X*w.Y - v.Y*w.X
	}
	return math.Abs(a) / 2
}
func radialPolygon(n int, x, y, radius, inner, angle float64) []geom.Vec3 {
	p := make([]geom.Vec3, n)
	for i := range p {
		a := angle + 2*math.Pi*float64(i)/float64(n)
		r := radius
		if i%2 == 1 {
			r = inner
		}
		p[i] = geom.Vec3{X: x + r*math.Cos(a), Y: y + r*math.Sin(a)}
	}
	return p
}
func TestIrregularHoledTriangulations(t *testing.T) {
	for i := 0; i < 24; i++ {
		outer := radialPolygon(10+2*(i%4), 0, 0, 5, 3.5, float64(i)*0.13)
		holes := [][]geom.Vec3{radialPolygon(6+2*(i%3), -1.3, 0, 0.8, 0.5, float64(i)*0.21), radialPolygon(8, 1.3, 0, 0.8, 0.6, float64(i)*0.17)}
		s, e := PrismWithHoles(outer, holes, geom.Vec3{X: 0.2, Y: -0.3, Z: 1.25})
		if e != nil {
			t.Fatalf("case %d: %v", i, e)
		}
		expected := polygonAreaXY(outer)
		for _, hole := range holes {
			expected -= polygonAreaXY(hole)
		}
		near(t, s.Tessellate().SignedVolume(), expected*1.25)
		if e = s.Validate(); e != nil {
			t.Fatal(e)
		}
		if s.Topology().EulerCharacteristic != -2 {
			t.Fatal(s.Topology())
		}
	}
}
func BenchmarkHoledPrism(b *testing.B) {
	outer := radialPolygon(32, 0, 0, 5, 5, 0)
	hole := radialPolygon(32, 0, 0, 2, 2, 0.03)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 1}); e != nil {
			b.Fatal(e)
		}
	}
}
