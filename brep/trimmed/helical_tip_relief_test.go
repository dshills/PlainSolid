package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestExternalHelicalTipRelief(t *testing.T) {
	slowtest.Skip(t)
	f, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, hand := range []geom.HelixHandedness{geom.LeftHanded, geom.RightHanded} {
		recipe := HelicalGrooveRecipe{Frame: f, OuterRadius: 4, Height: 5, Profile: []geom.Vec2{{X: 3.5, Y: 1.125}, {X: 4, Y: 1}, {X: 4, Y: 1.25}}, Lead: 1, Start: .25, End: 2.25, Hand: hand, Starts: 2}
		r, e := GrooveShaftWithTipRelief(context.Background(), recipe, 3, .5)
		if e != nil {
			t.Fatal(hand, e)
		}
		for i, face := range r.Original.Faces() {
			if i != 0 && i != 1 && !reflect.DeepEqual(face, r.Solid.Faces()[i]) {
				t.Fatal("changed groove face", i)
			}
		}
		if !reflect.DeepEqual(r.Original.Faces()[0].Surface, r.Solid.Faces()[0].Surface) || !reflect.DeepEqual(r.Original.Faces()[0].Holes, r.Solid.Faces()[0].Holes) {
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
		want := math.Pi * 7 * .5
		if math.Abs(old.Volume-now.Volume-want) > 1e-6 {
			t.Fatal("removed mass", old.Volume, now.Volume, want)
		}
		coeff, _ := r.RemovedVolumePiCoefficient.Float64()
		if coeff != 3.5 {
			t.Fatal("exact coefficient")
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadJSON(&data, JSONLimits{}); e != nil {
			t.Fatal(e)
		}
		for _, pair := range [][2]float64{{0, .5}, {4, .5}, {3, 0}, {3, 1.25}} {
			bad, e := GrooveShaftWithTipRelief(context.Background(), recipe, pair[0], pair[1])
			if e == nil || len(bad.Solid.Faces()) != 0 {
				t.Fatal("accepted", pair)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := GrooveShaftWithTipRelief(ctx, recipe, 3, .5); e == nil {
			t.Fatal("cancel")
		}
	}
}
