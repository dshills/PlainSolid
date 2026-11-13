package csg

import (
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestConvexOffsetCube(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	for _, d := range []float64{-0.25, 0, 0.25, 1} {
		result, e := Offset(s, d, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Tessellate().SignedVolume(), math.Pow(1+2*d, 3))
		bounds, e := result.Bounds()
		if e != nil {
			t.Fatal(e)
		}
		near(t, bounds.Min.X, -d)
		near(t, bounds.Max.Z, 1+d)
		if e = result.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, d := range []float64{-0.5, -1} {
		result, e := Offset(s, d, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Vertices()) != 0 {
			t.Fatal("collapsed erosion must be empty")
		}
	}
	r, e := Offset(brep.Solid{}, 1, 1e-9)
	if e != nil || len(r.Vertices()) != 0 {
		t.Fatal(e)
	}
}
func solidFromTriangles(t *testing.T, vertices []geom.Vec3, triangles [][3]int) brep.Solid {
	t.Helper()
	var edges []brep.Edge
	faces := make([]brep.Face, len(triangles))
	lookup := map[[2]int]int{}
	for f, triangle := range triangles {
		for i, a := range triangle {
			b := triangle[(i+1)%3]
			lo, hi := a, b
			if lo > hi {
				lo, hi = hi, lo
			}
			key := [2]int{lo, hi}
			edge, ok := lookup[key]
			if !ok {
				edge = len(edges)
				lookup[key] = edge
				edges = append(edges, brep.Edge{Start: lo, End: hi})
			}
			faces[f].Boundary = append(faces[f].Boundary, brep.EdgeUse{Edge: edge, Reversed: a > b})
		}
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestRegularTetrahedronOffset(t *testing.T) {
	s := solidFromTriangles(t, []geom.Vec3{{X: 1, Y: 1, Z: 1}, {X: -1, Y: -1, Z: 1}, {X: -1, Y: 1, Z: -1}, {X: 1, Y: -1, Z: -1}}, [][3]int{{1, 2, 3}, {0, 3, 2}, {0, 1, 3}, {0, 2, 1}})
	near(t, s.Tessellate().SignedVolume(), 8.0/3)
	for _, distance := range []float64{-0.1, 0.2} {
		result, e := Offset(s, distance, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		scale := 1 + distance*math.Sqrt(3)
		near(t, result.Tessellate().SignedVolume(), 8.0/3*scale*scale*scale)
		if result.Topology().Vertices != 4 {
			t.Fatal(result.Topology())
		}
	}
	result, e := Offset(s, -1/math.Sqrt(3), 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Vertices()) != 0 {
		t.Fatal("inradius erosion should collapse")
	}
}
func findEdge(t *testing.T, s brep.Solid, a, b geom.Vec3) int {
	t.Helper()
	v := s.Vertices()
	for i, e := range s.Edges() {
		if v[e.Start] == a && v[e.End] == b || v[e.Start] == b && v[e.End] == a {
			return i
		}
	}
	t.Fatal("edge not found")
	return -1
}
func TestSingleAndMultipleChamfers(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	edge := findEdge(t, s, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: 1, Z: 1})
	result, e := Chamfer(s, []int{edge}, 0.2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, result.Tessellate().SignedVolume(), 0.98)
	if result.Topology().Vertices != 10 || result.Topology().Faces != 7 || result.Topology().EulerCharacteristic != 2 {
		t.Fatal(result.Topology())
	}
	for _, p := range result.Vertices() {
		if p.X+p.Y > 1.8+1e-9 {
			t.Fatal("chamfer setback plane violated", p)
		}
	}
	other := findEdge(t, s, geom.Vec3{}, geom.Vec3{Z: 1})
	result, e = Chamfer(s, []int{edge, other, edge}, 0.2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, result.Tessellate().SignedVolume(), 0.96)
	all := make([]int, len(s.Edges()))
	for i := range all {
		all[i] = i
	}
	result, e = Chamfer(s, all, 0.1, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if result.Topology().Faces != 18 || result.Topology().EulerCharacteristic != 2 {
		t.Fatal(result.Topology())
	}
	volume := result.Tessellate().SignedVolume()
	if volume <= 0 || volume >= 1 {
		t.Fatal(volume)
	}
	result, e = Chamfer(s, []int{edge}, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(result.Vertices()) != 0 {
		t.Fatal("oversized bevel should collapse")
	}
}
func TestFeatureRigidTransform(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	rotation, e := geom.Rotation(geom.Vec3{X: 1, Y: 2, Z: 3}, 0.7)
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Translation(geom.Vec3{X: 3, Y: -4, Z: 7}).Mul(rotation)
	rotated, e := s.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	result, e := Offset(rotated, 0.2, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	near(t, result.Tessellate().SignedVolume(), math.Pow(1.4, 3))
	result, e = Chamfer(rotated, []int{0}, 0.2, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	near(t, result.Tessellate().SignedVolume(), 0.98)
}
func TestSubdividedPlanarSidesAndFeatureErrors(t *testing.T) {
	cube := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	m := cube.Tessellate()
	triangulated := solidFromTriangles(t, m.Vertices, m.Triangles)
	result, e := Offset(triangulated, 0.1, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	near(t, result.Tessellate().SignedVolume(), math.Pow(1.2, 3))
	// A diagonal across one cube side is topological but geometrically coplanar.
	diagonal := findEdge(t, triangulated, m.Vertices[m.Triangles[0][0]], m.Vertices[m.Triangles[0][2]])
	// Find any edge whose two supporting planes are equal, independent of cap triangulation.
	found := false
	for index, edge := range triangulated.Edges() {
		v := triangulated.Vertices()
		a, b := v[edge.Start], v[edge.End]
		different := 0
		if a.X != b.X {
			different++
		}
		if a.Y != b.Y {
			different++
		}
		if a.Z != b.Z {
			different++
		}
		if different == 2 {
			diagonal = index
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no coplanar diagonal")
	}
	if _, e = Chamfer(triangulated, []int{diagonal}, 0.1, 1e-9); e == nil {
		t.Fatal("accepted coplanar edge")
	}
	for _, d := range []float64{math.NaN(), math.Inf(1)} {
		if _, e = Offset(cube, d, 1e-9); e == nil {
			t.Fatal("bad offset")
		}
		if _, e = Chamfer(cube, []int{0}, d, 1e-9); e == nil {
			t.Fatal("bad bevel")
		}
	}
	if _, e = Chamfer(cube, []int{0}, -1, 1e-9); e == nil {
		t.Fatal("negative bevel")
	}
	if _, e = Chamfer(cube, []int{-1}, 1, 1e-9); e == nil {
		t.Fatal("bad index")
	}
	if _, e = Offset(cube, 1, 0); e == nil {
		t.Fatal("bad tolerance")
	}
	if _, e = Offset(cube.Reversed(), 1, 1e-9); e == nil {
		t.Fatal("inward input")
	}
	for _, indices := range [][]int{nil, {0}} {
		result, e = Chamfer(cube, indices, 0, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Tessellate().SignedVolume(), 1)
	}
}
func FuzzCubeOffset(f *testing.F) {
	f.Add(0.2)
	f.Fuzz(func(t *testing.T, d float64) {
		if math.IsNaN(d) || d < -0.49 || d > 10 {
			return
		}
		s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
		result, e := Offset(s, d, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Tessellate().SignedVolume(), math.Pow(1+2*d, 3))
	})
}

func TestTetrahedronChamferEqualSetback(t *testing.T) {
	vertices := []geom.Vec3{{X: 1, Y: 1, Z: 1}, {X: -1, Y: -1, Z: 1}, {X: -1, Y: 1, Z: -1}, {X: 1, Y: -1, Z: -1}}
	s := solidFromTriangles(t, vertices, [][3]int{{1, 2, 3}, {0, 3, 2}, {0, 1, 3}, {0, 2, 1}})
	distance := 0.2
	edge := findEdge(t, s, vertices[0], vertices[1])
	result, e := Chamfer(s, []int{edge}, distance, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	// This edge has normal angle acos(-1/3). Equal face setback d places its
	// bevel at z=1-d*sqrt(2/3). Integrating the tetrahedron's quadratic slice
	// area removes the volume fraction 3*t^2-2*t^3, t=(1-z)/2.
	cutoff := 1 - distance*math.Sqrt(2.0/3)
	fraction := (1 - cutoff) / 2
	expected := 8.0 / 3 * (1 - 3*fraction*fraction + 2*fraction*fraction*fraction)
	near(t, result.Tessellate().SignedVolume(), expected)
	bounds, e := result.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	near(t, bounds.Max.Z, cutoff)
}
func TestFeatureWorkLimit(t *testing.T) {
	s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if _, e := Chamfer(s, make([]int, MaxPlanes*MaxPlanes+1), 0.1, 1e-9); e == nil {
		t.Fatal("unbounded selection")
	}
	points := make([]geom.Vec3, 46)
	for i := range points {
		angle := 2 * math.Pi * float64(i) / float64(len(points))
		points[i] = geom.Vec3{X: math.Cos(angle), Y: math.Sin(angle)}
	}
	prism, e := brep.Prism(points, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	selected := make([]int, 20)
	for i := range selected {
		selected[i] = i
	}
	if _, e = Chamfer(prism, selected, 0.1, 1e-9); e == nil {
		t.Fatal("combined supporting-plane budget")
	}
}
func FuzzCubeChamfer(f *testing.F) {
	f.Add(0.2)
	f.Fuzz(func(t *testing.T, d float64) {
		if math.IsNaN(d) || d < 0.01 || d > 0.99 {
			return
		}
		s := box(t, geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
		edge := findEdge(t, s, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: 1, Z: 1})
		result, e := Chamfer(s, []int{edge}, d, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		near(t, result.Tessellate().SignedVolume(), 1-d*d/2)
	})
}
