package exchange

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCylinderPairSteppedCoaxialUnionRecipe(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	for _, tc := range []struct {
		name string
		tool geom.RawCylinder
		coef string
	}{
		{"overlap", geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 8}, Axis: geom.Vec3{Z: 3}, Radius: 2, Min: -1, Max: 1}, "8"},
		{"end contact", geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 7}, Axis: geom.Vec3{Z: 2}, Radius: 2, Min: 0, Max: 2}, "0"},
	} {
		for large, p := range [][2]geom.RawCylinder{{a, tc.tool}, {tc.tool, a}} {
			r := CylinderPairCommonRecipe{Name: tc.name, A: p[0], B: p[1], Tolerance: 1e-9, LengthUnit: .001}
			var data, again bytes.Buffer
			if err := WriteCylinderPairUnionRecipeJSON(ctx, &data, r); err != nil {
				t.Fatal(tc.name, large, err)
			}
			got, out, err := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(data.Bytes()), 0)
			if err != nil || got != r || len(out.Solid.Faces()) != 5 {
				t.Fatal(tc.name, large, err)
			}
			if tc.coef == "0" {
				if out.CommonVolumeSquared == nil || out.CommonVolumeSquared.Sign() != 0 || out.CommonVolumePiCoefficient != nil {
					t.Fatal("zero-volume axial contact proof")
				}
			} else if out.CommonVolumeSquared != nil || out.CommonVolumePiCoefficient == nil || out.CommonVolumePiCoefficient.RatString() != tc.coef {
				t.Fatal("exact overlap proof")
			}
			s := out.Solid
			for i, group := range [][]int{out.LowerCapFaces[large], out.UpperCapFaces[large], out.UpperCapFaces[1-large]} {
				if len(group) != 1 {
					t.Fatal("exposed cap ancestry", tc.name, large, i, group)
				}
				f := s.Faces()[group[0]]
				p, err := f.Surface.Eval(0, .5)
				if err != nil || p.Z != [3]float64{7, -1, 11}[i] {
					t.Fatal("original cap support", i, p, err)
				}
				if i == 0 && p.X != 4.5 {
					t.Fatal("step cap must be annular", p)
				}
			}
			if len(out.WallFaces[0]) != 1 || len(out.WallFaces[1]) != 1 || len(out.LowerCapFaces[1-large]) != 0 {
				t.Fatal("lost wall or retained hidden cap")
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(m.Volume-216*math.Pi) > 1e-8 || math.Abs(m.Area-146*math.Pi) > 1e-8 || math.Abs(m.Centroid.Z-744.0/216) > 1e-9 {
				t.Fatal("exact slab measures", m, err)
			}
			mesh, err := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, AnalyticBound: true})
			if err != nil || mesh.ValidateSolid() != nil {
				t.Fatal("closed certified display", err)
			}
			if err = WriteCylinderPairUnionRecipeJSON(ctx, &again, got); err != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
				t.Fatal("canonical bytes changed", err)
			}
		}
	}
}
