package exchange

import (
	"bytes"
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderPairAsymmetricRecipes(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	a := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 3.5}
	b := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 2}, Radius: 3, Min: -4, Max: 3}
	checkCaps := func(s parametric.Solid, c geom.RawCylinder, ids []int, want float64) {
		t.Helper()
		for _, id := range ids {
			f := s.Faces()[id]
			p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
			if e != nil {
				t.Fatal(e)
			}
			if p.Sub(c.Origin).Dot(c.Axis) != want*c.Axis.Dot(c.Axis) {
				t.Fatal("cap changed", p, want)
			}
		}
	}
	for _, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		r := CylinderPairCommonRecipe{Name: "asymmetric union", A: pair[0], B: pair[1], Tolerance: 1e-8, LengthUnit: .001}
		var encoded bytes.Buffer
		if e := WriteCylinderPairUnionRecipeJSON(ctx, &encoded, r); e != nil {
			t.Fatal(e)
		}
		got, u, e := ReadCylinderPairUnionRecipeJSON(ctx, bytes.NewReader(encoded.Bytes()), 0)
		if e != nil || got != r {
			t.Fatal(got, e)
		}
		for owner, c := range pair {
			checkCaps(u.Solid, c, u.LowerCapFaces[owner], c.Min)
			checkCaps(u.Solid, c, u.UpperCapFaces[owner], c.Max)
		}
		var again bytes.Buffer
		if e = WriteCylinderPairUnionRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(encoded.Bytes(), again.Bytes()) {
			t.Fatal("canonical", e)
		}
	}
	r := CylinderPairCommonRecipe{Name: "asymmetric difference", A: a, B: b, Tolerance: 1e-8, LengthUnit: .001}
	var encoded bytes.Buffer
	if e := WriteCylinderPairDifferenceRecipeJSON(ctx, &encoded, r); e != nil {
		t.Fatal(e)
	}
	got, d, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(encoded.Bytes()), 0)
	if e != nil || got != r || len(d.Solids[1].Faces()) != 0 {
		t.Fatal(got, e)
	}
	for _, id := range d.StockCapFaces[0] {
		f := d.Solids[0].Faces()[id]
		p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
		if e != nil {
			t.Fatal(e)
		}
		v := p.Sub(a.Origin).Dot(a.Axis) / a.Axis.Dot(a.Axis)
		if v != a.Min && v != a.Max {
			t.Fatal("cap changed", v)
		}
	}
	var again bytes.Buffer
	if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(encoded.Bytes(), again.Bytes()) {
		t.Fatal("canonical", e)
	}
	var native bytes.Buffer
	if e = parametric.WriteJSON(&native, d.Solids[0]); e != nil {
		t.Fatal(e)
	}
	restored, e := parametric.ReadJSON(&native, parametric.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	before, e := d.Solids[0].MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	after, e := restored.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || before != after {
		t.Fatal("reload mass", e)
	}
	// Altering the raw stock cap without replacing its retained native proof fails.
	altered := bytes.Replace(encoded.Bytes(), []byte(`"Max":3.5`), []byte(`"Max":4`), 1)
	if bytes.Equal(altered, encoded.Bytes()) {
		t.Fatal("tamper fixture")
	}
	if _, _, e = ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(altered), 0); e == nil {
		t.Fatal("accepted altered cap")
	}
}
