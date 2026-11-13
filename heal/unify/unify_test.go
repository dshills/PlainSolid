package unify

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/solid"
	"reflect"
	"testing"
)

func box(x, y, z, xx, yy, zz float64) geom.Box {
	return geom.Box{Min: geom.Vec3{X: x, Y: y, Z: z}, Max: geom.Vec3{X: xx, Y: yy, Z: zz}}
}
func cells(t *testing.T, bs ...geom.Box) brep.Solid {
	t.Helper()
	s, e := solid.New(bs...)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.BRep(solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func check(t *testing.T, s brep.Solid, wantFaces int) Result {
	t.Helper()
	r, e := Unify(context.Background(), s, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != wantFaces {
		t.Fatal("faces", len(r.Solid.Faces()), wantFaces)
	}
	a, e := s.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	b, e := r.Solid.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(a.Volume-b.Volume) > 1e-10 || math.Abs(a.SurfaceArea-b.SurfaceArea) > 1e-10 || a.Centroid.Sub(b.Centroid).Length() > 1e-10 {
		t.Fatal("geometry changed", a, b)
	}
	if len(r.OldToNew) != len(s.Faces()) || len(r.NewToOld) != wantFaces {
		t.Fatal("history lengths")
	}
	for i, g := range r.NewToOld {
		if len(g) == 0 {
			t.Fatal("empty ancestry")
		}
		for _, old := range g {
			if r.OldToNew[old] != i {
				t.Fatal("history inverse")
			}
		}
	}
	for old, newIndex := range r.VertexMap {
		if newIndex >= 0 && r.Solid.Vertices()[newIndex] != s.Vertices()[old] {
			t.Fatal("coordinate changed")
		}
	}
	again, e := Unify(context.Background(), s, Options{})
	if e != nil || !reflect.DeepEqual(r, again) {
		t.Fatal("nondeterministic", e)
	}
	idempotent, e := Unify(context.Background(), r.Solid, Options{})
	if e != nil || len(idempotent.Solid.Faces()) != wantFaces || idempotent.RemovedEdges != 0 || idempotent.RemovedVertices != 0 {
		t.Fatal("not idempotent", e)
	}
	return r
}
func TestSubdividedBox(t *testing.T) {
	s := cells(t, box(0, 0, 0, 1, 1, 1), box(1, 0, 0, 2, 1, 1), box(0, 1, 0, 2, 2, 1))
	if len(s.Faces()) <= 6 {
		t.Fatal("fixture not subdivided")
	}
	r := check(t, s, 6)
	if r.Solid.Topology().Vertices != 8 || r.Solid.Topology().Edges != 12 || r.RemovedEdges <= 0 || r.RemovedVertices <= 0 {
		t.Fatal(r.Solid.Topology())
	}
}
func TestHoledCaps(t *testing.T) {
	s := cells(t, box(0, 0, 0, 6, 2, 2), box(0, 4, 0, 6, 6, 2), box(0, 2, 0, 2, 4, 2), box(4, 2, 0, 6, 4, 2))
	r := check(t, s, 10)
	holes := 0
	for _, f := range r.Solid.Faces() {
		holes += len(f.Holes)
	}
	if holes != 2 || r.Solid.Topology().EulerCharacteristic != 0 {
		t.Fatal("lost holes", holes, r.Solid.Topology())
	}
}
func TestNonconvexAndDisconnected(t *testing.T) {
	s := cells(t, box(0, 0, 0, 3, 1, 2), box(0, 1, 0, 1, 3, 2))
	r := check(t, s, 8)
	if r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal(r.Solid.Topology())
	}
	s = cells(t, box(0, 0, 0, 1, 1, 1), box(3, 0, 0, 4, 1, 1))
	r = check(t, s, 12)
	if r.Solid.Topology().Shells != 2 {
		t.Fatal("merged disconnected faces")
	}
}
func TestTrianglesAndExactShear(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	m := s.Tessellate()
	var es []brep.Edge
	var fs []brep.Face
	emap := map[[2]int]int{}
	for _, tr := range m.Triangles {
		f := brep.Face{}
		for i, a := range tr {
			b := tr[(i+1)%3]
			rev := a > b
			if rev {
				a, b = b, a
			}
			key := [2]int{a, b}
			id, ok := emap[key]
			if !ok {
				id = len(es)
				emap[key] = id
				es = append(es, brep.Edge{Start: a, End: b})
			}
			f.Boundary = append(f.Boundary, brep.EdgeUse{Edge: id, Reversed: rev})
		}
		fs = append(fs, f)
	}
	s, e = brep.NewSolid(m.Vertices, es, fs)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.Transformed(geom.Affine{{1, .25, 0, 3}, {0, 1, .25, -2}, {.25, 0, 1, 7}})
	if e != nil {
		t.Fatal(e)
	}
	r := check(t, s, 6)
	for _, g := range r.NewToOld {
		if len(g) != 2 {
			t.Fatal(g)
		}
	}
}
func TestLimitsAndInvalid(t *testing.T) {
	s := cells(t, box(0, 0, 0, 1, 1, 1))
	for _, o := range []Options{{MaxFaces: 1}, {MaxTriangles: 1}, {MaxWork: 1}, {MaxCollisionPairs: 1}} {
		if _, e := Unify(context.Background(), s, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	for _, o := range []Options{{MaxWork: -1}, {MaxTriangles: 100001}} {
		if _, e := Unify(context.Background(), s, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	if _, e := Unify(context.Background(), s.Reversed(), Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Unify(ctx, s, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e := Unify(context.Background(), brep.Solid{}, Options{})
	if e != nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
}

func TestMultipleHolesAndNoWelding(t *testing.T) {
	s := cells(t, box(0, 0, 0, 10, 2, 1), box(0, 4, 0, 10, 6, 1), box(0, 2, 0, 2, 4, 1), box(4, 2, 0, 6, 4, 1), box(8, 2, 0, 10, 4, 1))
	r := check(t, s, 14)
	holes := 0
	for _, f := range r.Solid.Faces() {
		holes += len(f.Holes)
	}
	if holes != 4 || r.Solid.Topology().EulerCharacteristic != -2 {
		t.Fatal(holes, r.Solid.Topology())
	}
	s = cells(t, box(0, 0, 0, 1, 1, 1), box(math.Nextafter(1, 2), 0, 0, 2, 1, 1))
	r = check(t, s, 12)
	if r.Solid.Topology().Shells != 2 || len(r.Solid.Vertices()) != 16 {
		t.Fatal("tiny gap welded")
	}
}
func TestRejectNearlyPlanarFace(t *testing.T) {
	s, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	v := s.Vertices()
	v[6].Z = math.Nextafter(v[6].Z, 3)
	warped, e := brep.NewSolid(v, s.Edges(), s.Faces())
	if e != nil {
		t.Fatal("fixture must pass tolerant BRep planarity", e)
	}
	if _, e = Unify(context.Background(), warped, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal("accepted nonexact plane", e)
	}
}

func FuzzSubdividedBox(f *testing.F) {
	f.Add(uint8(1), uint8(2), uint8(3), uint8(4))
	f.Fuzz(func(t *testing.T, a, b, c, d uint8) {
		x, y := float64(a%4+1)/2, float64(b%4+1)/2
		xx, yy := x+float64(c%4+1)/2, y+float64(d%4+1)/2
		s := cells(t, box(0, 0, 0, x, y, 1), box(x, 0, 0, xx, y, 1), box(0, y, 0, x, yy, 1), box(x, y, 0, xx, yy, 1))
		var e error
		s, e = s.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, .25, 0}, {.25, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		r := check(t, s, 6)
		if len(r.Solid.Vertices()) != 8 {
			t.Fatal("subdivision vertices retained")
		}
	})
}
