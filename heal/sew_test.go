package heal

import (
	"errors"
	"math"
	"math/rand"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func cubeFaces() []Face {
	p := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}, {Z: 1}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 1}, {Y: 1, Z: 1}}
	ids := [][]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}
	f := make([]Face, len(ids))
	for i, ring := range ids {
		for _, j := range ring {
			f[i].Outer = append(f[i].Outer, p[j])
		}
	}
	return f
}
func reverseInput(face *Face) {
	for _, ring := range append([][]geom.Vec3{face.Outer}, face.Holes...) {
		for i, j := 0, len(ring)-1; i < j; i, j = i+1, j-1 {
			ring[i], ring[j] = ring[j], ring[i]
		}
	}
}
func near(t *testing.T, a, b, tolerance float64) {
	t.Helper()
	if math.Abs(a-b) > tolerance {
		t.Fatalf("got %.17g want %.17g", a, b)
	}
}
func TestSewExactCubeAndRemap(t *testing.T) {
	input := cubeFaces()
	s, r, e := Sew(input, Options{})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 1, 1e-12)
	if r.InputVertices != 24 || r.WeldedVertices != 16 || len(r.Vertices) != 8 || r.MaxDisplacement != 0 || len(r.FlippedFaces) != 0 {
		t.Fatal(r)
	}
	for f, face := range input {
		for i, p := range face.Outer {
			if r.Vertices[r.VertexMap[f].Outer[i]] != p {
				t.Fatal("bad remap")
			}
		}
	}
	if s.Topology() != (brep.Topology{Vertices: 8, Edges: 12, Faces: 6, Shells: 1, EulerCharacteristic: 2}) {
		t.Fatal(s.Topology())
	}
	r.Vertices[0].X = 999
	r.VertexMap[0].Outer[0] = 999
	input[0].Outer[0].X = 999
	if e = s.Validate(); e != nil {
		t.Fatal("result aliases input/report", e)
	}
}
func TestOrientationPolicies(t *testing.T) {
	input := cubeFaces()
	reverseInput(&input[2])
	if _, _, e := Sew(input, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal("unrepaired orientation should fail", e)
	}
	s, r, e := Sew(input, Options{RepairOrientation: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 1, 1e-12)
	if len(r.FlippedFaces) != 1 || r.FlippedFaces[0] != 2 {
		t.Fatal(r.FlippedFaces)
	}
	input = cubeFaces()
	reverseInput(&input[0])
	s, r, e = Sew(input, Options{RepairOrientation: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), -1, 1e-12)
	if len(r.FlippedFaces) != 5 {
		t.Fatal(r.FlippedFaces)
	}
	s, r, e = Sew(input, Options{OrientOutward: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 1, 1e-12)
	if len(r.FlippedFaces) != 1 || r.FlippedFaces[0] != 0 {
		t.Fatal(r.FlippedFaces)
	}
}
func TestNoisyTriangles(t *testing.T) {
	var input []Face
	for _, face := range cubeFaces() {
		for _, ids := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
			var triangle []geom.Vec3
			for _, id := range ids {
				p := face.Outer[id]
				p.X += float64(len(input)%5) * 1e-7
				triangle = append(triangle, p)
			}
			input = append(input, Face{Outer: triangle})
		}
	}
	s, r, e := Sew(input, Options{Tolerance: 1e-6, OrientOutward: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 1, 1e-6)
	if len(r.Vertices) != 8 || r.MaxDisplacement > 1e-6 {
		t.Fatal(r)
	}
	for f, face := range input {
		for i, p := range face.Outer {
			if p.Sub(r.Vertices[r.VertexMap[f].Outer[i]]).Length() > 1e-6 {
				t.Fatal("weld exceeded displacement bound")
			}
		}
	}
	if _, _, e = Sew(input, Options{}); !errors.Is(e, ErrOpenShell) {
		t.Fatal("exact sewing should retain gaps", e)
	}
}
func TestDiagnosticFailures(t *testing.T) {
	input := cubeFaces()
	_, r, e := Sew(input[:5], Options{})
	if !errors.Is(e, ErrOpenShell) || len(r.OpenEdges) != 4 {
		t.Fatal(e, r.OpenEdges)
	}
	input = append(cubeFaces(), cubeFaces()[0])
	_, r, e = Sew(input, Options{})
	if !errors.Is(e, ErrNonManifold) || len(r.NonManifoldEdges) != 4 {
		t.Fatal(e, r.NonManifoldEdges)
	}
	if len(r.NonManifoldEdges[0].Faces) != 3 {
		t.Fatal(r.NonManifoldEdges)
	}
	input = cubeFaces()
	input[0].Outer[0].Z = -1e-7
	_, _, e = Sew(input, Options{Tolerance: 1e-6})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal("warped face must fail explicitly", e)
	}
	if _, _, e = Sew([]Face{{Outer: []geom.Vec3{{}, {X: 0.1}, {Y: 1}}}}, Options{Tolerance: 0.2}); !errors.Is(e, ErrInvalid) {
		t.Fatal("collapsed face accepted", e)
	}
}
func TestWeldingDoesNotChainAndIsOrderIndependent(t *testing.T) {
	input := []Face{{Outer: []geom.Vec3{{}, {X: 10}, {Y: 10}}}, {Outer: []geom.Vec3{{X: 0.75}, {X: 20}, {X: 20, Y: 10}}}, {Outer: []geom.Vec3{{X: 1.5}, {X: 30}, {X: 30, Y: 10}}}}
	_, r, e := Sew(input, Options{Tolerance: 1})
	if !errors.Is(e, ErrOpenShell) {
		t.Fatal(e)
	}
	a, b, c := r.VertexMap[0].Outer[0], r.VertexMap[1].Outer[0], r.VertexMap[2].Outer[0]
	if a != b || a == c {
		t.Fatal("proximity chain expanded a cluster", r.VertexMap)
	}
	input[0], input[2] = input[2], input[0]
	_, q, e := Sew(input, Options{Tolerance: 1})
	if !errors.Is(e, ErrOpenShell) {
		t.Fatal(e)
	}
	if len(r.Vertices) != len(q.Vertices) {
		t.Fatal("order changed clustering")
	}
	for i, v := range r.Vertices {
		if v != q.Vertices[i] {
			t.Fatal("order changed representatives")
		}
	}
}
func independent(s brep.Solid) []Face {
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	result := make([]Face, len(faces))
	for f, face := range faces {
		for loop, uses := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
			points := make([]geom.Vec3, len(uses))
			for i, u := range uses {
				edge := edges[u.Edge]
				id := edge.Start
				if u.Reversed {
					id = edge.End
				}
				points[i] = vertices[id]
			}
			if loop == 0 {
				result[f].Outer = points
			} else {
				result[f].Holes = append(result[f].Holes, points)
			}
		}
	}
	return result
}
func TestHoledFacesAndNestedShellPolicy(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 3}, {X: 1, Y: 3}}
	tube, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	input := independent(tube)
	for i := range input {
		if i%2 == 0 {
			reverseInput(&input[i])
		}
	}
	s, r, e := Sew(input, Options{OrientOutward: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 36, 1e-11)
	if s.Topology().EulerCharacteristic != 0 || len(r.VertexMap[0].Holes) != 1 {
		t.Fatal(s.Topology(), r.VertexMap)
	}
	large, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	small, e := brep.Box(geom.Vec3{X: 1, Y: 1, Z: 1}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	input = append(independent(large), independent(small.Reversed())...)
	s, _, e = Sew(input, Options{RepairOrientation: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 63, 1e-11)
	s, _, e = Sew(input, Options{OrientOutward: true})
	if e != nil {
		t.Fatal(e)
	}
	near(t, s.Tessellate().SignedVolume(), 65, 1e-11) // Explicit outward-per-shell policy does not infer cavities.
}
func TestInvalidAndLimits(t *testing.T) {
	for _, tol := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, _, e := Sew(nil, Options{Tolerance: tol}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	if _, _, e := Sew(make([]Face, MaxFaces+1), Options{}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, _, e := Sew([]Face{{Outer: make([]geom.Vec3, MaxInputVertices+1)}}, Options{}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	for _, p := range [][]geom.Vec3{nil, {{X: math.NaN()}, {X: 1}, {Y: 1}}} {
		if _, _, e := Sew([]Face{{Outer: p}}, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	s, r, e := Sew(nil, Options{})
	if e != nil || len(s.Vertices()) != 0 || r.InputVertices != 0 {
		t.Fatal(e, r)
	}
}
func FuzzSewOrientation(f *testing.F) {
	f.Add(uint64(7))
	f.Fuzz(func(t *testing.T, seed uint64) {
		input := cubeFaces()
		rng := rand.New(rand.NewSource(int64(seed)))
		rng.Shuffle(len(input), func(i, j int) { input[i], input[j] = input[j], input[i] })
		for i := range input {
			if rng.Intn(2) == 0 {
				reverseInput(&input[i])
			}
		}
		s, _, e := Sew(input, Options{OrientOutward: true})
		if e != nil {
			t.Fatal(e)
		}
		near(t, s.Tessellate().SignedVolume(), 1, 1e-12)
	})
}

func TestNonOrientableIncidence(t *testing.T) {
	// Minimal six-vertex triangulation of the projective plane: every edge has
	// two incident faces, but no consistent global face orientation exists.
	indices := [][3]int{{1, 2, 3}, {1, 2, 4}, {1, 3, 5}, {1, 4, 6}, {1, 5, 6}, {2, 3, 6}, {2, 4, 5}, {2, 5, 6}, {3, 4, 5}, {3, 4, 6}}
	vertices := make([]geom.Vec3, 7)
	for i := 1; i < 7; i++ {
		x := float64(i)
		vertices[i] = geom.Vec3{X: x, Y: x * x, Z: x * x * x}
	}
	input := make([]Face, len(indices))
	for i, triangle := range indices {
		for _, v := range triangle {
			input[i].Outer = append(input[i].Outer, vertices[v])
		}
	}
	_, r, e := Sew(input, Options{RepairOrientation: true})
	if !errors.Is(e, ErrNonOrientable) {
		t.Fatal(e)
	}
	if len(r.OpenEdges) != 0 || len(r.NonManifoldEdges) != 0 {
		t.Fatal("incidence fixture is not edge-manifold", r)
	}
}
