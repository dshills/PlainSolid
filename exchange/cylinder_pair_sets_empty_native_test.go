package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderPairDifferenceEmptySlotsNative(t *testing.T) {
	ctx := context.Background()
	cases := []CylinderPairCommonRecipe{
		{Name: "untouched stock", A: geom.RawCylinder{Axis: geom.Vec3{Z: -2}, Radius: 2, Min: -1, Max: 1}, B: geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: 3}, Radius: 1, Min: -1, Max: 1}, Tolerance: 1e-9, LengthUnit: .001},
		{Name: "covered stock", A: geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2}, B: geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5}, Tolerance: 1e-9, LengthUnit: .001},
	}
	for i, r := range cases {
		t.Run(r.Name, func(t *testing.T) {
			var b bytes.Buffer
			if e := WriteCylinderPairDifferenceRecipeJSON(ctx, &b, r); e != nil {
				t.Fatal(e)
			}
			got, d, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(b.Bytes()), 0)
			if e != nil || got != r || d.ContactNodes != nil || d.MinimumGapSquared != nil {
				t.Fatal(got, d, e)
			}
			want := [2]int{3, 0}
			if i == 1 {
				want = [2]int{0, 0}
			}
			var env cylinderSetEnvelope
			if e = json.Unmarshal(b.Bytes(), &env); e != nil || len(env.Boundaries) != 2 {
				t.Fatal("slot count", e)
			}
			for slot, s := range d.Solids {
				if len(s.Faces()) != want[slot] {
					t.Fatal("slot geometry", slot)
				}
				seen := map[int]bool{}
				for _, group := range [][]int{d.StockWallFaces[slot], d.CutterWallFaces[slot], d.StockCapFaces[slot]} {
					for _, id := range group {
						if id < 0 || id >= len(s.Faces()) || seen[id] {
							t.Fatal("ancestry")
						}
						seen[id] = true
					}
				}
				if len(seen) != len(s.Faces()) {
					t.Fatal("missing ancestry")
				}
				var native bytes.Buffer
				if e = parametric.WriteJSON(&native, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
				if e != nil || len(loaded.Faces()) != want[slot] {
					t.Fatal("native slot changed", e)
				}
				var again bytes.Buffer
				if e = parametric.WriteJSON(&again, loaded); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
					t.Fatal("native canonical change", e)
				}
			}
			if i == 0 {
				if len(d.StockWallFaces[0]) != 1 || len(d.StockCapFaces[0]) != 2 || len(d.CutterWallFaces[0]) != 0 {
					t.Fatal("untouched roles")
				}
				m, e := d.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
				if e != nil || math.Abs(m.Volume-16*math.Pi) > 1e-9 {
					t.Fatal(m, e)
				}
				env.Boundaries[0], env.Boundaries[1] = env.Boundaries[1], env.Boundaries[0]
				bad, _ := json.Marshal(env)
				if _, out, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil || len(out.Solids[0].Faces()) != 0 {
					t.Fatal("swapped slot accepted")
				}
			}
			var again bytes.Buffer
			if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatal("regeneration", e)
			}
			if e = json.Unmarshal(b.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			env.MinimumGapSquared = "1"
			bad, _ := json.Marshal(env)
			if _, _, e = ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil {
				t.Fatal("invented gap")
			}
			if e = json.Unmarshal(b.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			env.Boundaries = env.Boundaries[:1]
			bad, _ = json.Marshal(env)
			if _, _, e = ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil {
				t.Fatal("empty slot dropped")
			}
		})
	}
}
