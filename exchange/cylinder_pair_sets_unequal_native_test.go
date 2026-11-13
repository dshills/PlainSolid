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

func TestCylinderPairUnequalCrossingUnionRecipe(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	a := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -2, Max: 2}
	for order, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		t.Run([]string{"AB", "BA"}[order], func(t *testing.T) {
			r := CylinderPairCommonRecipe{Name: "unequal perpendicular union", A: pair[0], B: pair[1], Tolerance: 1e-8, LengthUnit: .001}
			var data bytes.Buffer
			if e := WriteCylinderPairUnionRecipeJSON(ctx, &data, r); e != nil {
				t.Fatal(e)
			}
			got, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(data.Bytes()), 0)
			if e != nil || got != r || u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient != nil {
				t.Fatal(got, e)
			}
			faces := u.Solid.Faces()
			seen := map[int]bool{}
			for owner, c := range pair {
				for role, ids := range [][]int{u.WallFaces[owner], u.LowerCapFaces[owner], u.UpperCapFaces[owner]} {
					if len(ids) == 0 {
						t.Fatal("missing owner role", owner, role)
					}
					for _, id := range ids {
						if id < 0 || id >= len(faces) || seen[id] {
							t.Fatal("invalid ancestry")
						}
						seen[id] = true
						f := faces[id]
						p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
						if e != nil {
							t.Fatal(e)
						}
						d := p.Sub(c.Origin)
						if role > 0 {
							want := c.Min
							if role == 2 {
								want = c.Max
							}
							if d.Dot(c.Axis) != want*c.Axis.Dot(c.Axis) {
								t.Fatal("signed cap ownership", owner, role, p)
							}
						} else {
							radial := d.Dot(d) - d.Dot(c.Axis)*d.Dot(c.Axis)/c.Axis.Dot(c.Axis)
							if math.Abs(radial-c.Radius*c.Radius) > 1e-10 {
								t.Fatal("wrong owner radius", owner, radial)
							}
						}
					}
				}
			}
			if len(seen) != len(faces) {
				t.Fatal("unowned native face")
			}
			var native bytes.Buffer
			if e = parametric.WriteJSON(&native, u.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			var rewritten bytes.Buffer
			if e = parametric.WriteJSON(&rewritten, loaded); e != nil || !bytes.Equal(native.Bytes(), rewritten.Bytes()) {
				t.Fatal("chart reload changed", e)
			}
			var again bytes.Buffer
			if e = WriteCylinderPairUnionRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(data.Bytes(), again.Bytes()) {
				t.Fatal("recipe changed", e)
			}
			for _, field := range []string{"CommonVolumeSquared", "CommonVolumePiCoefficient"} {
				var env cylinderSetEnvelope
				if e = json.Unmarshal(data.Bytes(), &env); e != nil {
					t.Fatal(e)
				}
				if env.CommonVolumeSquared != "" || env.CommonVolumePiCoefficient != "" {
					t.Fatal("unexpected rational proof")
				}
				if field == "CommonVolumeSquared" {
					env.CommonVolumeSquared = "0"
				} else {
					env.CommonVolumePiCoefficient = "1"
				}
				bad, e := json.Marshal(env)
				if e != nil {
					t.Fatal(e)
				}
				if _, out, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(bad), 0); e == nil || len(out.Solid.Faces()) != 0 {
					t.Fatal("fake proof accepted", field, e)
				}
			}
		})
	}
}
