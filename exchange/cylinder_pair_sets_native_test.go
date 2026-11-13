package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func cylinderSetFixture() CylinderPairCommonRecipe {
	c := geom.Vec3{X: 4, Y: -3, Z: 2}
	return CylinderPairCommonRecipe{Name: "signed raw sets", A: geom.RawCylinder{Origin: c.Add(geom.Vec3{X: 3, Y: 4}), Axis: geom.Vec3{X: -3, Y: -4}, Radius: 2, Min: -2, Max: 4}, B: geom.RawCylinder{Origin: c.Add(geom.Vec3{X: -4, Y: 3}), Axis: geom.Vec3{X: 4, Y: -3}, Radius: 2, Min: -2, Max: 4}, Tolerance: 1e-8, LengthUnit: .001}
}
func TestCylinderPairSetRecipes(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"union", "contact", "gap"} {
		t.Run(kind, func(t *testing.T) {
			r := cylinderSetFixture()
			diff := kind != "union"
			if kind == "gap" {
				r.B.Radius = 3
			}
			var b bytes.Buffer
			if e := writeCylinderSet(ctx, &b, r, diff); e != nil {
				t.Fatal(e)
			}
			got, u, d, e := readCylinderSet(ctx, bytes.NewReader(b.Bytes()), 0, diff)
			if e != nil || !reflect.DeepEqual(got, r) {
				t.Fatal(got, e)
			}
			var solids []parametric.Solid
			var groups [][3][]int
			if diff {
				solids = d.Solids[:]
				for i := 0; i < 2; i++ {
					groups = append(groups, [3][]int{d.StockWallFaces[i], d.CutterWallFaces[i], d.StockCapFaces[i]})
				}
				if kind == "contact" && (d.ContactNodes == nil || d.MinimumGapSquared != nil) {
					t.Fatal("contact lost")
				}
				if kind == "gap" && (d.ContactNodes != nil || d.MinimumGapSquared == nil || d.MinimumGapSquared.RatString() != "20") {
					t.Fatal("gap lost")
				}
			} else {
				solids = []parametric.Solid{u.Solid}
				groups = [][3][]int{{append(append([]int{}, u.WallFaces[0]...), u.WallFaces[1]...), append(append([]int{}, u.LowerCapFaces[0]...), u.LowerCapFaces[1]...), append(append([]int{}, u.UpperCapFaces[0]...), u.UpperCapFaces[1]...)}}
				if u.CommonVolumeSquared == nil {
					t.Fatal("volume proof lost")
				}
			}
			for i, s := range solids {
				seen := map[int]bool{}
				for _, g := range groups[i] {
					for _, id := range g {
						if id < 0 || id >= len(s.Faces()) || seen[id] {
							t.Fatal("ancestry")
						}
						seen[id] = true
					}
				}
				if len(seen) != len(s.Faces()) {
					t.Fatal("lost face")
				}
				var n bytes.Buffer
				if e = parametric.WriteJSON(&n, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := parametric.ReadJSON(bytes.NewReader(n.Bytes()), parametric.JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				var again bytes.Buffer
				if e = parametric.WriteJSON(&again, loaded); e != nil || !bytes.Equal(n.Bytes(), again.Bytes()) {
					t.Fatal("native charts changed", e)
				}
			}
			var again bytes.Buffer
			if e = writeCylinderSet(ctx, &again, got, diff); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
				t.Fatal("noncanonical", e)
			}
			if _, _, _, e = readCylinderSet(ctx, bytes.NewReader(b.Bytes()), 0, !diff); e == nil {
				t.Fatal("cross kind")
			}
			if _, _, _, e = readCylinderSet(ctx, bytes.NewReader(b.Bytes()), 10, diff); e == nil {
				t.Fatal("limit")
			}
			var env cylinderSetEnvelope
			if e = json.Unmarshal(b.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			if kind == "contact" {
				env.ContactNodes[0].RootSign *= -1
			} else if kind == "gap" {
				env.MinimumGapSquared = "21"
			} else {
				env.CommonVolumeSquared = "1e999999999"
			}
			bad, _ := json.Marshal(env)
			if _, _, _, e = readCylinderSet(ctx, bytes.NewReader(bad), 0, diff); e == nil {
				t.Fatal("proof tamper")
			}
			if e = json.Unmarshal(b.Bytes(), &env); e != nil {
				t.Fatal(e)
			}
			if diff {
				env.Boundaries[0], env.Boundaries[1] = env.Boundaries[1], env.Boundaries[0]
			} else {
				env.WallFaces[0][0] = env.WallFaces[0][0] + 1
			}
			bad, _ = json.Marshal(env)
			if _, _, _, e = readCylinderSet(ctx, bytes.NewReader(bad), 0, diff); e == nil {
				t.Fatal("component/ancestry tamper")
			}
		})
	}
}
func TestCylinderPairSetAtomicFailure(t *testing.T) {
	r := cylinderSetFixture()
	r.B.Origin.Z++
	for _, diff := range []bool{false, true} {
		var b bytes.Buffer
		if e := writeCylinderSet(context.Background(), &b, r, diff); e == nil || b.Len() != 0 {
			t.Fatal("partial unsupported output", e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if e := writeCylinderSet(ctx, &b, cylinderSetFixture(), diff); e == nil || b.Len() != 0 {
			t.Fatal("partial canceled output")
		}
	}
}

func TestCylinderPairSetPublicCodecs(t *testing.T) {
	ctx := context.Background()
	r := cylinderSetFixture()
	var u, d bytes.Buffer
	if e := WriteCylinderPairUnionRecipeJSON(ctx, &u, r); e != nil {
		t.Fatal(e)
	}
	if _, s, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(u.Bytes()), 0); e != nil || len(s.Solid.Faces()) == 0 {
		t.Fatal(e)
	}
	if e := WriteCylinderPairDifferenceRecipeJSON(ctx, &d, r); e != nil {
		t.Fatal(e)
	}
	if _, s, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(d.Bytes()), 0); e != nil || s.ContactNodes == nil {
		t.Fatal(e)
	}
	if _, _, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(d.Bytes()), 0); e == nil {
		t.Fatal("difference accepted as union")
	}
	if _, _, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(u.Bytes()), 0); e == nil {
		t.Fatal("union accepted as difference")
	}
}
