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
	"testing"
)

func circularBlindExtrusionFixture(t *testing.T, height float64, all bool) CircularHoleExtrusionModel {
	m := circularHoleExtrusionFixture(t, height)
	b := &m.Bodies[0]
	if all {
		b.Blind = append(b.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: b.Holes[0], Depth: .75})
	}
	b.Blind = append(b.Blind, trimmed.ExtrusionBlindHole{ExtrusionCircle: b.Holes[1], Depth: 1.25})
	if all {
		b.Holes = nil
	} else {
		b.Holes = b.Holes[:1]
	}
	return m
}
func TestCircularBlindExtrusionTopology(t *testing.T) {
	for _, height := range []float64{3, -3} {
		for _, all := range []bool{false, true} {
			m := circularBlindExtrusionFixture(t, height, all)
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
			singleBoundPlanes := 0
			floorHeights := []float64{}
			for _, entity := range f.Entities {
				for _, r := range entity.Records {
					count[r.Type]++
					if r.Type == "ORIENTED_EDGE" {
						id, _ := ref(r.Args[3])
						s, _ := boolean(r.Args[4])
						c := uses[id]
						if s {
							c[0]++
						} else {
							c[1]++
						}
						uses[id] = c
					}
					if r.Type == "ADVANCED_FACE" {
						surface, _ := ref(r.Args[2])
						fr, e := planeFrame(f, surface)
						if e != nil {
							continue
						}
						bounds, e := listRefs(r.Args[1], 1)
						if e != nil {
							t.Fatal(e)
						}
						if len(bounds) == 1 {
							singleBoundPlanes++
							loops, e := simple(f, bounds[0], "FACE_OUTER_BOUND", 3)
							if e != nil {
								t.Fatal(e)
							}
							lid, _ := ref(loops.Args[1])
							loop, _ := simple(f, lid, "EDGE_LOOP", 2)
							edges, e := listRefs(loop.Args[1], 1)
							if e != nil {
								t.Fatal(e)
							}
							if len(edges) == 1 {
								floorHeights = append(floorHeights, m.Bodies[0].Frame.ToLocal(fr.Origin()).Z)
								sense, _ := boolean(r.Args[3])
								if sense != (height < 0) {
									t.Fatal("floor material normal")
								}
							}
						}
					}
				}
			}
			blind := len(m.Bodies[0].Blind)
			if count["ADVANCED_FACE"] != 9+blind || count["EDGE_CURVE"] != 21 || count["VERTEX_POINT"] != 14 || count["PCURVE"] != 42 || len(floorHeights) != blind {
				t.Fatal(count, singleBoundPlanes, floorHeights)
			}
			chi := count["VERTEX_POINT"] - count["EDGE_CURVE"] + count["ADVANCED_FACE"] - count["FACE_BOUND"]
			if chi != 2-2*len(m.Bodies[0].Holes) {
				t.Fatal("blind floor did not close material", chi)
			}
			for _, c := range uses {
				if c != [2]int{1, 1} {
					t.Fatal(c)
				}
			}
			expected := map[float64]bool{}
			for _, h := range m.Bodies[0].Blind {
				expected[math.Copysign(h.Depth, height)] = true
			}
			for _, z := range floorHeights {
				if !expected[z] {
					t.Fatal("unexpected floor depth", z)
				}
				delete(expected, z)
			}
			if len(expected) != 0 {
				t.Fatal("missing floor", expected)
			}

		}
	}
	for _, kind := range []string{"zero", "through", "past", "negative", "nan", "overlap"} {
		t.Run(kind, func(t *testing.T) {
			m := circularBlindExtrusionFixture(t, 3, false)
			h := &m.Bodies[0].Blind[0]
			switch kind {
			case "zero":
				h.Depth = 0
			case "through":
				h.Depth = 3
			case "past":
				h.Depth = 4
			case "negative":
				h.Depth = -1
			case "nan":
				h.Depth = math.NaN()
			case "overlap":
				h.Center = m.Bodies[0].Holes[0].Center
			}
			var b bytes.Buffer
			if e := WriteCircularHoleExtrusions(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid pocket emitted")
			}
		})
	}
}
func TestCircularBlindExtrusionOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	type measure struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	for _, all := range []bool{false, true} {
		mode := "circular-pocket-extrusion"
		if all {
			mode = "circular-blind-extrusion"
		}
		refPath := filepath.Join(t.TempDir(), "ref.jsonl")
		stepOracle(t, refPath, mode)
		data, e := os.ReadFile(refPath)
		if e != nil {
			t.Fatal(e)
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		for _, height := range []float64{3, -3} {
			t.Run(mode+real(height), func(t *testing.T) {
				var ref measure
				if e = dec.Decode(&ref); e != nil {
					t.Fatal(e)
				}
				m := circularBlindExtrusionFixture(t, height, all)
				b := m.Bodies[0]
				native, e := trimmed.ExtrudePolygonWithHoles(context.Background(), b.Frame, b.Outer, b.Holes, b.Blind, b.Height)
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
				raw, e := os.ReadFile(result)
				if e != nil {
					t.Fatal(e)
				}
				var got measure
				if e = json.Unmarshal(raw, &got); e != nil {
					t.Fatal(e)
				}
				want := math.Abs(height) * 28.5
				for _, h := range b.Holes {
					want -= math.Abs(height) * math.Pi * h.Radius * h.Radius
				}
				for _, h := range b.Blind {
					want -= h.Depth * math.Pi * h.Radius * h.Radius
				}
				if math.Abs(got.Volume-want) > 1e-8 || math.Abs(got.Volume-ref.Volume) > 1e-8 || math.Abs(got.Area-ref.Area) > 1e-8 || math.Abs(got.Volume-mass.Volume) > 1e-8 || math.Abs(got.Area-mass.Area) > 1e-8 {
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
}
