package fillet

import (
	"context"
	"math"
	"reflect"
	"testing"

	"bytes"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBoredRectangleBlindPocket(t *testing.T) {
	for _, cap := range []int{0, 1} {
		p := RectanglePocket{Center: geom.Vec2{X: 1.5, Y: 1.75}, Radius: .25, Depth: .75, EntryCap: cap}
		r, e := PrismRectangleCornersBoredPocket(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}, p, Options{})
		if e != nil {
			t.Fatalf("cap%d: %v", cap, e)
		}
		before := r.Bore.Solid
		if !reflect.DeepEqual(before.Vertices(), r.Solid.Vertices()[:len(before.Vertices())]) || !reflect.DeepEqual(before.Edges(), r.Solid.Edges()[:len(before.Edges())]) {
			t.Fatal("changed prior vertices/edges")
		}
		fs := r.Solid.Faces()
		for i, f := range before.Faces() {
			got := fs[i]
			if i == r.EntryFace {
				if len(got.Holes) != len(f.Holes)+1 {
					t.Fatal("hole count")
				}
				got.Holes = got.Holes[:len(f.Holes)]
			}
			if !reflect.DeepEqual(f, got) {
				t.Fatalf("changed face %d", i)
			}
		}
		if len(fs) != len(before.Faces())+5 || len(r.Solid.Vertices()) != len(before.Vertices())+8 || len(r.Solid.Edges()) != len(before.Edges())+12 {
			t.Fatal("ancestry")
		}
		props, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cross := 24 - (1-math.Pi/4)*(.25*.25+.5*.5) - math.Pi*.375*.375
		perimeter := 20 - 2*(.25+.5) + math.Pi/2*(.25+.5) + 2*math.Pi*.375
		volume := 2*cross - math.Pi*.25*.25*.75
		area := 2*cross + 2*perimeter + 2*math.Pi*.25*.75
		if math.Abs(props.Volume-volume) > 1e-7 || math.Abs(props.Area-area) > 1e-7 || math.Abs(r.NominalVolume-volume) > 1e-12 || math.Abs(r.NominalSurfaceArea-area) > 1e-12 {
			t.Fatalf("mass %+v expected %g %g", props, volume, area)
		}
		// Each holed cap contributes 1-holes to the boundary Euler sum.
		chi := len(r.Solid.Vertices()) - len(r.Solid.Edges())
		for _, f := range r.Solid.Faces() {
			chi += 1 - len(f.Holes)
		}
		if chi != 0 {
			t.Fatal("blind pocket changed genus")
		}
		var encoded bytes.Buffer
		if e := trimmed.WriteJSON(&encoded, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := trimmed.ReadJSON(&encoded, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-volume) > 1e-7 {
			t.Fatalf("roundtrip mass %v %+v", e, mass)
		}

	}
}
func TestBoredRectangleBlindPocketRejects(t *testing.T) {
	good := RectanglePocket{Center: geom.Vec2{X: 1.5, Y: 2}, Radius: .25, Depth: .75}
	bad := []RectanglePocket{good, good, good, good, good, good}
	bad[0].Depth = 2
	bad[1].Depth = 0
	bad[2].Center.X = 3
	bad[3].Center.X = 2.375
	bad[4].Center.X = 1.25
	bad[5].EntryCap = 2
	for _, p := range bad {
		r, e := PrismRectangleCornersBoredPocket(context.Background(), geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .5}}, p, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatalf("accepted %+v", p)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PrismRectangleCornersBoredPocket(ctx, geom.Vec2{}, geom.Vec2{X: 6, Y: 4}, -1, 1, .375, []CornerRadius{{Corner: 0, Radius: .5}}, good, Options{}); e == nil {
		t.Fatal("cancel")
	}
}
