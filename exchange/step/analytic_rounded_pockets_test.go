package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticRoundedPocketsNative(t *testing.T) {
	t.Parallel()
	for _, h := range []float64{3, -3} {
		for _, mode := range []string{"stock", "through", "mixed", "blind"} {
			t.Run(mode+real(h), func(t *testing.T) {
				m := roundedSTEPFixture(t, h)
				switch mode {
				case "stock":
					m.Bodies[0].Holes = nil
					m.Bodies[0].Blind = nil
				case "through":
					m.Bodies[0].Blind = nil
				case "blind":
					m.Bodies[0].Holes = nil
				}
				var data bytes.Buffer
				if e := WriteRoundedRectangleExtrusions(&data, m); e != nil {
					t.Fatal(e)
				}
				got, e := ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(data.Bytes()), Limits{})
				if e != nil {
					t.Fatal(e)
				}
				b := got.Bodies[0]
				if len(b.Holes) != len(m.Bodies[0].Holes) || len(b.Blind) != len(m.Bodies[0].Blind) || math.Abs(b.Height) != 3 {
					t.Fatal(b)
				}
				if len(b.Blind) > 0 && b.Height != h {
					t.Fatal("entry lost")
				}
				if mode != "stock" {
					if _, e = ReadAnalyticRoundedRectangleExtrusions(bytes.NewReader(data.Bytes()), Limits{}); e == nil {
						t.Fatal("stock API broadened")
					}
				}
				var out bytes.Buffer
				if e = WriteRoundedRectangleExtrusions(&out, got); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
func TestAnalyticRoundedPocketsIndependent(t *testing.T) {
	t.Parallel()
	data, e := os.ReadFile("testdata/rounded-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	if len(b.Holes) != 1 || len(b.Blind) != 1 || math.Abs(b.Height) != 3 || b.Blind[0].Depth != 1 {
		t.Fatal(b)
	}
}
func TestAnalyticRoundedPocketsOCCT(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-rounded-pocket")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(data), Limits{})
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
	want := 3*(40-(1-math.Pi/4)*3.5) - math.Pi*(.25*3+.5625)
	if math.Abs(got[0].Volume-want) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 || math.Abs(got[0].Area-got[1].Area) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}

func TestAnalyticRoundedPocketsRejects(t *testing.T) {
	t.Parallel()
	data, e := os.ReadFile("testdata/rounded-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"floor-sense", "floor-loop", "floor-plane", "inward-owner", "inner-radius", "inner-seam", "outer-owner", "extra"} {
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
			floor, inner, outer := -1, -1, -1
			for i, face := range g.faces {
				if face.kind == "CYLINDER" {
					if face.sense {
						outer = i
					} else {
						inner = i
					}
				} else if len(face.loops) == 1 && len(face.loops[0]) == 1 {
					floor = i
				}
			}
			if floor < 0 || inner < 0 || outer < 0 {
				t.Fatal("missing fixture role")
			}
			ff, wall := g.faces[floor], g.faces[inner]
			switch kind {
			case "floor-sense":
				r := f.Entities[ff.id].Records[0]
				sense, _ := boolean(r.Args[3])
				v := "T"
				if sense {
					v = "F"
				}
				r.Args[3] = Value{Kind: Enumeration, Text: v}
			case "floor-loop":
				r := f.Entities[ff.bounds[0]].Records[0]
				sense, _ := boolean(r.Args[2])
				v := "T"
				if sense {
					v = "F"
				}
				r.Args[2] = Value{Kind: Enumeration, Text: v}
			case "floor-plane":
				r := f.Entities[ff.surface].Records[0]
				pid, _ := ref(r.Args[1])
				p := f.Entities[pid].Records[0]
				vid, _ := ref(p.Args[1])
				v := f.Entities[vid].Records[0]
				v.Args[1].Items[0] = Value{Kind: Number, Text: "99"}
			case "inward-owner":
				r := f.Entities[wall.id].Records[0]
				r.Args[3] = Value{Kind: Enumeration, Text: "T"}
			case "inner-radius":
				r := f.Entities[wall.surface].Records[0]
				r.Args[2] = Value{Kind: Number, Text: "99"}
			case "inner-seam":
				for _, u := range wall.loops[0] {
					edge := g.edges[u.id]
					if edge.wrapper == "SEAM_CURVE" {
						r := f.Entities[edge.geometry].Records[0]
						r.Args[2].Items[1] = r.Args[2].Items[0]
						break
					}
				}
			case "outer-owner":
				edge := g.edges[g.faces[outer].loops[0][0].id]
				r := f.Entities[edge.pcs[0]].Records[0]
				r.Args[1] = Value{Kind: Reference, Ref: wall.surface}
			case "extra":
				newID := int64(10000)
				copy := f.Entities[ff.surface]
				copy.ID = newID
				f.Entities[newID] = copy
				f.Order = append(f.Order, newID)
			}
			if _, e = ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted malformed source")
			}
		})
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e = ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(data), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
}
func FuzzReadAnalyticRoundedPockets(f *testing.F) {
	data, e := os.ReadFile("testdata/rounded-pocket-occt.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(data)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadAnalyticRoundedPocketExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
