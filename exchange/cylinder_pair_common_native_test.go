package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func cylinderPairRecipeFixture() CylinderPairCommonRecipe {
	return CylinderPairCommonRecipe{Name: "raw scaled axes", A: geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 2, Min: -3, Max: 3}, B: geom.RawCylinder{Axis: geom.Vec3{Z: 3}, Radius: 2, Min: -3, Max: 3}, Tolerance: 1e-8, LengthUnit: .001}
}
func TestCylinderPairCommonRecipeNative(t *testing.T) {
	ctx := context.Background()
	cross := cylinderPairRecipeFixture()
	cap := cross
	cap.A.Axis = geom.Vec3{X: 1}
	cap.B.Axis = geom.Vec3{Z: 1}
	cap.A.Radius, cap.B.Radius = 5, 5
	cap.A.Min, cap.A.Max = -6, 2
	cap.B.Min, cap.B.Max = -6, 6
	skew := cross
	skew.A.Radius = 3
	skew.A.Min, skew.A.Max = -5, 5
	skew.B = geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 3, Y: 4}, Radius: 2, Min: -5, Max: 5}
	parallel := cross
	parallel.A = geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2}, Axis: geom.Vec3{Y: 2}, Radius: 2, Min: -1, Max: 3}
	parallel.B = geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 10}, Axis: geom.Vec3{Y: -3}, Radius: 2, Min: 1, Max: 2}
	empty := parallel
	empty.B.Origin.X = 5
	for i, r := range []CylinderPairCommonRecipe{cross, cap, skew, parallel, empty} {
		t.Run([]string{"perpendicular-scaled", "finite-cap", "skew", "parallel-reversed", "empty"}[i], func(t *testing.T) {
			var data bytes.Buffer
			if e := WriteCylinderPairCommonRecipeJSON(ctx, &data, r); e != nil {
				t.Fatal(e)
			}
			got, s, e := ReadCylinderPairCommonRecipeJSON(ctx, bytes.NewReader(data.Bytes()), 0)
			if e != nil || !reflect.DeepEqual(got, r) {
				t.Fatal(got, e)
			}
			want := []parametric.CylinderCommonConstruction{parametric.CylinderCommonIntersectingAxes, parametric.CylinderCommonPerpendicularCap, parametric.CylinderCommonSkewAxes, parametric.CylinderCommonParallelAxes, parametric.CylinderCommonEmpty}[i]
			if s.Construction != want {
				t.Fatal(s.Construction, want)
			}
			seen := map[int]bool{}
			for operand := 0; operand < 2; operand++ {
				for _, group := range [][]int{s.SideFaces[operand], s.LowerCapFaces[operand], s.UpperCapFaces[operand]} {
					for _, face := range group {
						if face < 0 || face >= len(s.Solid.Faces()) || seen[face] {
							t.Fatal("ancestry", face)
						}
						seen[face] = true
					}
				}
			}
			if len(seen) != len(s.Solid.Faces()) {
				t.Fatal("lost ancestry")
			}
			var again bytes.Buffer
			if e = WriteCylinderPairCommonRecipeJSON(ctx, &again, got); e != nil || !bytes.Equal(again.Bytes(), data.Bytes()) {
				t.Fatal("changed recipe", e)
			}
			if i == 4 {
				return
			}
			var native bytes.Buffer
			if e = parametric.WriteJSON(&native, s.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := parametric.ReadJSON(&native, parametric.JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			a, e := s.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || a != b {
				t.Fatal("changed native mass", e)
			}
			if i == 0 && math.Abs(a.Volume-128.0/3) > 1e-7 {
				t.Fatal("analytic perpendicular volume", a.Volume)
			}
		})
	}
}

func TestCylinderPairCommonRecipeRejects(t *testing.T) {
	ctx := context.Background()
	var b bytes.Buffer
	r := cylinderPairRecipeFixture()
	if e := WriteCylinderPairCommonRecipeJSON(ctx, &b, r); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	var envelope cylinderPairCommonEnvelope
	if e := json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	badGroup := envelope
	badGroup.SideFaces[0] = []int{999}
	groupJSON, _ := json.Marshal(badGroup)
	mutations := [][]byte{
		append(append([]byte(nil), raw...), []byte("{}")...),
		bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1),
		bytes.Replace(raw, []byte(`"Version":1`), []byte(`"Version":1,"Unknown":0`), 1),
		bytes.Replace(raw, []byte(`"Tolerance":1e-8`), []byte(`"Tolerance":0.00000001`), 1),
		bytes.Replace(raw, []byte(`"Construction":1`), []byte(`"Construction":2`), 1),
		bytes.Replace(raw, []byte(`"Min":-3`), []byte(`"Min":0`), 1),
		bytes.Replace(raw, []byte(`"Reversed":false`), []byte(`"Reversed":true`), 1), groupJSON,
	}
	for i, data := range mutations {
		if bytes.Equal(data, raw) {
			t.Fatal("ineffective mutation", i)
		}
		source, solid, e := ReadCylinderPairCommonRecipeJSON(ctx, bytes.NewReader(data), 0)
		if e == nil || source.Name != "" || len(solid.Solid.Faces()) != 0 {
			t.Fatal("accepted mutation", i, e)
		}
	}
	if _, _, e := ReadCylinderPairCommonRecipeJSON(ctx, bytes.NewReader(raw), 32); e == nil {
		t.Fatal("byte limit")
	}
	var spaced bytes.Buffer
	if e := json.Indent(&spaced, raw, "", "  "); e != nil {
		t.Fatal(e)
	}
	if _, _, e := ReadCylinderPairCommonRecipeJSON(ctx, &spaced, 0); e != nil {
		t.Fatal("whitespace", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := ReadCylinderPairCommonRecipeJSON(canceled, bytes.NewReader(raw), 0); e != context.Canceled {
		t.Fatal(e)
	}
	b.Reset()
	r.A.Axis = geom.Vec3{}
	if e := WriteCylinderPairCommonRecipeJSON(ctx, &b, r); e == nil || b.Len() != 0 {
		t.Fatal("invalid publish", e)
	}
	if e := WriteCylinderPairCommonRecipeJSON(ctx, shortCylinderRecipeWriter{}, cylinderPairRecipeFixture()); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}

type shortCylinderRecipeWriter struct{}

func (shortCylinderRecipeWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
