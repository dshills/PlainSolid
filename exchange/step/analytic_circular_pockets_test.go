package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticCircularPocketIndependent(t *testing.T) {
	data, e := os.ReadFile("testdata/circular-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticCircularPocketExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	if len(b.Holes) != 1 || len(b.Blind) != 1 || b.Height != 3 || b.Blind[0].Depth != 1.25 || b.Blind[0].Radius != .75 {
		t.Fatal(b)
	}
	var out bytes.Buffer
	if e = WriteCircularHoleExtrusions(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCircularHoleExtrusions(&out, Limits{}); e != nil {
		t.Fatal(e)
	}
}
func TestAnalyticCircularPocketSigned(t *testing.T) {
	for _, h := range []float64{3, -3} {
		for _, all := range []bool{false, true} {
			m := circularBlindExtrusionFixture(t, h, all)
			var out bytes.Buffer
			if e := WriteCircularHoleExtrusions(&out, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadAnalyticCircularPocketExtrusions(bytes.NewReader(out.Bytes()), Limits{})
			if e != nil {
				t.Fatal(h, all, e)
			}
			b := got.Bodies[0]
			if b.Height != h || len(b.Blind) != len(m.Bodies[0].Blind) || len(b.Holes) != len(m.Bodies[0].Holes) {
				t.Fatal(b)
			}
			for i := range b.Blind {
				if b.Blind[i].Depth != m.Bodies[0].Blind[i].Depth {
					t.Fatal(b)
				}
			}
			if _, e = ReadAnalyticCircularThroughExtrusions(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
				t.Fatal("through API broadened")
			}
		}
	}
}
func TestAnalyticCircularPocketRejects(t *testing.T) {
	data, e := os.ReadFile("testdata/circular-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"floor-sense", "floor-loop", "floor-plane", "floor-owner", "floor-vertex", "parameter-scale"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(data), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			g, e := readCircularSourceGraph(f, 15)
			if e != nil {
				t.Fatal(e)
			}
			var floor circularSourceFace
			for _, face := range g.faces {
				if face.kind == "PLANE" && len(face.loops) == 1 && len(face.loops[0]) == 1 {
					floor = face
				}
			}
			if floor.id == 0 {
				t.Fatal("floor absent")
			}
			r := f.Entities[floor.id].Records[0]
			edge := g.edges[floor.loops[0][0].id]
			switch kind {
			case "parameter-scale":
				for _, pc := range edge.pcs {
					_, gid, err := circularPCurveGeometry(f, pc)
					if err != nil {
						t.Fatal(err)
					}
					gr := f.Entities[gid].Records[0]
					if gr.Type == "LINE" {
						vid, _ := ref(gr.Args[2])
						v := f.Entities[vid].Records[0]
						v.Args[2] = Value{Kind: Number, Text: "2"}
					}
				}
			case "floor-sense":
				r.Args[3] = Value{Kind: Enumeration, Text: "T"}
			case "floor-loop":
				br := f.Entities[floor.bounds[0]].Records[0]
				sense, _ := boolean(br.Args[2])
				text := "T"
				if sense {
					text = "F"
				}
				br.Args[2] = Value{Kind: Enumeration, Text: text}
			case "floor-plane":
				pr := f.Entities[floor.surface].Records[0]
				pid, _ := ref(pr.Args[1])
				p := f.Entities[pid].Records[0]
				vid, _ := ref(p.Args[1])
				v := f.Entities[vid].Records[0]
				v.Args[1].Items[0] = Value{Kind: Number, Text: "12"}
			case "floor-owner":
				for _, pc := range edge.pcs {
					p := f.Entities[pc].Records[0]
					owner, _ := ref(p.Args[1])
					if owner == floor.surface {
						p.Args[1] = Value{Kind: Reference, Ref: 126}
					}
				}
			case "floor-vertex":
				v := f.Entities[edge.a].Records[0]
				v.Args[1] = Value{Kind: Reference, Ref: edge.a}
			}
			if _, e = ReadAnalyticCircularPocketExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted invalid floor")
			}
		})
	}
}
func TestAnalyticCircularPocketOCCT(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-circular-pocket")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticCircularPocketExtrusions(bytes.NewReader(data), Limits{})
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
	if math.Abs(got[0].Volume-(3*28.5-math.Pi*(.25*3+.5625*1.25))) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 || math.Abs(got[0].Area-got[1].Area) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}
func FuzzReadAnalyticCircularPockets(f *testing.F) {
	data, e := os.ReadFile("testdata/circular-pocket-occt.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(data)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadAnalyticCircularPocketExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}

func TestAnalyticCircularPocketCommonEntryGuard(t *testing.T) {
	// Exercise the role proof independently of pcurve rejection: relocate only the
	// second source rim's declared cap owner. Mixed entries cannot be normalized
	// into this descriptor even if subsequent geometric checks might be repaired.
	m := circularBlindExtrusionFixture(t, 3, true)
	var raw bytes.Buffer
	if e := WriteCircularHoleExtrusions(&raw, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	body := int64(0)
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			body = id
		}
	}
	g, e := readCircularSourceGraph(f, body)
	if e != nil {
		t.Fatal(e)
	}
	cylinders := []int{}
	caps := [2]int{-1, -1}
	floors := map[int]bool{}
	outer := map[int]int{}
	for i, face := range g.faces {
		if face.kind == "CYLINDER" {
			cylinders = append(cylinders, i)
			continue
		}
		if len(face.loops) == 1 && len(face.loops[0]) == 1 {
			floors[i] = true
			continue
		}
		outer[i] = 0
		n := face.frame.ZAxis()
		if !face.sense {
			n = n.Scale(-1)
		}
		d := n.Dot(m.Bodies[0].Frame.ZAxis())
		if d > 0.9 {
			caps[1] = i
		} else if d < -0.9 {
			caps[0] = i
		}
	}
	if len(cylinders) != 2 || caps[0] < 0 || caps[1] < 0 {
		t.Fatal("unexpected fixture")
	}
	ci := cylinders[1]
	changed := false
	for _, u := range g.faces[ci].loops[0] {
		edge := g.edges[u.id]
		if edge.kind != "CIRCLE" {
			continue
		}
		owners := g.owners[u.id]
		for i, owner := range owners {
			if owner == caps[0] {
				owners[i] = caps[1]
				changed = true
			}
		}
	}
	if !changed {
		t.Fatal("entry absent")
	}
	out := m.Bodies[0]
	out.Holes = nil
	out.Blind = nil
	if e = collectCircularSourcePockets(g, cylinders, caps, outer, floors, out.Frame, 3, 1e-8, &out); e == nil {
		t.Fatal("opposite entry pockets accepted")
	}
}
