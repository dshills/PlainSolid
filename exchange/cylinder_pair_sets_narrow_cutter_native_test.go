package exchange

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderPairNarrowCutterRecipe(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	r := CylinderPairCommonRecipe{Name: "wider stock narrow transverse bore", A: geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}, B: geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -2, Max: 2}, Tolerance: 1e-8, LengthUnit: .001}
	var b bytes.Buffer
	if e := WriteCylinderPairDifferenceRecipeJSON(ctx, &b, r); e != nil {
		t.Fatal(e)
	}
	got, d, e := ReadCylinderPairDifferenceRecipeJSON(ctx, bytes.NewReader(b.Bytes()), 0)
	if e != nil || got != r {
		t.Fatal(got, e)
	}
	if len(d.Solids[0].Faces()) == 0 || len(d.Solids[1].Faces()) != 0 || d.ContactNodes != nil || d.MinimumGapSquared != nil {
		t.Fatal("single component semantics")
	}
	if len(d.StockWallFaces[1])+len(d.CutterWallFaces[1])+len(d.StockCapFaces[1]) != 0 {
		t.Fatal("empty ancestry")
	}
	faces := d.Solids[0].Faces()
	seen := map[int]bool{}
	for role, ids := range [][]int{d.StockWallFaces[0], d.CutterWallFaces[0], d.StockCapFaces[0]} {
		if len(ids) == 0 {
			t.Fatal("lost role", role)
		}
		for _, id := range ids {
			if id < 0 || id >= len(faces) || seen[id] {
				t.Fatal("ancestry")
			}
			seen[id] = true
			f := faces[id]
			p, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, (f.V.Min+f.V.Max)/2)
			if e != nil {
				t.Fatal(e)
			}
			raw := r.A
			if role == 1 {
				raw = r.B
			}
			q := p.Sub(raw.Origin)
			if role == 2 {
				at := q.Dot(raw.Axis) / raw.Axis.Dot(raw.Axis)
				if at != r.A.Min && at != r.A.Max {
					t.Fatal("stock cap moved", at)
				}
			} else {
				radial := q.Dot(q) - q.Dot(raw.Axis)*q.Dot(raw.Axis)/raw.Axis.Dot(raw.Axis)
				if math.Abs(radial-raw.Radius*raw.Radius) > 1e-10 {
					t.Fatal("wrong support", role, radial)
				}
			}
		}
	}
	if len(seen) != len(faces) {
		t.Fatal("unowned face")
	}
	var native bytes.Buffer
	if e = parametric.WriteJSON(&native, d.Solids[0]); e != nil {
		t.Fatal(e)
	}
	loaded, e := parametric.ReadJSON(bytes.NewReader(native.Bytes()), parametric.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var restored bytes.Buffer
	if e = parametric.WriteJSON(&restored, loaded); e != nil || !bytes.Equal(native.Bytes(), restored.Bytes()) {
		t.Fatal("charts changed", e)
	}
	// Independent cross-section integral, x=3 sin(theta), avoids endpoint roots.
	const n = 4096
	h := (math.Pi / 2) / n
	sum := 0.0
	for i := 0; i <= n; i++ {
		theta := float64(i) * h
		v := 72 * math.Sqrt(25-9*math.Sin(theta)*math.Sin(theta)) * math.Cos(theta) * math.Cos(theta)
		w := 2.0
		if i == 0 || i == n {
			w = 1
		} else if i%2 == 1 {
			w = 4
		}
		sum += w * v
	}
	want := 200*math.Pi - h*sum/3
	m, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(m.Volume-want) > 1e-5 || m.Centroid.Sub(center).Length() > 1e-7 {
		t.Fatal(m, want, e)
	}
	var again bytes.Buffer
	if e = WriteCylinderPairDifferenceRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("canonical recipe changed", e)
	}
}
