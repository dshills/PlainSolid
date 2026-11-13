package draft

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestRemoveChamferSupport(t *testing.T) {
	s := cube(t)
	cut, e := Chamfer(context.Background(), s, []EdgeChamfer{{cornerEdge(t, s), 3, .25, .5}}, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	face := cut.EdgeFaces[0]
	r, e := RemoveSupport(context.Background(), cut.Solid, face, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 1, 1)
	m, e := r.Solid.MassProperties()
	if e != nil || math.Abs(m.SurfaceArea-6) > 1e-12 {
		t.Fatal(e, m)
	}
	if r.OldToNew[face] != -1 || len(r.Solid.Faces()) != 6 {
		t.Fatal("history")
	}
	for _, src := range r.FaceSources {
		for _, id := range src {
			if id == face {
				t.Fatal("removed source survives")
			}
		}
	}
}
func TestRemoveSupportReject(t *testing.T) {
	s := cube(t)
	_, e := RemoveSupport(context.Background(), s, 1, Options{})
	if !errors.Is(e, ErrUnbounded) {
		t.Fatal(e)
	}
	if _, e = RemoveSupport(context.Background(), s, -1, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e = RemoveSupport(context.Background(), s, 0, Options{MaxWork: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func TestRemoveMultipleChamfers(t *testing.T) {
	s := cube(t)
	vs := s.Vertices()
	var cuts []EdgeChamfer
	for i, edge := range s.Edges() {
		a, b := vs[edge.Start], vs[edge.End]
		if a.X == b.X && a.Y == b.Y {
			face := -1
			for j, f := range s.Faces() {
				for _, u := range f.Boundary {
					if u.Edge == i {
						face = j
					}
				}
			}
			cuts = append(cuts, EdgeChamfer{i, face, .125, .125})
		}
	}
	ch, e := Chamfer(context.Background(), s, cuts, false, Options{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := RemoveSupports(context.Background(), ch.Solid, ch.EdgeFaces, Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 1, 1)
	if len(r.Solid.Faces()) != 6 {
		t.Fatal("cube not restored")
	}
	for _, i := range ch.EdgeFaces {
		if r.OldToNew[i] != -1 {
			t.Fatal("selected ancestry")
		}
	}
	r, e = RemoveSupports(context.Background(), ch.Solid, ch.EdgeFaces[:2], Options{})
	if e != nil {
		t.Fatal(e)
	}
	checkVolume(t, r, 63, 64)
	if _, e = RemoveSupports(context.Background(), s, []int{0, 1}, Options{}); !errors.Is(e, ErrUnbounded) {
		t.Fatal(e)
	}
	if _, e = RemoveSupports(context.Background(), s, []int{0, 0}, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
