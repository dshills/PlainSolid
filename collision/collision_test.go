package collision

import (
	"errors"
	"math"
	"math/rand"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func baseTriangle() [3]geom.Vec3 { return [3]geom.Vec3{{}, {X: 2}, {Y: 2}} }
func TestTriangleContactKinds(t *testing.T) {
	cases := []struct {
		name   string
		b      [3]geom.Vec3
		kind   Kind
		points int
	}{
		{"proper crossing", [3]geom.Vec3{{X: 0.5, Y: 0.5, Z: -1}, {X: 0.5, Y: 0.5, Z: 1}, {X: 1.5, Y: 0.5}}, Crossing, 2},
		{"vertex touch", [3]geom.Vec3{{X: 2}, {X: 3, Z: 1}, {X: 3, Y: 1, Z: 1}}, Touching, 1},
		{"edge touch", [3]geom.Vec3{{}, {X: 2}, {X: 1, Z: 1}}, Touching, 2},
		{"coplanar containment", [3]geom.Vec3{{X: 0.2, Y: 0.2}, {X: 0.8, Y: 0.2}, {X: 0.2, Y: 0.8}}, CoplanarOverlap, 3},
		{"coplanar edge", [3]geom.Vec3{{}, {Y: -2}, {X: 2}}, Touching, 2},
		{"coplanar point", [3]geom.Vec3{{X: 2}, {X: 3, Y: -1}, {X: 3, Y: 1}}, Touching, 1},
		{"coplanar disjoint", [3]geom.Vec3{{X: 3}, {X: 4}, {X: 3, Y: 1}}, Disjoint, 0},
		{"parallel separated", [3]geom.Vec3{{Z: 1}, {X: 2, Z: 1}, {Y: 2, Z: 1}}, Disjoint, 0},
		{"noncoplanar disjoint", [3]geom.Vec3{{X: 3, Y: 0.5, Z: -1}, {X: 3, Y: 0.5, Z: 1}, {X: 4, Y: 0.5}}, Disjoint, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseTriangle()
			got, e := Triangles(a, tc.b)
			if e != nil {
				t.Fatal(e)
			}
			if got.Kind != tc.kind || len(got.Points) != tc.points {
				t.Fatal(got)
			}
			reverse, e := Triangles(tc.b, a)
			if e != nil || reverse.Kind != got.Kind {
				t.Fatal("asymmetric result", reverse, e)
			}
			tc.b[1], tc.b[2] = tc.b[2], tc.b[1]
			reverse, e = Triangles(a, tc.b)
			if e != nil || reverse.Kind != got.Kind {
				t.Fatal("winding changed classification", reverse, e)
			}
		})
	}
}
func TestCrossingEndpointsAndCoplanarHexagon(t *testing.T) {
	b := [3]geom.Vec3{{X: 0.5, Y: 0.5, Z: -1}, {X: 0.5, Y: 0.5, Z: 1}, {X: 1.5, Y: 0.5}}
	intersection, e := Triangles(baseTriangle(), b)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range intersection.Points {
		if p.Y != 0.5 || p.Z != 0 || p.X < 0.5 || p.X > 1.5 {
			t.Fatal(p)
		}
	}
	a := [3]geom.Vec3{{}, {X: 4}, {X: 2, Y: 4}}
	b = [3]geom.Vec3{{Y: 3}, {X: 4, Y: 3}, {X: 2, Y: -1}}
	intersection, e = Triangles(a, b)
	if e != nil {
		t.Fatal(e)
	}
	if intersection.Kind != CoplanarOverlap || len(intersection.Points) != 6 {
		t.Fatal(intersection)
	}
}
func TestExactExtremesAndNoToleranceSnapping(t *testing.T) {
	a := baseTriangle()
	b := [3]geom.Vec3{{X: 0.5, Y: 0.5, Z: -1}, {X: 0.5, Y: 0.5, Z: 1}, {X: 1.5, Y: 0.5}}
	for _, factor := range []float64{1e-200, 1e150} {
		aa, bb := a, b
		for i := 0; i < 3; i++ {
			aa[i] = aa[i].Scale(factor)
			bb[i] = bb[i].Scale(factor)
		}
		intersection, e := Triangles(aa, bb)
		if e != nil || intersection.Kind != Crossing {
			t.Fatal(factor, intersection, e)
		}
		for _, p := range intersection.Points {
			if !p.IsFinite() {
				t.Fatal(p)
			}
		}
	}
	separated := a
	for i := range separated {
		separated[i].Z = math.SmallestNonzeroFloat64
	}
	intersection, e := Triangles(a, separated)
	if e != nil || intersection.Kind != Disjoint {
		t.Fatal("silently snapped separated planes", intersection, e)
	}
	m := math.MaxFloat64
	a = [3]geom.Vec3{{X: -m, Y: -m}, {X: m, Y: -m}, {Y: m}}
	b = [3]geom.Vec3{{Z: -1}, {Z: 1}, {X: 1}}
	intersection, e = Triangles(a, b)
	if e != nil || intersection.Kind != Crossing {
		t.Fatal("overflow-safe exact classification failed", intersection, e)
	}
}
func TestSharedIndexedFeatures(t *testing.T) {
	cases := []struct {
		name  string
		m     mesh.Mesh
		kind  Kind
		count int
	}{
		{"ordinary shared edge", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {X: 1, Z: 1}}, Triangles: [][3]int{{0, 1, 2}, {0, 1, 3}}}, Disjoint, 0},
		{"coplanar adjacent", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {Y: -2}}, Triangles: [][3]int{{0, 1, 2}, {0, 3, 1}}}, Disjoint, 0},
		{"overlapping shared edge", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {X: 0.5, Y: 0.5}}, Triangles: [][3]int{{0, 1, 2}, {0, 1, 3}}}, CoplanarOverlap, 1},
		{"ordinary shared vertex", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {X: -1, Z: 1}, {Y: -1, Z: 1}}, Triangles: [][3]int{{0, 1, 2}, {0, 3, 4}}}, Disjoint, 0},
		{"crossing beyond shared vertex", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {X: 0.5, Y: 0.5, Z: 1}, {X: 0.5, Y: 0.5, Z: -1}}, Triangles: [][3]int{{0, 1, 2}, {0, 3, 4}}}, Crossing, 1},
		{"duplicate triangle", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}}, Triangles: [][3]int{{0, 1, 2}, {2, 1, 0}}}, CoplanarOverlap, 1},
		{"unshared coincident point", mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 2}, {Y: 2}, {}, {X: -1, Z: 1}, {Y: -1, Z: 1}}, Triangles: [][3]int{{0, 1, 2}, {3, 4, 5}}}, Touching, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, e := SelfIntersections(tc.m, Options{})
			if e != nil || !report.Complete {
				t.Fatal(e, report)
			}
			if len(report.Contacts) != tc.count {
				t.Fatal(report)
			}
			if tc.count > 0 && report.Contacts[0].Kind != tc.kind {
				t.Fatal(report.Contacts)
			}
		})
	}
}
func TestClosedBoxAndBetweenMeshes(t *testing.T) {
	box, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	report, e := SelfIntersections(box, Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal(e, report)
	}
	other, e := mesh.Box(geom.Vec3{X: 0.5, Y: 0.5, Z: 0.5}, geom.Vec3{X: 1.5, Y: 1.5, Z: 1.5})
	if e != nil {
		t.Fatal(e)
	}
	report, e = MeshIntersections(box, other, Options{})
	if e != nil || !report.Complete || len(report.Contacts) == 0 {
		t.Fatal(e, report)
	}
	proper := false
	for _, c := range report.Contacts {
		if c.Kind == Crossing {
			proper = true
		}
		if c.TriangleA < 0 || c.TriangleA >= len(box.Triangles) || c.TriangleB < 0 || c.TriangleB >= len(other.Triangles) {
			t.Fatal(c)
		}
	}
	if !proper {
		t.Fatal("missed crossing shells")
	}
	separated, e := other.Transformed(geom.Translation(geom.Vec3{X: 5}))
	if e != nil {
		t.Fatal(e)
	}
	report, e = MeshIntersections(box, separated, Options{})
	if e != nil || len(report.Contacts) != 0 || !report.Complete {
		t.Fatal(e, report)
	}
}
func TestBroadphaseAndLimits(t *testing.T) {
	var spaced mesh.Mesh
	for i := 0; i < 500; i++ {
		start := len(spaced.Vertices)
		x := float64(i) * 10
		spaced.Vertices = append(spaced.Vertices, geom.Vec3{X: x}, geom.Vec3{X: x + 1}, geom.Vec3{X: x, Y: 1})
		spaced.Triangles = append(spaced.Triangles, [3]int{start, start + 1, start + 2})
	}
	r, e := SelfIntersections(spaced, Options{MaxPairs: 1})
	if e != nil || !r.Complete || r.BroadPhasePairs != 0 || r.NarrowPhasePairs != 0 {
		t.Fatal(e, r)
	}
	dense := mesh.Mesh{Vertices: []geom.Vec3{{}, {X: 1}, {Y: 1}}}
	for i := 0; i < 10; i++ {
		dense.Triangles = append(dense.Triangles, [3]int{0, 1, 2})
	}
	r, e = SelfIntersections(dense, Options{MaxPairs: 5})
	if !errors.Is(e, ErrLimit) || r.Complete || r.BroadPhasePairs != 5 {
		t.Fatal(e, r)
	}
	r, e = SelfIntersections(dense, Options{MaxContacts: 1})
	if !errors.Is(e, ErrLimit) || r.Complete || len(r.Contacts) != 1 {
		t.Fatal(e, r)
	}
	if _, e = SelfIntersections(mesh.Mesh{Triangles: make([][3]int, MaxTriangles+1)}, Options{}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func TestInvalidInput(t *testing.T) {
	for _, options := range []Options{{MaxPairs: -1}, {MaxContacts: -1}} {
		if _, e := SelfIntersections(mesh.Mesh{}, options); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	for _, a := range [][3]geom.Vec3{{{}, {X: 1}, {X: 2}}, {{X: math.NaN()}, {X: 1}, {Y: 1}}, {{}, {}, {Y: 1}}} {
		if _, e := Triangles(a, baseTriangle()); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	for _, m := range []mesh.Mesh{{Vertices: []geom.Vec3{{X: math.Inf(1)}}}, {Vertices: []geom.Vec3{{}}, Triangles: [][3]int{{0, 1, 2}}}, {Vertices: []geom.Vec3{{}, {X: 1}, {X: 2}}, Triangles: [][3]int{{0, 1, 2}}}} {
		if _, e := SelfIntersections(m, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	r, e := SelfIntersections(mesh.Mesh{}, Options{})
	if e != nil || !r.Complete {
		t.Fatal(e, r)
	}
	r, e = MeshIntersections(mesh.Mesh{}, mesh.Mesh{}, Options{})
	if e != nil || !r.Complete {
		t.Fatal(e, r)
	}
}
func FuzzTriangleSymmetry(f *testing.F) {
	f.Add(uint64(7))
	f.Fuzz(func(t *testing.T, seed uint64) {
		rng := rand.New(rand.NewSource(int64(seed)))
		var a, b [3]geom.Vec3
		for i := 0; i < 3; i++ {
			a[i] = geom.Vec3{X: float64(rng.Intn(17) - 8), Y: float64(rng.Intn(17) - 8), Z: float64(rng.Intn(17) - 8)}
			b[i] = geom.Vec3{X: float64(rng.Intn(17) - 8), Y: float64(rng.Intn(17) - 8), Z: float64(rng.Intn(17) - 8)}
		}
		ab, e1 := Triangles(a, b)
		ba, e2 := Triangles(b, a)
		if (e1 == nil) != (e2 == nil) {
			t.Fatal(e1, e2)
		}
		if e1 != nil {
			return
		}
		if ab.Kind != ba.Kind {
			t.Fatal(ab, ba)
		}
		for i := 0; i < 3; i++ {
			a[i] = a[i].Scale(8).Add(geom.Vec3{X: 64, Y: -32, Z: 16})
			b[i] = b[i].Scale(8).Add(geom.Vec3{X: 64, Y: -32, Z: 16})
		}
		transformed, e := Triangles(a, b)
		if e != nil || transformed.Kind != ab.Kind {
			t.Fatal("exact affine covariance", transformed, ab, e)
		}
	})
}

func TestBroadphaseAgainstAllPairs(t *testing.T) {
	rng := rand.New(rand.NewSource(824))
	makeMesh := func() mesh.Mesh {
		var m mesh.Mesh
		for len(m.Triangles) < 12 {
			var tri [3]geom.Vec3
			for i := range tri {
				tri[i] = geom.Vec3{X: float64(rng.Intn(13) - 6), Y: float64(rng.Intn(13) - 6), Z: float64(rng.Intn(13) - 6)}
			}
			if validateTriangle(tri) != nil {
				continue
			}
			start := len(m.Vertices)
			m.Vertices = append(m.Vertices, tri[:]...)
			m.Triangles = append(m.Triangles, [3]int{start, start + 1, start + 2})
		}
		return m
	}
	a, b := makeMesh(), makeMesh()
	report, e := MeshIntersections(a, b, Options{})
	if e != nil {
		t.Fatal(e)
	}
	got := map[[2]int]Kind{}
	for _, contact := range report.Contacts {
		got[[2]int{contact.TriangleA, contact.TriangleB}] = contact.Kind
	}
	for i := range a.Triangles {
		for j := range b.Triangles {
			result, e := Triangles(triangle(a, i), triangle(b, j))
			if e != nil {
				t.Fatal(e)
			}
			if got[[2]int{i, j}] != result.Kind {
				t.Fatal("broadphase differs from exhaustive classification", i, j, result, got[[2]int{i, j}])
			}
		}
	}
}
func TestCoplanarProjectionAxes(t *testing.T) {
	// Cyclic coordinate permutations exercise all three exact projection axes.
	a := baseTriangle()
	b := [3]geom.Vec3{{X: 0.2, Y: 0.2}, {X: 0.8, Y: 0.2}, {X: 0.2, Y: 0.8}}
	for axis := 0; axis < 3; axis++ {
		result, e := Triangles(a, b)
		if e != nil || result.Kind != CoplanarOverlap {
			t.Fatal(result, e)
		}
		for i := 0; i < 3; i++ {
			a[i] = geom.Vec3{X: a[i].Z, Y: a[i].X, Z: a[i].Y}
			b[i] = geom.Vec3{X: b[i].Z, Y: b[i].X, Z: b[i].Y}
		}
	}
}
func BenchmarkTriangleCrossing(b *testing.B) {
	a := baseTriangle()
	other := [3]geom.Vec3{{X: 0.5, Y: 0.5, Z: -1}, {X: 0.5, Y: 0.5, Z: 1}, {X: 1.5, Y: 0.5}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := Triangles(a, other); e != nil {
			b.Fatal(e)
		}
	}
}
