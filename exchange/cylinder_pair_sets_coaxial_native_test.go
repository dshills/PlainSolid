package exchange

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderPairCoaxialThroughRecipe(t *testing.T) {
	ctx := context.Background()
	r := CylinderPairCommonRecipe{Name: "signed coaxial through", A: geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}, B: geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 5}, Axis: geom.Vec3{Z: 3}, Radius: 3, Min: -3, Max: 2}, Tolerance: 1e-9, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteCylinderPairDifferenceRecipeJSON(ctx, &b, r); e != nil {
		t.Fatal(e)
	}
	got, d, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(b.Bytes()), 0)
	if e != nil || got != r {
		t.Fatal(got, e)
	}
	if len(d.Solids[0].Faces()) == 0 || len(d.Solids[1].Faces()) != 0 || d.ContactNodes != nil || d.MinimumGapSquared != nil {
		t.Fatal("slots/proofs")
	}
	if len(d.StockCapFaces[0]) != 8 || len(d.StockWallFaces[0]) == 0 || len(d.CutterWallFaces[0]) == 0 {
		t.Fatal("roles")
	}
	if len(d.StockWallFaces[1])+len(d.CutterWallFaces[1])+len(d.StockCapFaces[1]) != 0 {
		t.Fatal("empty slot ancestry")
	}
	faces := d.Solids[0].Faces()
	seen := map[int]bool{}
	for role, ids := range [][]int{d.StockWallFaces[0], d.CutterWallFaces[0], d.StockCapFaces[0]} {
		for index, id := range ids {
			if id < 0 || id >= len(faces) || seen[id] {
				t.Fatal("ancestry")
			}
			seen[id] = true
			f := faces[id]
			u, v := (f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2
			p, e := f.Surface.Eval(u, v)
			if e != nil {
				t.Fatal(e)
			}
			if role == 2 {
				want := [2]float64{7, -1}[index/4]
				if p.Z != want {
					t.Fatal("original raw cap order", index, p.Z, want)
				}
			} else {
				want := 5.0
				if role == 1 {
					want = 3
				}
				radial := geom.Vec3{X: p.X - 1, Y: p.Y - 2}
				if math.Abs(radial.Length()-want) > 1e-11 {
					t.Fatal("radius", role, p)
				}
				du, dv, e := f.Surface.Partials(u, v)
				if e != nil {
					t.Fatal(e)
				}
				n := du.Cross(dv)
				if f.Reversed {
					n = n.Scale(-1)
				}
				if role == 1 && n.Dot(radial) >= 0 || role == 0 && n.Dot(radial) <= 0 {
					t.Fatal("wall orientation", role)
				}
			}
		}
	}
	if len(seen) != len(faces) {
		t.Fatal("unowned faces")
	}
	var native bytes.Buffer
	if e = parametric.WriteJSON(&native, d.Solids[0]); e != nil {
		t.Fatal(e)
	}
	s, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = parametric.WriteJSON(&again, s); e != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("chart changed", e)
	}
	m, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-128*math.Pi) > 1e-8 || math.Abs(m.Area-160*math.Pi) > 1e-8 || m.Centroid.Sub(r.A.Origin).Length() > 1e-9 {
		t.Fatal(m, e)
	}
	again.Reset()
	if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("recipe changed", e)
	}
}
