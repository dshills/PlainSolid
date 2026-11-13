package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticRoundedRectangleIndependent(t *testing.T) {
	data, e := os.ReadFile("testdata/rounded-stock-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	if b.Height != 3 || len(b.Holes) != 0 || len(b.Blind) != 0 {
		t.Fatal(b)
	}
	var out bytes.Buffer
	if e = WriteRoundedRectangleExtrusions(&out, m); e != nil {
		t.Fatal(e)
	}
}
func TestAnalyticRoundedRectangleNative(t *testing.T) {
	for _, h := range []float64{3, -3} {
		m := roundedSTEPFixture(t, h)
		m.Bodies[0].Holes = nil
		m.Bodies[0].Blind = nil
		var data bytes.Buffer
		if e := WriteRoundedRectangleExtrusions(&data, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadAnalyticRoundedRectangleExtrusions(&data, Limits{})
		if e != nil {
			t.Fatal(h, e)
		}
		if got.Bodies[0].Height != math.Abs(h) {
			t.Fatal(got)
		}
	}
}
func TestAnalyticRoundedRectangleOCCT(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-rounded-stock")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteRoundedRectangleExtrusions(&out, m); e != nil {
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
	if math.Abs(got[0].Volume-3*(40-(1-math.Pi/4)*3.5)) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 || math.Abs(got[0].Area-got[1].Area) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}

func TestAnalyticRoundedRectangleRejects(t *testing.T) {
	data, e := os.ReadFile("testdata/rounded-stock-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"winding", "wall-sense", "radius", "wrapped-quarter", "owner", "vertex", "extra"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(data), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			id := int64(0)
			for _, v := range f.Order {
				en := f.Entities[v]
				if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
					id = v
				}
			}
			g, e := readCircularSourceGraph(f, id)
			if e != nil {
				t.Fatal(e)
			}
			curved := -1
			for i, face := range g.faces {
				if face.kind == "CYLINDER" {
					curved = i
					break
				}
			}
			if curved < 0 {
				t.Fatal("missing curve")
			}
			face := g.faces[curved]
			changed := false
			switch kind {
			case "winding":
				r := f.Entities[face.id].Records[0]
				r.Args[3] = Value{Kind: Enumeration, Text: "F"}
				changed = true
			case "wall-sense":
				for _, p := range g.faces {
					if p.kind == "PLANE" && math.Abs(p.frame.ZAxis().Dot(face.frame.ZAxis())) < .1 {
						r := f.Entities[p.id].Records[0]
						sense, _ := boolean(r.Args[3])
						v := "T"
						if sense {
							v = "F"
						}
						r.Args[3] = Value{Kind: Enumeration, Text: v}
						changed = true
						break
					}
				}
			case "radius":
				r := f.Entities[face.surface].Records[0]
				r.Args[2] = Value{Kind: Number, Text: "0.125"}
				changed = true
			case "wrapped-quarter":
				for _, u := range face.loops[0] {
					edge := g.edges[u.id]
					if edge.kind != "LINE" {
						continue
					}
					for _, pc := range edge.pcs {
						owner, gid, e := circularPCurveGeometry(f, pc)
						if e != nil {
							t.Fatal(e)
						}
						if owner == face.surface {
							r := f.Entities[gid].Records[0]
							pid, _ := ref(r.Args[1])
							p := f.Entities[pid].Records[0]
							v, e := numeric(p.Args[1].Items[0])
							if e != nil {
								t.Fatal(e)
							}
							p.Args[1].Items[0] = Value{Kind: Number, Text: real(v + 2*math.Pi)}
							changed = true
							break
						}
					}
					if changed {
						break
					}
				}
			case "owner":
				edge := g.edges[face.loops[0][0].id]
				r := f.Entities[edge.pcs[0]].Records[0]
				r.Args[1] = Value{Kind: Reference, Ref: id}
				changed = true
			case "vertex":
				edge := g.edges[face.loops[0][0].id]
				r := f.Entities[edge.id].Records[0]
				r.Args[2] = r.Args[1]
				changed = true
			case "extra":
				newID := int64(10000)
				copy := f.Entities[face.surface]
				copy.ID = newID
				f.Entities[newID] = copy
				f.Order = append(f.Order, newID)
				changed = true
			}
			if !changed {
				t.Fatal("missing mutation")
			}
			if _, e = ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted malformed source")
			}
		})
	}
	m := roundedSTEPFixture(t, 3)
	var raw bytes.Buffer
	if e = WriteRoundedRectangleExtrusions(&raw, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadAnalyticRoundedRectangleExtrusions(&raw, Limits{}); e == nil {
		t.Fatal("pockets accepted")
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e = ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(data), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
}
func FuzzReadAnalyticRoundedRectangle(f *testing.F) {
	data, e := os.ReadFile("testdata/rounded-stock-occt.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(data)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}

func TestAnalyticRoundedRectangleCornerPatterns(t *testing.T) {
	for _, radii := range [][4]float64{{.5, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, .75, 0}, {0, 0, 0, 1.5}, {.5, 1, .75, 1.5}} {
		m := roundedSTEPFixture(t, 3)
		m.Bodies[0].Holes = nil
		m.Bodies[0].Blind = nil
		m.Bodies[0].Profile.Radii = radii
		var raw bytes.Buffer
		if e := WriteRoundedRectangleExtrusions(&raw, m); e != nil {
			t.Fatal(e)
		}
		if _, e := ReadAnalyticRoundedRectangleExtrusions(&raw, Limits{}); e != nil {
			t.Fatal(radii, e)
		}
	}
}
