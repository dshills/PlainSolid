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

func circularHoleExtrusionFixture(t *testing.T, height float64) CircularHoleExtrusionModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	return CircularHoleExtrusionModel{LengthUnit: .001, Bodies: []CircularHoleExtrusionBody{{Name: "two bores", Frame: f, Outer: []geom.Vec2{{}, {X: 6}, {X: 7, Y: 3}, {X: 4, Y: 5}, {Y: 4}}, Holes: []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 2, Y: 2}, Radius: .5}, {Center: geom.Vec2{X: 5, Y: 2}, Radius: .75}}, Height: height}}}
}
func TestCircularHoleExtrusionSTEP(t *testing.T) {
	t.Parallel()
	for _, height := range []float64{3, -3} {
		m := circularHoleExtrusionFixture(t, height)
		var b bytes.Buffer
		if e := WriteCircularHoleExtrusions(&b, m); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		count := map[string]int{}
		uses := map[int64][2]int{}
		for _, entity := range f.Entities {
			for _, r := range entity.Records {
				count[r.Type]++
				if r.Type == "ORIENTED_EDGE" {
					id, e := ref(r.Args[3])
					if e != nil {
						t.Fatal(e)
					}
					sense, e := boolean(r.Args[4])
					if e != nil {
						t.Fatal(e)
					}
					c := uses[id]
					if sense {
						c[0]++
					} else {
						c[1]++
					}
					uses[id] = c
				}
			}
		}
		if count["ADVANCED_FACE"] != 9 || count["EDGE_CURVE"] != 21 || count["VERTEX_POINT"] != 14 || count["PCURVE"] != 42 || count["SEAM_CURVE"] != 2 {
			t.Fatal(count)
		}
		for _, c := range uses {
			if c != [2]int{1, 1} {
				t.Fatal(c)
			}
		}
		if e = WriteCircularHoleExtrusions(failWriter{}, m); e == nil {
			t.Fatal("lost writer error")
		}
	}
	for _, kind := range []string{"outside", "touch", "overlap", "negative", "zero-height", "concave", "empty", "unit"} {
		t.Run(kind, func(t *testing.T) {
			m := circularHoleExtrusionFixture(t, 3)
			b := &m.Bodies[0]
			switch kind {
			case "outside":
				b.Holes[0].Center.X = -1
			case "touch":
				b.Holes[0].Center.X = .5
			case "overlap":
				b.Holes[1].Center = b.Holes[0].Center
			case "negative":
				b.Holes[0].Radius = -1
			case "zero-height":
				b.Height = 0
			case "concave":
				b.Outer[2] = geom.Vec2{X: 2, Y: 2}
			case "empty":
				b.Holes = nil
			case "unit":
				m.LengthUnit = 0
			}
			var out bytes.Buffer
			if e := WriteCircularHoleExtrusions(&out, m); e == nil || out.Len() != 0 {
				t.Fatal("invalid extrusion emitted")
			}
		})
	}
}
func TestCircularHoleExtrusionSTEPOCCT(t *testing.T) {
	t.Parallel()
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	type measure struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	reference := filepath.Join(t.TempDir(), "ref.jsonl")
	stepOracle(t, reference, "circular-hole-extrusion")
	data, e := os.ReadFile(reference)
	if e != nil {
		t.Fatal(e)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	for _, height := range []float64{3, -3} {
		t.Run(real(height), func(t *testing.T) {
			var ref measure
			if e = dec.Decode(&ref); e != nil {
				t.Fatal(e)
			}
			m := circularHoleExtrusionFixture(t, height)
			b := m.Bodies[0]
			native, e := trimmed.ExtrudePolygonWithCircularHoles(context.Background(), b.Frame, b.Outer, b.Holes, b.Height)
			if e != nil {
				t.Fatal(e)
			}
			mass, e := native.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			if e = WriteCircularHoleExtrusions(&out, m); e != nil {
				t.Fatal(e)
			}
			decoded, e := ReadCircularHoleExtrusions(bytes.NewReader(out.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			out.Reset()
			if e = WriteCircularHoleExtrusions(&out, decoded); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "body.step")
			if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			result := filepath.Join(t.TempDir(), "mass.json")
			stepOracle(t, result, "step-mass-precise", path)
			data, e := os.ReadFile(result)
			if e != nil {
				t.Fatal(e)
			}
			var got measure
			if e = json.Unmarshal(data, &got); e != nil {
				t.Fatal(e)
			}
			want := math.Abs(height) * (28.5 - math.Pi*(.25+.5625))
			if math.Abs(got.Volume-want) > 1e-8 || math.Abs(got.Volume-mass.Volume) > 1e-8 || math.Abs(got.Area-mass.Area) > 1e-8 || math.Abs(got.Volume-ref.Volume) > 1e-8 || math.Abs(got.Area-ref.Area) > 1e-8 {
				t.Fatal(got, mass, ref, want)
			}
			for i, v := range []float64{mass.Centroid.X, mass.Centroid.Y, mass.Centroid.Z} {
				if math.Abs(got.Centroid[i]-v) > 1e-8 || math.Abs(got.Centroid[i]-ref.Centroid[i]) > 1e-8 {
					t.Fatal(got, mass, ref)
				}
			}
		})
	}
}

func TestCircularHoleExtrusionWinding(t *testing.T) {
	t.Parallel()
	m := circularHoleExtrusionFixture(t, -3)
	before := append([]geom.Vec2(nil), m.Bodies[0].Outer...)
	for a, z := 0, len(before)-1; a < z; a, z = a+1, z-1 {
		m.Bodies[0].Outer[a], m.Bodies[0].Outer[z] = m.Bodies[0].Outer[z], m.Bodies[0].Outer[a]
	}
	var b bytes.Buffer
	if e := WriteCircularHoleExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	for i, p := range m.Bodies[0].Outer {
		if p != before[len(before)-1-i] {
			t.Fatal("caller outer loop mutated")
		}
	}
	if _, e := Parse(&b, Limits{}); e != nil {
		t.Fatal(e)
	}
}
