package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestGrooveTubeWithBothEndReliefs(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	r := HelicalGrooveRecipe{
		Frame: frame, OuterRadius: 4, BoreRadius: 2, Height: 5,
		Profile: []geom.Vec2{{X: 2, Y: 1}, {X: 2.5, Y: 1.125}, {X: 2, Y: 1.25}},
		Lead:    1, Start: .25, End: 2.25, Starts: 2,
	}
	lower, upper := HelicalReliefSpec{2.75, .5}, HelicalReliefSpec{3, .75}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		r.Hand = hand
		for _, lo := range []HelicalReliefSpec{{}, lower} {
			d, e := GrooveTubeWithEndReliefs(ctx, r, lo, upper)
			if e != nil {
				t.Fatal(hand, lo, e)
			}
			want := big.NewRat(15, 4)
			if lo.Depth > 0 {
				want.Add(want, big.NewRat(57, 32))
			}
			if d.RemovedVolumePiCoefficient.Cmp(want) != 0 || d.UpperFaces[0] < 0 || (d.LowerFaces[0] < 0) != (lo.Depth == 0) {
				t.Fatal("missing or wrong relief", d.UpperFaces, d.LowerFaces, d.RemovedVolumePiCoefficient, want)
			}
			original, modified := d.Original.Faces(), d.Solid.Faces()
			for i := 4; i < len(original); i++ {
				if !reflect.DeepEqual(original[i], modified[i]) {
					t.Fatal("changed helical source face", i)
				}
			}
			if !reflect.DeepEqual(original[1].Surface, modified[1].Surface) || !reflect.DeepEqual(original[1].Holes, modified[1].Holes) ||
				d.Solid.Topology().EulerCharacteristic != 0 || d.Solid.Topology().InnerWires != 2 {
				t.Fatal("changed inner grooves or topology", d.Solid.Topology())
			}
			a, e := d.Original.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil {
				t.Fatal(e)
			}
			b, e := d.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
			w, _ := want.Float64()
			if e != nil || math.Abs((a.Volume-b.Volume)-w*math.Pi) > 1e-6 {
				t.Fatal(a.Volume, b.Volume, w*math.Pi, e)
			}
			var data bytes.Buffer
			if e := WriteJSON(&data, d.Solid); e != nil {
				t.Fatal(e)
			}
			native, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil || native.Topology().Faces != d.Solid.Topology().Faces {
				t.Fatal(native.Topology(), e)
			}
		}
	}
	if d, e := GrooveTubeWithEndReliefs(ctx, r, lower, HelicalReliefSpec{3, 1}); !errors.Is(e, geom.ErrConvergence) || len(d.Solid.Faces()) != 0 {
		t.Fatal("touching groove end accepted", e)
	}
	if d, e := GrooveTubeWithEndReliefs(ctx, r, HelicalReliefSpec{3, 2}, HelicalReliefSpec{3, 3}); !errors.Is(e, geom.ErrDomain) || len(d.Solid.Faces()) != 0 {
		t.Fatal("touching reliefs accepted", e)
	}
}
