package exchange

import (
	"bytes"
	"context"
	"plainsolid/geom"
	"testing"
)

func TestParallelTransverseNativeRecipes(t *testing.T) {
	ctx := context.Background()
	r := CylinderPairCommonRecipe{Name: "parallel crossing", A: geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}, B: geom.RawCylinder{Origin: geom.Vec3{X: 5, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 4}, Radius: 3, Min: -1, Max: 1}, Tolerance: 1e-8, LengthUnit: .001}
	for _, swap := range []bool{false, true} {
		if swap {
			r.A, r.B = r.B, r.A
		}
		var a, b bytes.Buffer
		if e := WriteCylinderPairUnionRecipeJSON(ctx, &a, r); e != nil {
			t.Fatal(e)
		}
		rr, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(a.Bytes()), 0)
		if e != nil {
			t.Fatal(e)
		}
		if u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient != nil {
			t.Fatal("invented rational volume")
		}
		if e = WriteCylinderPairUnionRecipeJSON(ctx, &b, rr); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatal("union canonical", e)
		}
		a.Reset()
		b.Reset()
		if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &a, r); e != nil {
			t.Fatal(e)
		}
		rr, d, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(a.Bytes()), 0)
		if e != nil {
			t.Fatal(e)
		}
		if len(d.StockCapFaces[0]) != 2 || len(d.Solids[1].Faces()) != 0 {
			t.Fatal("ancestry")
		}
		if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &b, rr); e != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Fatal("difference canonical", e)
		}
	}
}
