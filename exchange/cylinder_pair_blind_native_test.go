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

func TestFiniteEccentricBlindRecipe(t *testing.T) {
	for _, sign := range []float64{-1, 1} {
		for _, lower := range []bool{false, true} {
			for _, equal := range []bool{false, true} {
				a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 2 * sign}, Radius: 5, Min: -2, Max: 2}
				lo, hi := 0., 6.
				if lower {
					lo, hi = -6, 0
				}
				if equal {
					if lower {
						lo = -4
					} else {
						hi = 4
					}
				}
				b := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -sign}, Radius: 2, Min: -hi, Max: -lo}
				r := CylinderPairCommonRecipe{Name: "eccentric blind", A: a, B: b, Tolerance: 1e-8, LengthUnit: .001}
				var buf bytes.Buffer
				if err := WriteCylinderPairDifferenceRecipeJSON(context.Background(), &buf, r); err != nil {
					t.Fatal(sign, lower, equal, err)
				}
				rr, d, err := ReadCylinderPairDifferenceRecipeJSON(context.Background(), bytes.NewReader(buf.Bytes()), 0)
				if err != nil {
					t.Fatal(err)
				}

				var env map[string]json.RawMessage
				if err = json.Unmarshal(buf.Bytes(), &env); err != nil {
					t.Fatal(err)
				}
				env["CutterCapFaces"] = json.RawMessage(`[[0],null]`)
				bad, _ := json.Marshal(env)
				if _, _, err = ReadCylinderPairDifferenceRecipeJSON(context.Background(), bytes.NewReader(bad), 0); err == nil {
					t.Fatal("floor ancestry tamper")
				}
				var native bytes.Buffer
				if err = parametric.WriteJSON(&native, d.Solids[0]); err != nil {
					t.Fatal(err)
				}
				var again bytes.Buffer
				if err = WriteCylinderPairDifferenceRecipeJSON(context.Background(), &again, rr); err != nil || !bytes.Equal(buf.Bytes(), again.Bytes()) {
					t.Fatal("canonical", err)
				}
				if len(d.Solids[0].Faces()) != 20 || len(d.Solids[1].Faces()) != 0 || len(d.CutterCapFaces[0]) != 4 {
					t.Fatal("ownership")
				}
				mass, err := d.Solids[0].MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil {
					t.Fatal(err)
				}
				// Stock volume 200pi; removed eccentric cylinder 16pi. Exact first
				// moments are (200-32,400-32,600-16*(3 +/- 2))/184.
				zc := 3 + sign*2
				if lower {
					zc = 3 - sign*2
				}
				want := geom.Vec3{X: 168. / 184, Y: 2, Z: (600 - 16*zc) / 184}
				if math.Abs(mass.Volume-184*math.Pi) > 1e-7 || math.Abs(mass.Area-146*math.Pi) > 1e-7 || mass.Centroid.Sub(want).Length() > 1e-8 {
					t.Fatal(mass, want)
				}
				for i, id := range d.StockCapFaces[0] {
					face := d.Solids[0].Faces()[id]
					p, e := face.Surface.Eval(face.U.Min, face.V.Min)
					if e != nil {
						t.Fatal(e)
					}
					z := 3 - 4*sign
					if i >= 4 {
						z = 3 + 4*sign
					}
					if math.Abs(p.Z-z) > 1e-12 {
						t.Fatal("cap order", p, z)
					}
				}
				for _, id := range d.CutterCapFaces[0] {
					face := d.Solids[0].Faces()[id]
					p, e := face.Surface.Eval(face.U.Min, face.V.Max)
					if e != nil || p.Z != 3 {
						t.Fatal("floor", p, e)
					}
				}
			}
		}
	}
}
