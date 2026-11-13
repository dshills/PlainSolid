package draft

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func cornerEdge(t *testing.T, s brep.Solid) int {
	t.Helper()
	v := s.Vertices()
	for i, e := range s.Edges() {
		if v[e.Start].X == 1 && v[e.End].X == 1 && v[e.Start].Y == 1 && v[e.End].Y == 1 {
			return i
		}
	}
	t.Fatal("edge missing")
	return -1
}
func TestTwoDistanceChamfer(t *testing.T) {
	s := cube(t)
	id := cornerEdge(t, s)
	for _, coordinate := range []bool{false, true} {
		r, e := Chamfer(context.Background(), s, []EdgeChamfer{{id, 3, .25, .5}}, coordinate, Options{})
		if e != nil {
			t.Fatal(e)
		}
		checkVolume(t, r.Result, 15, 16)
		m, e := r.Solid.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		wantArea := 6 - .25 - .5 - 2*(.25*.5/2) + math.Hypot(.25, .5)
		if math.Abs(m.SurfaceArea-wantArea) > 1e-12 {
			t.Fatalf("area %g want %g", m.SurfaceArea, wantArea)
		}
		if len(r.Solid.Faces()) != 7 || r.EdgeFaces[0] < 0 || len(r.FaceEdges[r.EdgeFaces[0]]) != 1 || r.FaceEdges[r.EdgeFaces[0]][0] != id {
			t.Fatal("ancestry", r)
		}
		if len(r.FaceSources[r.EdgeFaces[0]]) != 0 {
			t.Fatal("bevel falsely attributed to source face")
		}
	}
}
func TestChamferFaceOrder(t *testing.T) {
	s := cube(t)
	id := cornerEdge(t, s)
	a, e := Chamfer(context.Background(), s, []EdgeChamfer{{id, 3, .25, .5}}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Chamfer(context.Background(), s, []EdgeChamfer{{id, 4, .5, .25}}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if a.ExactVolume.Cmp(b.ExactVolume) != 0 {
		t.Fatal("swapped distances differ")
	}
	for i, p := range a.ExactVertices {
		if key(p) != key(b.ExactVertices[i]) {
			t.Fatal("swapped vertices differ")
		}
	}
}
func TestChamferFailures(t *testing.T) {
	s := cube(t)
	id := cornerEdge(t, s)
	valid := EdgeChamfer{id, 3, .25, .5}
	for _, cs := range [][]EdgeChamfer{{{id, 0, .25, .5}}, {{id, 3, 0, .5}}, {valid, valid}, {{-1, 3, .25, .5}}} {
		if _, e := Chamfer(context.Background(), s, cs, false, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	if _, e := Chamfer(context.Background(), s, []EdgeChamfer{valid}, false, Options{MaxPlanes: 6}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func TestIrrationalPerpendicularAndCoordinateMode(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	edge := -1
	v := s.Vertices()
	for i, x := range s.Edges() {
		if v[x.Start].X == 0 && v[x.End].X == 0 {
			edge = i
			break
		}
	}
	if edge < 0 {
		t.Fatal("missing")
	}
	face := -1
	for i, f := range s.Faces() {
		for _, u := range f.Boundary {
			if u.Edge == edge {
				face = i
			}
		}
	}
	cs := []EdgeChamfer{{edge, face, .125, .25}}
	if _, e := Chamfer(context.Background(), s, cs, false, Options{}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	r, e := Chamfer(context.Background(), s, cs, true, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactVolume.Sign() <= 0 {
		t.Fatal("empty")
	}
}
func TestMultipleChamfersAndEmpty(t *testing.T) {
	s := cube(t)
	v := s.Vertices()
	var cs []EdgeChamfer
	for i, e := range s.Edges() {
		a, b := v[e.Start], v[e.End]
		if a.X == b.X && a.Y == b.Y {
			face := -1
			for j, f := range s.Faces() {
				for _, u := range f.Boundary {
					if u.Edge == i {
						face = j
					}
				}
			}
			cs = append(cs, EdgeChamfer{i, face, .125, .125})
		}
	}
	r, e := Chamfer(context.Background(), s, cs, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r.Result, 31, 32)
	if len(r.Solid.Faces()) != 10 {
		t.Fatal("faces", len(r.Solid.Faces()))
	}
	r, e = Chamfer(context.Background(), s, []EdgeChamfer{{cornerEdge(t, s), 3, 4, 4}}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r.Result, 0, 1)
	if r.EdgeFaces[0] != -1 {
		t.Fatal("empty bevel ancestry")
	}
}
