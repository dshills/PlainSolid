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

func TestAxisPolygonRevolveSTEPOCCT(t *testing.T) {
	t.Parallel()
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	referencePath := filepath.Join(t.TempDir(), "reference.jsonl")
	stepOracle(t, referencePath, "polygon-revolve-axis")
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
	for i := 0; i < 9; i++ {
		var x measure
		if e = dec.Decode(&x); e != nil {
			t.Fatal(e)
		}
		references = append(references, x)
	}
	referenceIndex := 0
	for _, kind := range []string{"axis-concave", "apex", "apex-end"} {
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

func TestAxisPolygonRevolveSTEPTopology(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"axis-concave", "apex"} {
		m := polygonRevolveFixture(t, kind, math.Pi/2)
		var b bytes.Buffer
		if e := WritePolygonRevolves(&b, m); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		count := map[string]int{}
		uses := map[int64][2]int{}
		for _, entity := range f.Entities {
			for _, record := range entity.Records {
				count[record.Type]++
				if record.Type == "EDGE_CURVE" {
					a, e := ref(record.Args[1])
					if e != nil {
						t.Fatal(e)
					}
					z, e := ref(record.Args[2])
					if e != nil || a == z {
						t.Fatal("degenerate pole edge", e)
					}
				}
				if record.Type == "ORIENTED_EDGE" {
					id, e := ref(record.Args[3])
					if e != nil {
						t.Fatal(e)
					}
					c := uses[id]
					sense, e := boolean(record.Args[4])
					if e != nil {
						t.Fatal(e)
					}
					if sense {
						c[0]++
					} else {
						c[1]++
					}
					uses[id] = c
				}
			}
		}
		n := len(m.Bodies[0].Profile)
		if count["VERTEX_POINT"] != 2*n-2 || count["EDGE_CURVE"] != 3*n-3 || count["ADVANCED_FACE"] != n+1 || count["PCURVE"] != 2*(3*n-3) {
			t.Fatal(count)
		}
		for _, c := range uses {
			if c != [2]int{1, 1} {
				t.Fatal("unowned axis edge", c)
			}
		}
		m.Bodies[0].AllowAxisContact = false
		b.Reset()
		if e := WritePolygonRevolves(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("implicit axis broadening")
		}
	}
	for _, p := range [][]geom.Vec2{
		{{}, {X: 2}, {X: 2, Y: 2}, {X: 1, Y: 2}},
		{{}, {X: 2}, {Y: 1}, {X: 1, Y: 2}},
		{{}, {X: 2}, {X: 2, Y: 2}, {Y: 2}, {Y: 1}},
	} {
		m := polygonRevolveFixture(t, "apex", 1)
		m.Bodies[0].Profile = p
		var b bytes.Buffer
		if e := WritePolygonRevolves(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("unsupported axis incidence emitted")
		}
	}
}
