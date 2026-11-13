package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func polygonRevolveFixture(t *testing.T, kind string, angle float64) PolygonRevolveModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := []geom.Vec2{{X: 1}, {X: 3}, {X: 3, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}
	if kind == "cone" {
		p = []geom.Vec2{{X: 1}, {X: 3}, {X: 2.5, Y: 1}, {X: 3, Y: 2}, {X: 1.5, Y: 2}, {X: 1, Y: 1}}
	}
	return PolygonRevolveModel{LengthUnit: .001, Bodies: []PolygonRevolveBody{{Name: kind, Frame: f, Profile: p, Angle: angle}}}
}
func TestPolygonRevolveSTEP(t *testing.T) {
	for _, kind := range []string{"concave", "cone"} {
		for _, angle := range []float64{math.Pi / 2, -math.Pi / 3, 3 * math.Pi / 2} {
			m := polygonRevolveFixture(t, kind, angle)
			var b bytes.Buffer
			if e := WritePolygonRevolves(&b, m); e != nil {
				t.Fatal(e)
			}
			f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			count := map[string]int{}
			for _, x := range f.Entities {
				for _, c := range x.Records {
					count[c.Type]++
				}
			}
			useCounts := map[int64][2]int{}
			for _, entity := range f.Entities {
				for _, record := range entity.Records {
					if record.Type == "ORIENTED_EDGE" {
						id, e := ref(record.Args[3])
						if e != nil {
							t.Fatal(e)
						}
						counts := useCounts[id]
						if record.Args[4].Text == "T" {
							counts[0]++
						} else {
							counts[1]++
						}
						useCounts[id] = counts
					}
				}
			}
			for _, counts := range useCounts {
				if counts != [2]int{1, 1} {
					t.Fatal("edge not oppositely owned twice", counts)
				}
			}
			if count["ADVANCED_FACE"] != 8 || count["EDGE_CURVE"] != 18 || count["VERTEX_POINT"] != 12 || count["PCURVE"] != 36 || count["MANIFOLD_SOLID_BREP"] != 1 {
				t.Fatal(count)
			}
			if e := WritePolygonRevolves(failWriter{}, m); e == nil {
				t.Fatal("write error lost")
			}
		}
	}
	for _, kind := range []string{"axis", "cross", "angle", "full", "tiny", "size", "unit", "frame", "name"} {
		t.Run(kind, func(t *testing.T) {
			m := polygonRevolveFixture(t, "concave", 1)
			switch kind {
			case "axis":
				m.Bodies[0].Profile[0].X = 0
			case "cross":
				m.Bodies[0].Profile[1], m.Bodies[0].Profile[4] = m.Bodies[0].Profile[4], m.Bodies[0].Profile[1]
			case "angle":
				m.Bodies[0].Angle = math.NaN()
			case "full":
				m.Bodies[0].Angle = 2 * math.Pi
			case "tiny":
				m.Bodies[0].Angle = 1e-18
			case "size":
				m.Bodies[0].Profile = make([]geom.Vec2, 33)
			case "unit":
				m.LengthUnit = 0
			case "frame":
				m.Bodies[0].Frame = geom.Frame{}
			case "name":
				m.Bodies[0].Name = "\xff"
			}
			var b bytes.Buffer
			if e := WritePolygonRevolves(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid body emitted", e)
			}
		})
	}
}
func TestPolygonRevolveSTEPOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	referencePath := filepath.Join(t.TempDir(), "reference.jsonl")
	stepOracle(t, referencePath, "polygon-revolve")
	referenceRaw, e := os.ReadFile(referencePath)
	if e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	var references []measure
	dec := json.NewDecoder(bytes.NewReader(referenceRaw))
	for i := 0; i < 6; i++ {
		var x measure
		if e = dec.Decode(&x); e != nil {
			t.Fatal(e)
		}
		references = append(references, x)
	}
	referenceIndex := 0
	for _, kind := range []string{"concave", "cone"} {
		for _, angle := range []float64{math.Pi / 2, -math.Pi / 3, 3 * math.Pi / 2} {
			t.Run(kind+real(angle), func(t *testing.T) {
				ref := references[referenceIndex]
				referenceIndex++
				m := polygonRevolveFixture(t, kind, angle)
				b := m.Bodies[0]
				native, e := trimmed.RevolvePolygon(context.Background(), b.Frame, b.Profile, b.Angle)
				if e != nil {
					t.Fatal(e)
				}
				mass, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
				if e != nil {
					t.Fatal(e)
				}
				var out bytes.Buffer
				if e = WritePolygonRevolves(&out, m); e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(t.TempDir(), "body.step")
				if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				result := filepath.Join(t.TempDir(), "mass.json")
				stepOracle(t, result, "step-mass-precise", path)
				raw, e := os.ReadFile(result)
				if e != nil {
					t.Fatal(e)
				}
				var got struct {
					Area, Volume float64
					Centroid     [3]float64
				}
				if e = json.Unmarshal(raw, &got); e != nil {
					t.Fatal(e)
				}
				if math.Abs(got.Volume-ref.Volume) > 1e-8 || math.Abs(got.Area-ref.Area) > 1e-8 {
					t.Fatal("independent OCCT revolution", got, ref)
				}
				if math.Abs(got.Volume-mass.Volume) > 1e-8 || math.Abs(got.Area-mass.Area) > 1e-8 {
					t.Fatal(got, mass)
				}
				if (geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}).Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8 {
					t.Fatal("independent centroid", got, ref)
				}
				if (geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}).Sub(mass.Centroid).Length() > 1e-8 {
					t.Fatal(got, mass)
				}
			})
		}
	}
}

func TestPolygonRevolveInputWinding(t *testing.T) {
	m := polygonRevolveFixture(t, "cone", -math.Pi/3)
	before := append([]geom.Vec2(nil), m.Bodies[0].Profile...)
	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		m.Bodies[0].Profile[i], m.Bodies[0].Profile[j] = m.Bodies[0].Profile[j], m.Bodies[0].Profile[i]
	}
	var b bytes.Buffer
	if e := WritePolygonRevolves(&b, m); e != nil {
		t.Fatal(e)
	}
	for i, p := range m.Bodies[0].Profile {
		if p != before[len(before)-1-i] {
			t.Fatal("caller profile mutated")
		}
	}
	if _, e := Parse(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}
