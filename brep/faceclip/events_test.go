package faceclip

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestSourceEdgeEventTraversalAndHoles(t *testing.T) {
	s := holed(t)
	for _, face := range []int{0, 1} {
		r, e := ClipRegionWithEvents(context.Background(), s, face, []Halfspace{{geom.Vec3{X: 1}, 2}}, RegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Events) != 4 || len(r.Edges) != 8 {
			t.Fatal(len(r.Events), len(r.Edges))
		}
		holeEvents := 0
		for _, event := range r.Events {
			if !event.Retained || event.Parameter.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal(event)
			}
			if event.Loop > 0 {
				holeEvents++
			}
			edge := s.Edges()[event.Edge]
			a, b := vec(s.Vertices()[edge.Start]), vec(s.Vertices()[edge.End])
			if event.Reversed {
				a, b = b, a
			}
			for k := range a {
				want := add(a[k], mul(event.Parameter, sub(b[k], a[k])))
				if event.Point[k].Cmp(want) != 0 {
					t.Fatal("wrong traversal parameter")
				}
			}
		}
		if holeEvents != 2 {
			t.Fatal("hole events")
		}
	}
}
func TestCoincidentEventAndCulling(t *testing.T) {
	s := holed(t)
	r, e := ClipRegionWithEvents(context.Background(), s, 1, []Halfspace{{geom.Vec3{X: 3}, 1}, {geom.Vec3{X: 6}, 2}, {geom.Vec3{Y: 1}, 0}}, RegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Events) != 2 {
		t.Fatal(r.Events)
	}
	retained, coincident := 0, 0
	for _, ev := range r.Events {
		if len(ev.Halfspaces) != 2 {
			t.Fatal(ev)
		}
		if ev.Point[0].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(ev.Point)
		}
		if ev.Retained {
			retained++
		}
	}
	for _, edge := range r.Edges {
		coincident += len(edge.CoincidentHalfspaces)
		if edge.From != nil && (edge.From.Sign() < 0 || edge.To.Cmp(big.NewRat(1, 1)) > 0) {
			t.Fatal("interval out of range")
		}
	}
	if retained != 1 || coincident != 1 {
		t.Fatal(retained, coincident)
	}
}
func TestEventFullEmptyAndLimits(t *testing.T) {
	s := holed(t)
	r, e := ClipRegionWithEvents(context.Background(), s, 1, nil, RegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, edge := range r.Edges {
		if edge.From.Sign() != 0 || edge.To.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(edge)
		}
	}
	r, e = ClipRegionWithEvents(context.Background(), s, 1, []Halfspace{{geom.Vec3{X: 1}, -1}}, RegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, edge := range r.Edges {
		if edge.From != nil {
			t.Fatal("retained outside interval")
		}
	}
	if _, e = ClipRegionWithEvents(context.Background(), s, 1, []Halfspace{{geom.Vec3{X: 1}, 2}}, RegionOptions{Clip: Options{MaxWork: 1}}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
