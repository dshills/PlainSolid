package exchange

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCylinderPairCoaxialUnionRecipe(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 2, Min: -1, Max: 2} // z=5→-1
	for _, tc := range []struct {
		name string
		min  float64
		coef string
	}{{"overlap", -1, "12"}, {"end contact", 0, "0"}} {
		t.Run(tc.name, func(t *testing.T) {
			b := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 5}, Axis: geom.Vec3{Z: 3}, Radius: 2, Min: tc.min, Max: 2}
			for order, p := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
				r := CylinderPairCommonRecipe{Name: tc.name, A: p[0], B: p[1], LengthUnit: .001, Tolerance: 1e-9}
				var data, again bytes.Buffer
				if err := WriteCylinderPairUnionRecipeJSON(ctx, &data, r); err != nil {
					t.Fatal(err)
				}
				got, out, err := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(data.Bytes()), 0)
				if err != nil || got != r || len(out.Solid.Faces()) != 3 || len(out.WallFaces[0]) != 1 || len(out.WallFaces[1]) != 1 || out.WallFaces[0][0] != out.WallFaces[1][0] {
					t.Fatal("retained union and both wall owners", err)
				}
				if tc.coef == "0" {
					if out.CommonVolumeSquared == nil || out.CommonVolumeSquared.Sign() != 0 || out.CommonVolumePiCoefficient != nil {
						t.Fatal("end contact has zero common volume")
					}
				} else if out.CommonVolumePiCoefficient == nil || out.CommonVolumePiCoefficient.RatString() != tc.coef || out.CommonVolumeSquared != nil {
					t.Fatal("coaxial overlap proof")
				}
				for i := range p {
					if len(out.LowerCapFaces[i]) != 0 || len(out.UpperCapFaces[i]) != 1 {
						t.Fatal("only original maximum caps should survive", order, i)
					}
					point, err := out.Solid.Faces()[out.UpperCapFaces[i][0]].Surface.Eval(0, 0)
					if err != nil || point != p[i].Origin.Add(p[i].Axis.Scale(p[i].Max)) {
						t.Fatal("lost exposed original cap", order, i, point, err)
					}
				}
				mass, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
				if err != nil || math.Abs(mass.Volume-48*math.Pi) > 1e-8 || math.Abs(mass.Area-56*math.Pi) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: 1, Y: 2, Z: 5}).Length() > 1e-9 {
					t.Fatal(mass, err)
				}
				if err = WriteCylinderPairUnionRecipeJSON(ctx, &again, got); err != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
					t.Fatal("canonical bytes changed", err)
				}
			}
		})
	}
}
