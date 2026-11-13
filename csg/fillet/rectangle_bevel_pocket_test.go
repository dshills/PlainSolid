package fillet

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestBeveledBoredRectanglePocket(t *testing.T) {
	for _, cap := range []int{0, 1} {
		p := RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75, EntryCap: cap}
		r, e := PrismRectangleTopBeveledBoredPocket(context.Background(), geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, p, Options{})
		if e != nil {
			t.Fatal(e)
		}
		old := r.Stock.Solid
		if !reflect.DeepEqual(old.Vertices(), r.Solid.Vertices()[:len(old.Vertices())]) || !reflect.DeepEqual(old.Edges(), r.Solid.Edges()[:len(old.Edges())]) {
			t.Fatal("changed prior topology")
		}
		fs := r.Solid.Faces()
		for i, f := range old.Faces() {
			got := fs[i]
			if i == r.EntryFace {
				if len(got.Holes) != 2 {
					t.Fatal("entry holes")
				}
				got.Holes = got.Holes[:1]
			}
			if !reflect.DeepEqual(f, got) {
				t.Fatal("changed prior face", i)
			}
		}
		if len(fs) != len(old.Faces())+5 {
			t.Fatal("generated faces")
		}
		mass, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		before, e := old.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(before.Volume-mass.Volume-math.Pi*.25*.25*.75) > 1e-7 || math.Abs(mass.Area-before.Area-2*math.Pi*.25*.75) > 1e-7 || math.Abs(mass.Volume-r.NominalVolume) > 1e-7 || math.Abs(mass.Area-r.NominalSurfaceArea) > 1e-7 {
			t.Fatal("differential mass", before, mass)
		}
		chi := len(r.Solid.Vertices()) - len(r.Solid.Edges())
		for _, f := range fs {
			chi += 1 - len(f.Holes)
		}
		if chi != 0 {
			t.Fatal("genus")
		}
		var data bytes.Buffer
		if e := trimmed.WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := trimmed.ReadJSON(&data, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		check, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(check.Volume-mass.Volume) > 1e-7 {
			t.Fatal("reload", e)
		}

	}
}
func TestBeveledBoredRectanglePocketInvalid(t *testing.T) {
	good := RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75}
	for i := 0; i < 4; i++ {
		p := good
		switch i {
		case 0:
			p.Depth = 3
		case 1:
			p.Center = geom.Vec2{X: 4, Y: 2.5}
		case 2:
			p.Center.X = 1.75
		case 3:
			p.EntryCap = 2
		}
		r, e := PrismRectangleTopBeveledBoredPocket(context.Background(), geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, p, Options{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted", p)
		}
	}
}

func TestBeveledBoredRectanglePocketCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := PrismRectangleTopBeveledBoredPocket(ctx, geom.Vec2{}, geom.Vec2{X: 8, Y: 5}, -1, 2, .75, .25, .5, .375, RectanglePocket{Center: geom.Vec2{X: 2, Y: 2}, Radius: .25, Depth: .75}, Options{})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("cancel")
	}
}
