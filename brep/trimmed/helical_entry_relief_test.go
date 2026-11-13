package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestBlindHelicalEntryRelief(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	recipe := HelicalGrooveRecipe{Frame: f, OuterRadius: 4, BoreRadius: 2, Height: 5, Profile: []geom.Vec2{{X: 2, Y: 1}, {X: 2.5, Y: 1.125}, {X: 2, Y: 1.25}}, Lead: 1, Start: .25, End: 2.25, Hand: geom.RightHanded, Starts: 1}
	r, e := GrooveTubeWithEntryRelief(context.Background(), recipe, 2.75, .5)
	if e != nil {
		t.Fatal(e)
	}
	for i, face := range r.Original.Faces() {
		if i != 1 && i != 2 && !reflect.DeepEqual(face, r.Solid.Faces()[i]) {
			t.Fatal("changed groove face", i)
		}
	}
	if !reflect.DeepEqual(r.Original.Faces()[1].Surface, r.Solid.Faces()[1].Surface) || !reflect.DeepEqual(r.Original.Faces()[1].Holes, r.Solid.Faces()[1].Holes) {
		t.Fatal("lost groove chart")
	}
	old, e := r.Original.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	now, e := r.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := math.Pi * (2.75*2.75 - 4) * .5
	if math.Abs(old.Volume-now.Volume-want) > 1e-6 {
		t.Fatal("differential volume", old.Volume, now.Volume, want)
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, r.Solid); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadJSON(&data, JSONLimits{}); e != nil {
		t.Fatal(e)
	}
	if _, e := GrooveTubeWithEntryRelief(context.Background(), recipe, 2.75, 1.25); e == nil {
		t.Fatal("touching groove accepted")
	}
}

func TestHelicalEntryReliefHandsAndLimits(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, hand := range []geom.HelixHandedness{geom.LeftHanded, geom.RightHanded} {
		r := HelicalGrooveRecipe{Frame: f, OuterRadius: 4, BoreRadius: 2, Height: 5, Profile: []geom.Vec2{{X: 2, Y: 1}, {X: 2.5, Y: 1.125}, {X: 2, Y: 1.25}}, Lead: 1, Start: .25, End: 2.25, Hand: hand, Starts: 2}
		if _, e := GrooveTubeWithHelicalStarts(context.Background(), r.Frame, r.OuterRadius, r.BoreRadius, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts); e != nil {
			t.Fatalf("original hand %v: %v", hand, e)
		}
		out, e := GrooveTubeWithEntryRelief(context.Background(), r, 2.75, .5)
		if e != nil {
			t.Fatal(hand, e)
		}
		if len(out.Solid.Faces()[1].Holes) != r.Starts {
			t.Fatal("lost starts")
		}
		for _, pair := range [][2]float64{{2, .5}, {4, .5}, {2.75, 0}, {2.75, 1.25}} {
			bad, e := GrooveTubeWithEntryRelief(context.Background(), r, pair[0], pair[1])
			if e == nil || len(bad.Solid.Faces()) != 0 {
				t.Fatal("partial/invalid", pair)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := GrooveTubeWithEntryRelief(ctx, r, 2.75, .5); e == nil {
			t.Fatal("cancel")
		}
	}
}
