package faceclip

import (
	"context"
	"errors"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func TestMergedHoleAndOpenedHole(t *testing.T) {
	s := holed(t)
	for _, x := range []float64{4, 2} {
		r, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{X: 1}, x}, Options{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := MergeLoops(context.Background(), r, LoopOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 1
		if x == 4 {
			want = 2
		}
		if len(b.Loops) != want {
			t.Fatal(len(b.Loops))
		}
		holes := 0
		for _, l := range b.Loops {
			if l.Hole {
				holes++
				if l.Parent < 0 || b.Loops[l.Parent].Hole {
					t.Fatal("parent")
				}
			}
		}
		if holes != want-1 {
			t.Fatal(holes)
		}
		if b.ProjectedArea.Cmp(r.ProjectedArea) != 0 {
			t.Fatal("area")
		}
	}
}
func TestMergedDisconnectedIslands(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 3}, {X: 3, Y: 3}, {X: 2, Y: 3}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 3}, {Y: 3}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Clip(context.Background(), s, 1, Halfspace{geom.Vec3{Y: -1}, -2}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Loops) != 2 {
		t.Fatal(len(b.Loops))
	}
	for _, l := range b.Loops {
		if l.Hole || l.Parent != -1 {
			t.Fatal("island")
		}
	}
}
func TestMergedLimitsAndEmpty(t *testing.T) {
	r, e := Clip(context.Background(), holed(t), 1, Halfspace{geom.Vec3{X: 1}, 4}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []LoopOptions{{MaxWork: 1}, {MaxVertices: 1}, {MaxBits: 1}} {
		if _, e := MergeLoops(context.Background(), r, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	r.Fragments = append(r.Fragments, r.Fragments[0])
	if _, e := MergeLoops(context.Background(), r, LoopOptions{}); !errors.Is(e, ErrNonmanifold) {
		t.Fatal(e)
	}
	r, e = Clip(context.Background(), holed(t), 1, Halfspace{geom.Vec3{X: 1}, -1}, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil || len(b.Loops) != 0 {
		t.Fatal(e)
	}
}
