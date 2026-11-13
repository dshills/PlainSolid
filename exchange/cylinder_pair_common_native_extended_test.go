package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestCylinderPairCommonExtendedNative(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		a, b geom.RawCylinder
		kind parametric.CylinderCommonConstruction
	}{
		{"signed-slab", geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: -1}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: 0, Max: 2.5}, geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 6, Z: -3}, Axis: geom.Vec3{Y: 4}, Radius: 5, Min: -2.5, Max: .5}, parametric.CylinderCommonPerpendicularSlab},
		{"dual-clipped", geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 5, Min: -6, Max: -2}, geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -6, Max: 3}, parametric.CylinderCommonPerpendicularDualCap},
		{"four-cap", geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 5, Min: -1, Max: 2}, geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -3, Max: 4}, parametric.CylinderCommonPerpendicularCapWindows},
		{"unequal", geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: 3, Min: -2, Max: 4.5}, geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: -2, Max: 1}, parametric.CylinderCommonPerpendicularCapWindows},
		{"skew-contained", geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2}, geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5}, parametric.CylinderCommonContainedCylinder},
	}
	for _, tc := range cases {
		for order, pair := range [][2]geom.RawCylinder{{tc.a, tc.b}, {tc.b, tc.a}} {
			t.Run(tc.name+[]string{"/AB", "/BA"}[order], func(t *testing.T) {
				r := CylinderPairCommonRecipe{Name: tc.name, A: pair[0], B: pair[1], Tolerance: 1e-8, LengthUnit: .001}
				var encoded bytes.Buffer
				if e := WriteCylinderPairCommonRecipeJSON(ctx, &encoded, r); e != nil {
					t.Fatal(e)
				}
				got, s, e := ReadCylinderPairCommonRecipeJSON(ctx, bytes.NewReader(encoded.Bytes()), 0)
				if e != nil || !reflect.DeepEqual(got, r) || s.Construction != tc.kind {
					t.Fatal(got, s.Construction, e)
				}
				var env cylinderPairCommonEnvelope
				if e = json.Unmarshal(encoded.Bytes(), &env); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(env.SideFaces, s.SideFaces) || !reflect.DeepEqual(env.LowerCapFaces, s.LowerCapFaces) || !reflect.DeepEqual(env.UpperCapFaces, s.UpperCapFaces) {
					t.Fatal("ancestry changed")
				}
				owned := map[int]bool{}
				faces := s.Solid.Faces()
				for owner := 0; owner < 2; owner++ {
					for role, ids := range [][]int{s.SideFaces[owner], s.LowerCapFaces[owner], s.UpperCapFaces[owner]} {
						for _, id := range ids {
							if id < 0 || id >= len(faces) || owned[id] {
								t.Fatal("ownership", id)
							}
							owned[id] = true
							if role > 0 {
								face := faces[id]
								p, e := face.Surface.Eval((face.U.Min+face.U.Max)/2, (face.V.Min+face.V.Max)/2)
								if e != nil {
									t.Fatal(e)
								}
								raw := pair[owner]
								parameter := p.Sub(raw.Origin).Dot(raw.Axis) / raw.Axis.Dot(raw.Axis)
								want := raw.Min
								if role == 2 {
									want = raw.Max
								}
								if parameter != want {
									t.Fatal("raw signed cap ancestry", owner, role, parameter, want)
								}
							}
						}
					}
				}
				if len(owned) != len(faces) {
					t.Fatal("unowned faces")
				}
				if tc.name == "four-cap" || tc.name == "unequal" {
					for owner := 0; owner < 2; owner++ {
						if len(s.LowerCapFaces[owner]) == 0 || len(s.UpperCapFaces[owner]) == 0 {
							t.Fatal("lost finite cap")
						}
					}
				}
				if tc.name == "skew-contained" {
					owner := order
					if len(s.SideFaces[owner]) != 1 || len(s.SideFaces[1-owner]) != 0 {
						t.Fatal("contained owner lost")
					}
				}
				var native bytes.Buffer
				if e = parametric.WriteJSON(&native, s.Solid); e != nil {
					t.Fatal(e)
				}
				loaded, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				var restored bytes.Buffer
				if e = parametric.WriteJSON(&restored, loaded); e != nil || !bytes.Equal(native.Bytes(), restored.Bytes()) {
					t.Fatal("chart canonical reload changed", e)
				}
				var again bytes.Buffer
				if e = WriteCylinderPairCommonRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(encoded.Bytes(), again.Bytes()) {
					t.Fatal("recipe regeneration changed", e)
				}
				// Change a retained face role to another valid index: indices alone are not ancestry proof.
				for owner := 0; owner < 2; owner++ {
					if len(env.SideFaces[owner]) > 0 {
						env.SideFaces[owner][0] = (env.SideFaces[owner][0] + 1) % len(faces)
						break
					}
				}
				bad, e := json.Marshal(env)
				if e != nil {
					t.Fatal(e)
				}
				if _, _, e = ReadCylinderPairCommonRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil {
					t.Fatal("stale ancestry accepted")
				}
			})
		}
	}
}
