package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestCircularSourceIndependentPCurves(t *testing.T) {
	r, e := os.Open("testdata/circular-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	f, e := Parse(r, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := readCircularSourceGraph(f, 15); e != nil {
		t.Fatal(e)
	}
	faces := map[int64]circularSourceFace{}
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_FACE" {
			face, e := readCircularSourceFace(f, id)
			if e != nil {
				t.Fatal(id, e)
			}
			faces[face.surface] = face
		}
	}
	checked := 0
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "EDGE_CURVE" {
			continue
		}
		edge, e := readCircularSourceEdge(f, id)
		if e != nil {
			t.Fatal(id, e)
		}
		for _, pc := range edge.pcs {
			owner, _, e := circularPCurveGeometry(f, pc)
			if e != nil {
				t.Fatal(pc, e)
			}
			if _, e = verifyCircularPCurve(f, pc, faces[owner], edge, 1e-8); e != nil {
				t.Fatalf("edge %d pc %d owner %d: %v", id, pc, owner, e)
			}
			checked++
		}
	}
	if checked != 42 {
		t.Fatal(checked)
	}
}

func TestReadIndependentCircularThrough(t *testing.T) {
	r, e := os.Open("testdata/circular-through-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	m, e := ReadAnalyticCircularThroughExtrusions(r, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].Holes) != 2 || m.Bodies[0].Height != 3 {
		t.Fatal(m)
	}
}

func TestAnalyticCircularThroughRejects(t *testing.T) {
	data, e := os.ReadFile("testdata/circular-through-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"owner", "seam", "radius", "sense", "extra", "vertex", "pcurve", "circle-pcurve"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(data), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			changed := false
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				switch kind {
				case "owner":
					if r.Type == "PCURVE" {
						r.Args[1] = Value{Kind: Reference, Ref: id}
						changed = true
					}
				case "seam":
					if r.Type == "SEAM_CURVE" {
						r.Args[2].Items[1] = r.Args[2].Items[0]
						changed = true
					}
				case "radius":
					if r.Type == "CYLINDRICAL_SURFACE" {
						r.Args[2] = Value{Kind: Number, Text: "0.6"}
						changed = true
					}
				case "sense":
					if r.Type == "ADVANCED_FACE" {
						r.Args[3] = Value{Kind: Enumeration, Text: "T"}
						changed = true
					}
				case "extra":
					if r.Type == "CIRCLE" {
						newID := int64(10000)
						f.Entities[newID] = Entity{ID: newID, Records: []Record{r}}
						f.Order = append(f.Order, newID)
						changed = true
					}
				case "vertex":
					if r.Type == "EDGE_CURVE" {
						r.Args[2] = r.Args[1]
						changed = true
					}
				case "circle-pcurve":
					if r.Type == "PCURVE" {
						_, gid, err := circularPCurveGeometry(f, id)
						if err != nil {
							t.Fatal(err)
						}
						gr := f.Entities[gid].Records[0]
						if gr.Type == "CIRCLE" {
							gr.Args[2] = Value{Kind: Number, Text: "0.625"}
							changed = true
						}
					}
				case "pcurve":
					if r.Type == "PCURVE" {
						_, gid, e := circularPCurveGeometry(f, id)
						if e != nil {
							t.Fatal(e)
						}
						gr := f.Entities[gid].Records[0]
						if gr.Type == "LINE" {
							pid, _ := ref(gr.Args[1])
							pr := f.Entities[pid].Records[0]
							pr.Args[1].Items[0] = Value{Kind: Number, Text: "12"}
							changed = true
						}
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("missing mutation")
			}
			if _, e = ReadAnalyticCircularThroughExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted mutation")
			}
		})
	}
	blind, e := os.ReadFile("testdata/circular-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadAnalyticCircularThroughExtrusions(bytes.NewReader(blind), Limits{}); e == nil {
		t.Fatal("blind floor accepted")
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e = ReadAnalyticCircularThroughExtrusions(bytes.NewReader(data), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
}
func TestAnalyticCircularThroughCanonicalSigned(t *testing.T) {
	for _, h := range []float64{3, -3} {
		var b bytes.Buffer
		if e := WriteCircularHoleExtrusions(&b, circularHoleExtrusionFixture(t, h)); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadAnalyticCircularThroughExtrusions(&b, Limits{}); e != nil {
			t.Fatal(h, e)
		}
	}
}
func TestAnalyticCircularThroughOCCT(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-circular-through")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticCircularThroughExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteCircularHoleExtrusions(&out, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "rewritten.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	type measure struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	var got [2]measure
	for i, p := range []string{source, path} {
		dest := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, dest, "step-mass-precise", p)
		raw, e := os.ReadFile(dest)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &got[i]); e != nil {
			t.Fatal(e)
		}
	}
	if math.Abs(got[0].Volume-3*(28.5-math.Pi*(.25+.5625))) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 || math.Abs(got[0].Area-got[1].Area) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}
func FuzzReadAnalyticCircularThrough(f *testing.F) {
	data, e := os.ReadFile("testdata/circular-through-occt.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(data)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadAnalyticCircularThroughExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
