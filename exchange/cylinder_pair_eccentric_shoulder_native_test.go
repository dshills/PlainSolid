package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"plainsolid/geom"
)

func TestFiniteCylinderEccentricShoulderRecipe(t *testing.T) {
	ctx := context.Background()
	r := CylinderPairCommonRecipe{Name: "eccentric shoulder", LengthUnit: .001, Tolerance: 1e-9,
		A: geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 1, Z: 3}, Axis: geom.Vec3{Z: -2}, Min: 0, Max: 2, Radius: 4},
		B: geom.RawCylinder{Origin: geom.Vec3{X: 3, Y: 1}, Axis: geom.Vec3{Z: 2}, Min: -1.5, Max: .5, Radius: 2}}
	var out, repeated bytes.Buffer
	if err := WriteCylinderPairUnionRecipeJSON(ctx, &out, r); err != nil {
		t.Fatal(err)
	}
	recipe, got, err := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(out.Bytes()), 0)
	if err != nil || len(got.Solid.Faces()) != 20 || len(got.UpperCapFaces[0]) != 4 || got.CommonVolumePiCoefficient == nil || got.CommonVolumePiCoefficient.RatString() != "8" {
		t.Fatal("shoulder recipe", err)
	}
	if err = WriteCylinderPairUnionRecipeJSON(ctx, &repeated, recipe); err != nil || !bytes.Equal(out.Bytes(), repeated.Bytes()) {
		t.Fatal("canonical shoulder encoding", err)
	}
	var changed map[string]json.RawMessage
	if err = json.Unmarshal(out.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	changed["UpperCapFaces"] = json.RawMessage(`[[3,8,13,18],null]`)
	bad, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(bad), 0); err == nil {
		t.Fatal("substituted cap ancestry was accepted")
	}
}
