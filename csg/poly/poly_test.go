package poly

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/csg"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/solid"
	"testing"
)

func box(t *testing.T, x, y, z, xx, yy, zz float64) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{X: x, Y: y, Z: z}, geom.Vec3{X: xx, Y: yy, Z: zz})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func volume(t *testing.T, s brep.Solid) float64 {
	t.Helper()
	if len(s.Faces()) == 0 {
		return 0
	}
	p, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	return p.Volume
}
func run(t *testing.T, a, b brep.Solid, op Operation, want float64) brep.Solid {
	t.Helper()
	s, e := Combine(context.Background(), a, b, op, Options{})
	if e != nil {
		t.Fatal(e)
	}
	got := volume(t, s)
	if math.Abs(got-want) > 1e-8*math.Max(1, math.Abs(want)) {
		t.Fatalf("op %d volume %.14g want %.14g", op, got, want)
	}
	return s
}
func TestBoxes(t *testing.T) {
	a := box(t, 0, 0, 0, 2, 2, 2)
	for _, c := range []struct {
		name string
		b    brep.Solid
		i    float64
	}{{"overlap", box(t, 1, 1, 1, 3, 3, 3), 1}, {"self", a, 8}, {"disjoint", box(t, 3, 3, 3, 5, 5, 5), 0}, {"face", box(t, 2, 0, 0, 4, 2, 2), 0}, {"contained", box(t, .5, .5, .5, 1.5, 1.5, 1.5), 1}} {
		t.Run(c.name, func(t *testing.T) {
			vb := volume(t, c.b)
			run(t, a, c.b, Union, 8+vb-c.i)
			run(t, a, c.b, Intersection, c.i)
			run(t, a, c.b, Difference, 8-c.i)
		})
	}
}
func TestEmptyAndBudgets(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	run(t, a, brep.Solid{}, Union, 1)
	run(t, a, brep.Solid{}, Intersection, 0)
	run(t, a, brep.Solid{}, Difference, 1)
	run(t, brep.Solid{}, a, Difference, 0)
	for _, o := range []Options{{MaxWork: 1}, {MaxFragments: 1}, {MaxDepth: 1}, {MaxBits: 1}, {MaxVertices: 1}, {MaxInputTriangles: 1}, {MaxOutputTriangles: 1}} {
		_, e := Combine(context.Background(), a, box(t, .5, .5, .5, 1.5, 1.5, 1.5), Union, o)
		if !errors.Is(e, ErrLimit) {
			t.Fatalf("%+v: %v", o, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Combine(ctx, a, a, Union, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := Combine(nil, a, a, Union, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func cells(t *testing.T, coords [][3]int, xf geom.Affine) (brep.Solid, []brep.Solid) {
	t.Helper()
	var boxes []geom.Box
	var pieces []brep.Solid
	for _, p := range coords {
		b := geom.Box{Min: geom.Vec3{X: float64(p[0]), Y: float64(p[1]), Z: float64(p[2])}, Max: geom.Vec3{X: float64(p[0] + 1), Y: float64(p[1] + 1), Z: float64(p[2] + 1)}}
		boxes = append(boxes, b)
		s, e := brep.Box(b.Min, b.Max)
		if e != nil {
			t.Fatal(e)
		}
		s, e = s.Transformed(xf)
		if e != nil {
			t.Fatal(e)
		}
		pieces = append(pieces, s)
	}
	s, e := solid.New(boxes...)
	if e != nil {
		t.Fatal(e)
	}
	br, e := s.BRep(solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	br, e = br.Transformed(xf)
	if e != nil {
		t.Fatal(e)
	}
	return br, pieces
}
func TestNonconvexThreeDimensionalIndependentOracle(t *testing.T) {
	slowtest.Skip(t)
	a, aa := cells(t, [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, geom.Affine{{1, .25, 0, 0}, {0, 1, .25, 0}, {.25, 0, 1, 0}})
	b, bb := cells(t, [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 1, 1}}, geom.Affine{{1, 0, .25, .5}, {.25, 1, 0, .25}, {0, .25, 1, .125}})
	iv := 0.
	for _, x := range aa {
		for _, y := range bb {
			s, e := csg.Intersect(x, y, 1e-10)
			if e != nil {
				t.Fatal(e)
			}
			iv += volume(t, s)
		}
	}
	t.Logf("independent pairwise convex-cell intersection volume %.14g", iv)
	run(t, a, b, Intersection, iv)
	run(t, a, b, Union, volume(t, a)+volume(t, b)-iv)
	run(t, a, b, Difference, volume(t, a)-iv)
}
func TestRejectInvertedShell(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	_, e := Combine(context.Background(), a.Reversed(), a, Union, Options{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func TestRejectNonmanifoldContact(t *testing.T) {
	a := box(t, 0, 0, 0, 1, 1, 1)
	for _, b := range []brep.Solid{box(t, 1, 1, 0, 2, 2, 1), box(t, 1, 1, 1, 2, 2, 2)} {
		if _, e := Combine(context.Background(), a, b, Union, Options{}); e == nil {
			t.Fatal("accepted nonmanifold contact")
		}
	}
}

func TestCavityNestingAndIslands(t *testing.T) {
	slowtest.Skip(t)
	outer := box(t, 0, 0, 0, 6, 6, 6)
	inner := box(t, 1, 1, 1, 5, 5, 5)
	island := box(t, 2, 2, 2, 4, 4, 4)
	cavity := run(t, outer, inner, Difference, 152)
	withIsland := run(t, cavity, island, Union, 160)
	run(t, withIsland, outer, Intersection, 160)
	run(t, outer, withIsland, Difference, 56)
	// Two outward nested shells violate material parity.
	m := outer.Tessellate()
	n := inner.Tessellate()
	offset := len(m.Vertices)
	m.Vertices = append(m.Vertices, n.Vertices...)
	for _, tr := range n.Triangles {
		m.Triangles = append(m.Triangles, [3]int{tr[0] + offset, tr[1] + offset, tr[2] + offset})
	}
	ps := make([]polygon, len(m.Triangles))
	for i, tr := range m.Triangles {
		v := []point{vec(m.Vertices[tr[0]]), vec(m.Vertices[tr[1]]), vec(m.Vertices[tr[2]])}
		ps[i] = polygon{v: v, p: makePlane(v[0], v[1], v[2])}
	}
	o, _ := defaults(Options{})
	if e := orientations(m, ps, &budget{ctx: context.Background(), o: o}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func TestConversionBoundAndOptions(t *testing.T) {
	a := box(t, 0, 0, 0, 2, 2, 2)
	b, e := box(t, 0, 0, 0, 2, 2, 2).Transformed(geom.Affine{{1, .25, 0, .25}, {0, 1, .25, .25}, {.25, 0, 1, .25}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Combine(context.Background(), a, b, Intersection, Options{MaxConversionError: 1e-30}); !errors.Is(e, ErrPrecision) {
		t.Fatal(e)
	}
	if _, e = Combine(context.Background(), a, b, Intersection, Options{MaxConversionError: 1, RequireExactConversion: true}); !errors.Is(e, ErrPrecision) {
		t.Fatal("exact conversion allowed rounding", e)
	}
	for _, o := range []Options{{MaxWork: -1}, {MaxConversionError: -1}, {MaxConversionError: math.NaN()}, {MaxConversionError: math.Inf(1)}} {
		if _, e = Combine(context.Background(), a, b, Union, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	if _, e = Combine(context.Background(), a, b, Operation(99), Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func FuzzBoxVolumeIdentities(f *testing.F) {
	f.Add(uint8(3), uint8(5), uint8(1))
	f.Add(uint8(8), uint8(8), uint8(8))
	f.Fuzz(func(t *testing.T, x, y, z uint8) {
		a := box(t, 0, 0, 0, 2, 2, 2)
		dx, dy, dz := float64(x%32)/8, float64(y%32)/8, float64(z%32)/8
		b := box(t, dx, dy, dz, dx+2, dy+2, dz+2)
		iv := math.Max(0, 2-dx) * math.Max(0, 2-dy) * math.Max(0, 2-dz)
		run(t, a, b, Intersection, iv)
		if (dx == 2 && dy >= 2) || (dx == 2 && dz >= 2) || (dy == 2 && dz >= 2) {
			return
		}
		run(t, a, b, Union, 16-iv)
		run(t, a, b, Difference, 8-iv)
	})
}

// concatenate preserves separate shell identity, deliberately without sewing.
func concatenate(t *testing.T, ss ...brep.Solid) brep.Solid {
	t.Helper()
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	for _, s := range ss {
		vo, eo := len(vertices), len(edges)
		vertices = append(vertices, s.Vertices()...)
		for _, e := range s.Edges() {
			edges = append(edges, brep.Edge{Start: e.Start + vo, End: e.End + vo})
		}
		for _, f := range s.Faces() {
			for i := range f.Boundary {
				f.Boundary[i].Edge += eo
			}
			for _, h := range f.Holes {
				for i := range h {
					h[i].Edge += eo
				}
			}
			faces = append(faces, f)
		}
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestInputEmbeddingAndNesting(t *testing.T) {
	a := box(t, 0, 0, 0, 4, 4, 4)
	for _, bad := range []brep.Solid{concatenate(t, a, box(t, 1, 1, 1, 2, 2, 2)), concatenate(t, a, box(t, 3, 3, 3, 5, 5, 5)), concatenate(t, a, box(t, 5, 5, 5, 6, 6, 6).Reversed())} {
		if _, e := Combine(context.Background(), bad, a, Union, Options{}); e == nil {
			t.Fatal("invalid material boundary accepted")
		}
	}
	good := concatenate(t, a, box(t, 1, 1, 1, 2, 2, 2).Reversed())
	run(t, good, a, Intersection, 63)
}
func TestNonconvexDisjointAndSharedFace(t *testing.T) {
	a, _ := cells(t, [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, geom.Identity())
	b, _ := cells(t, [][3]int{{2, 0, 0}, {3, 0, 0}, {3, 1, 0}, {3, 0, 1}}, geom.Identity())
	run(t, a, b, Union, 8)
	run(t, a, b, Intersection, 0)
	run(t, a, b, Difference, 4)
	b, e := b.Transformed(geom.Translation(geom.Vec3{X: 5}))
	if e != nil {
		t.Fatal(e)
	}
	run(t, a, b, Union, 8)
	run(t, a, b, Intersection, 0)
	run(t, a, b, Difference, 4)
}

func TestHistory(t *testing.T) {
	a := box(t, 0, 0, 0, 3, 3, 3)
	b := box(t, 1, 1, 1, 2, 2, 2)
	for _, op := range []Operation{Union, Intersection, Difference} {
		s, h, e := CombineWithHistory(context.Background(), a, b, op, Options{})
		if e != nil {
			t.Fatal(e)
		}
		if len(h.Faces) != len(s.Faces()) {
			t.Fatal("face history length")
		}
		am, bm, rm := a.Tessellate(), b.Tessellate(), s.Tessellate()
		for i, src := range h.Faces {
			im, fmap := am, a.TriangleFaces()
			entries := h.A
			if src.Operand == OperandB {
				im, fmap, entries = bm, b.TriangleFaces(), h.B
			}
			if src.Face != fmap[src.Triangle] {
				t.Fatal("original face ancestry")
			}
			if entries[src.Triangle].State == Deleted {
				t.Fatal("live face marked deleted")
			}
			it, rt := im.Triangles[src.Triangle], rm.Triangles[i]
			in := im.Vertices[it[1]].Sub(im.Vertices[it[0]]).Cross(im.Vertices[it[2]].Sub(im.Vertices[it[0]]))
			rn := rm.Vertices[rt[1]].Sub(rm.Vertices[rt[0]]).Cross(rm.Vertices[rt[2]].Sub(rm.Vertices[rt[0]]))
			if (in.Dot(rn) < 0) != src.Reversed {
				t.Fatal("orientation ancestry")
			}
			if src.Reversed != (op == Difference && src.Operand == OperandB) {
				t.Fatal("unexpected reversal")
			}
		}
		for _, hist := range [][]TriangleHistory{h.A, h.B} {
			for _, entry := range hist {
				if (entry.State == Deleted) != (len(entry.OutputFaces) == 0) {
					t.Fatal("deletion mismatch")
				}
			}
		}
	}
	c := box(t, 1, 1, 1, 4, 4, 4)
	s, h, e := CombineWithHistory(context.Background(), a, c, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	seen := make([]bool, len(s.Faces()))
	for _, hist := range [][]TriangleHistory{h.A, h.B} {
		for _, entry := range hist {
			if entry.State == Fragmented {
				count++
			}
			for _, i := range entry.OutputFaces {
				if seen[i] {
					t.Fatal("duplicate ancestry")
				}
				seen[i] = true
			}
		}
	}
	if count == 0 {
		t.Fatal("no fragments reported")
	}
	for _, v := range seen {
		if !v {
			t.Fatal("missing ancestry")
		}
	}
	_, h, e = CombineWithHistory(context.Background(), a, a, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range h.B {
		if entry.State != Deleted {
			t.Fatal("coincident owner should be A")
		}
	}
	_, h, e = CombineWithHistory(context.Background(), a, brep.Solid{}, Union, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range h.A {
		if entry.State != Retained {
			t.Fatal("empty union ancestry")
		}
	}
	_, h, e = CombineWithHistory(context.Background(), a, c, Union, Options{MaxWork: 1})
	if e == nil || len(h.Faces) != 0 || len(h.A) != 0 {
		t.Fatal("partial history on failure")
	}
}

func TestNonconvexIdempotence(t *testing.T) {
	a, _ := cells(t, [][3]int{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, geom.Affine{{1, .25, 0, 0}, {0, 1, .25, 0}, {.25, 0, 1, 0}})
	v := volume(t, a)
	run(t, a, a, Union, v)
	run(t, a, a, Intersection, v)
	run(t, a, a, Difference, 0)
}
