package mesh

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func closeValue(t *testing.T, a, b, tol float64) {
	t.Helper()
	if math.Abs(a-b) > tol {
		t.Fatalf("got %.17g want %.17g", a, b)
	}
}
func TestBox(t *testing.T) {
	m, err := Box(geom.Vec3{X: 1e9, Y: 2e9, Z: -3e9}, geom.Vec3{X: 1e9 + 2, Y: 2e9 + 3, Z: -3e9 + 4})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	closeValue(t, m.SignedVolume(), 24, 1e-12)
	closeValue(t, m.Area(), 52, 1e-12)
	closeValue(t, m.Reversed().SignedVolume(), -24, 1e-12)
}
func TestSphere(t *testing.T) {
	for _, res := range []int{4, 16, 64} {
		m, err := Sphere(geom.Vec3{}, 2, res*2, res)
		if err != nil {
			t.Fatal(err)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		exact := 4 * math.Pi * 8 / 3
		if m.SignedVolume() <= 0 || m.SignedVolume() >= exact {
			t.Fatal("invalid sphere approximation")
		}
		if res == 64 {
			closeValue(t, m.SignedVolume(), exact, 0.04)
			closeValue(t, m.Area(), 16*math.Pi, 0.06)
		}
	}
}
func TestCylinder(t *testing.T) {
	m, err := Cylinder(geom.Vec3{X: 1, Y: 2, Z: 3}, 2, 5, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	expected := 0.5 * 32 * 4 * math.Sin(2*math.Pi/32) * 5
	closeValue(t, m.SignedVolume(), expected, 1e-11)
}
func TestExtrude(t *testing.T) {
	p := []geom.Vec3{{X: 0, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 2, Y: 1, Z: 0}, {X: 1, Y: 1, Z: 0}, {X: 1, Y: 2, Z: 0}, {X: 0, Y: 2, Z: 0}}
	for _, offset := range []geom.Vec3{{X: 0, Y: 0, Z: 3}, {X: 1, Y: 2, Z: -3}} {
		m, err := Extrude(p, offset)
		if err != nil {
			t.Fatal(err)
		}
		closeValue(t, m.SignedVolume(), 9, 1e-12)
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
	// A polygon in the YZ plane exercises dominant-coordinate projection.
	m, err := Extrude([]geom.Vec3{{X: 2, Y: 0, Z: 0}, {X: 2, Y: 3, Z: 0}, {X: 2, Y: 3, Z: 4}, {X: 2, Y: 0, Z: 4}}, geom.Vec3{X: -5, Y: 0, Z: 0})
	if err != nil {
		t.Fatal(err)
	}
	closeValue(t, m.SignedVolume(), 60, 1e-12)
}
func TestInvalidExtrusion(t *testing.T) {
	cases := [][]geom.Vec3{{{X: 0, Y: 0, Z: 0}, {X: 1, Y: 1, Z: 0}, {X: 0, Y: 1, Z: 0}, {X: 1, Y: 0, Z: 0}}, {{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 1, Y: 1, Z: 1}, {X: 0, Y: 1, Z: 0}}, {{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 2, Y: 1, Z: 0}, {X: 0, Y: 1, Z: 0}}, {{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0}, {X: 0, Y: 0, Z: 0}}}
	for _, p := range cases {
		if _, err := Extrude(p, geom.Vec3{Z: 1}); err == nil {
			t.Fatal("accepted invalid polygon")
		}
	}
	if _, err := Extrude([]geom.Vec3{{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0}}, geom.Vec3{X: 1}); err == nil {
		t.Fatal("accepted parallel extrusion")
	}
}
func TestValidation(t *testing.T) {
	invalid := []Mesh{{Vertices: []geom.Vec3{{X: math.NaN()}}}, {Vertices: []geom.Vec3{{}}, Triangles: [][3]int{{0, 1, 2}}}, {Vertices: []geom.Vec3{{}, {X: 1}, {X: 2}}, Triangles: [][3]int{{0, 1, 2}}}}
	for _, m := range invalid {
		if m.Validate() == nil {
			t.Fatal("accepted invalid mesh")
		}
	}
	m, _ := Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	m.Triangles = m.Triangles[1:]
	if m.ValidateSolid() == nil {
		t.Fatal("accepted open solid")
	}
	m, _ = Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	m.Triangles[0][1], m.Triangles[0][2] = m.Triangles[0][2], m.Triangles[0][1]
	if m.ValidateSolid() == nil {
		t.Fatal("accepted inconsistent winding")
	}
	if err := (Mesh{}).ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
func TestPinchedVertex(t *testing.T) {
	a, _ := Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	b, _ := Box(geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{})
	// Merge the coincident corner of two otherwise disjoint closed shells.
	mapping := make([]int, len(b.Vertices))
	for i, v := range b.Vertices {
		if v == (geom.Vec3{}) {
			mapping[i] = 0
		} else {
			mapping[i] = len(a.Vertices)
			a.Vertices = append(a.Vertices, v)
		}
	}
	for _, f := range b.Triangles {
		a.Triangles = append(a.Triangles, [3]int{mapping[f[0]], mapping[f[1]], mapping[f[2]]})
	}
	if a.ValidateSolid() == nil {
		t.Fatal("accepted pinched vertex")
	}
}
func TestInvalidPrimitives(t *testing.T) {
	if _, e := Box(geom.Vec3{}, geom.Vec3{}); e == nil {
		t.Fatal("box")
	}
	if _, e := Sphere(geom.Vec3{}, -1, 8, 4); e == nil {
		t.Fatal("sphere")
	}
	if _, e := Cylinder(geom.Vec3{}, 1, 0, 8); e == nil {
		t.Fatal("cylinder")
	}
}
func FuzzBoxVolume(f *testing.F) {
	f.Add(1.0, 2.0, 3.0)
	f.Fuzz(func(t *testing.T, x, y, z float64) {
		if !finite(x) || !finite(y) || !finite(z) || x < 1e-4 || y < 1e-4 || z < 1e-4 || x > 1e4 || y > 1e4 || z > 1e4 {
			return
		}
		m, e := Box(geom.Vec3{}, geom.Vec3{X: x, Y: y, Z: z})
		if e != nil {
			t.Fatal(e)
		}
		closeValue(t, m.SignedVolume(), x*y*z, x*y*z*1e-14)
	})
}
func BenchmarkSphere(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Sphere(geom.Vec3{}, 1, 64, 32)
	}
}

func TestTransform(t *testing.T) {
	m, _ := Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	r, e := m.Transformed(geom.Translation(geom.Vec3{X: 4}).Mul(geom.Scaling(geom.Vec3{X: -2, Y: 3, Z: 4})))
	if e != nil {
		t.Fatal(e)
	}
	if e = r.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	closeValue(t, r.SignedVolume(), 144, 1e-10)
	bounds, e := r.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	if bounds.Min.X != 2 || bounds.Max.X != 4 {
		t.Fatal(bounds)
	}
	if _, e = m.Transformed(geom.Affine{}); e == nil {
		t.Fatal("accepted singular transform")
	}
}

func TestMassProperties(t *testing.T) {
	m, _ := Box(geom.Vec3{X: 100, Y: 200, Z: 300}, geom.Vec3{X: 102, Y: 204, Z: 306})
	props, e := m.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	if props.Centroid != (geom.Vec3{X: 101, Y: 202, Z: 303}) {
		t.Fatal(props)
	}
	closeValue(t, props.Volume, 48, 1e-12)
	reversed, e := m.Reversed().MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	if reversed.Centroid != props.Centroid || reversed.Volume != -props.Volume {
		t.Fatal(reversed)
	}
	// An L-section has area 3 and centroid (5/6,5/6); extrusion adds z=1.5.
	p := []geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	m, e = Extrude(p, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	props, e = m.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, props.Centroid.X, 5.0/6, 1e-12)
	closeValue(t, props.Centroid.Y, 5.0/6, 1e-12)
	closeValue(t, props.Centroid.Z, 1.5, 1e-12)
	if _, e = (Mesh{}).MassProperties(); e == nil {
		t.Fatal("empty centroid accepted")
	}
}

func TestExtrudeWindingScaleAndObliquePlane(t *testing.T) {
	for _, scale := range []float64{1e-8, 1, 1e8} {
		p := []geom.Vec3{{}, {X: 2 * scale, Z: 2 * scale}, {X: 2 * scale, Y: 3 * scale, Z: 2 * scale}, {Y: 3 * scale}}
		for reverse := 0; reverse < 2; reverse++ {
			m, e := Extrude(p, geom.Vec3{Z: 4 * scale})
			if e != nil {
				t.Fatal(e)
			}
			expected := 24 * scale * scale * scale
			closeValue(t, m.SignedVolume(), expected, expected*1e-13)
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}
	}
}

func TestDisjointAndCavityProperties(t *testing.T) {
	outer, _ := Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	inner, _ := Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 3, Y: 3, Z: 3})
	inner = inner.Reversed()
	shift := len(outer.Vertices)
	outer.Vertices = append(outer.Vertices, inner.Vertices...)
	for _, f := range inner.Triangles {
		outer.Triangles = append(outer.Triangles, [3]int{f[0] + shift, f[1] + shift, f[2] + shift})
	}
	p, e := outer.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	closeValue(t, p.Volume, 56, 1e-12)
	closeValue(t, p.Centroid.X, 2, 1e-12)
	closeValue(t, p.Centroid.Y, 2, 1e-12)
	closeValue(t, p.Centroid.Z, 2, 1e-12)
}

func FuzzExtrudeRectangle(f *testing.F) {
	f.Add(1.0, 2.0, 3.0)
	f.Fuzz(func(t *testing.T, x, y, z float64) {
		if !finite(x) || !finite(y) || !finite(z) || x < 0.01 || y < 0.01 || z < 0.01 || x > 100 || y > 100 || z > 100 {
			return
		}
		m, e := Extrude([]geom.Vec3{{}, {X: x}, {X: x, Y: y}, {Y: y}}, geom.Vec3{Z: z})
		if e != nil {
			t.Fatal(e)
		}
		closeValue(t, m.SignedVolume(), x*y*z, x*y*z*1e-13)
	})
}
