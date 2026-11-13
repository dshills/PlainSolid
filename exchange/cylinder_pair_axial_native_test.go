package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestFiniteCylinderAxialCutRecipe(t *testing.T) {
	ctx := context.Background()
	r := CylinderPairCommonRecipe{Name: "split cylinder", LengthUnit: .001, Tolerance: 1e-9,
		A: geom.RawCylinder{Axis: geom.Vec3{Z: -3}, Min: -2, Max: 2, Radius: 2},
		B: geom.RawCylinder{Origin: geom.Vec3{X: .5}, Axis: geom.Vec3{Z: 2}, Min: -.5, Max: .5, Radius: 3}}
	var buf, again bytes.Buffer
	if err := WriteCylinderPairDifferenceRecipeJSON(ctx, &buf, r); err != nil {
		t.Fatal(err)
	}
	recipe, got, err := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(buf.Bytes()), 0)
	if err != nil || got.AxialCutCaps[0] == nil || got.AxialCutCaps[1] == nil ||
		*got.AxialCutCaps[0] != (parametric.FiniteCylinderCutCap{Solid: 1, Face: 1}) ||
		*got.AxialCutCaps[1] != (parametric.FiniteCylinderCutCap{Solid: 0, Face: 2}) {
		t.Fatal("cap ancestry", got.AxialCutCaps, err)
	}
	if err = WriteCylinderPairDifferenceRecipeJSON(ctx, &again, recipe); err != nil || !bytes.Equal(buf.Bytes(), again.Bytes()) {
		t.Fatal("canonical reload", err)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	env["AxialCutCaps"] = json.RawMessage(`[{"Solid":0,"Face":1},{"Solid":0,"Face":2}]`)
	bad, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(bad), 0); err == nil {
		t.Fatal("altered original cutter ownership accepted")
	}
}
