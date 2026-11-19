package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestAnalyticDiskNativeSigned(t *testing.T) {
	t.Parallel()
	for _, height := range []float64{3, -3} {
		for _, mode := range []string{"solid", "through", "mixed", "blind", "opposed"} {
			t.Run(mode+real(height), func(t *testing.T) {
				m := diskExtrusionFixture(t, height)
				switch mode {
				case "solid":
					m.Bodies[0].Holes = nil
					m.Bodies[0].Blind = nil
				case "through":
					m.Bodies[0].Blind = nil
				case "blind":
					m.Bodies[0].Holes = nil
				case "opposed":
					m.Bodies[0].Blind = append(m.Bodies[0].Blind, trimmed.ExtrusionBlindHole{
						ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 1, Y: 0}, Radius: .5},
						Depth:           2, OppositeCap: true,
					})
				}
				var data bytes.Buffer
				if e := WriteDiskHoleExtrusions(&data, m); e != nil {
					t.Fatal(e)
				}
				got, e := ReadAnalyticDiskHoleExtrusions(&data, Limits{})
				if e != nil {
					t.Fatal(e)
				}
				b := got.Bodies[0]
				if len(b.Holes) != len(m.Bodies[0].Holes) || len(b.Blind) != len(m.Bodies[0].Blind) || b.Outer.Radius != 4 || math.Abs(b.Height) != 3 {
					t.Fatal(b)
				}
				if len(b.Blind) > 0 && b.Height != height {
					t.Fatal("entry lost")
				}
				if mode == "opposed" && (b.Blind[0].OppositeCap || !b.Blind[1].OppositeCap || b.Blind[1].Depth != 2) {
					t.Fatal("opposite entry lost", b.Blind)
				}
				var out bytes.Buffer
				if e = WriteDiskHoleExtrusions(&out, got); e != nil {
					t.Fatal(e)
				}
				if _, e = ReadDiskHoleExtrusions(&out, Limits{}); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
func TestAnalyticDiskIndependent(t *testing.T) {
	t.Parallel()
	data, e := os.ReadFile("testdata/disk-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticDiskHoleExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	b := m.Bodies[0]
	if len(b.Holes) != 1 || len(b.Blind) != 1 || b.Outer.Radius != 4 || b.Height != 3 || b.Blind[0].Depth != 1.25 {
		t.Fatal(b)
	}
	if _, e = ReadAnalyticCircularPocketExtrusions(bytes.NewReader(data), Limits{}); e == nil {
		t.Fatal("polygon API broadened")
	}
}
func TestAnalyticDiskOCCT(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-disk-pocket")
	data, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAnalyticDiskHoleExtrusions(bytes.NewReader(data), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteDiskHoleExtrusions(&out, m); e != nil {
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
	if math.Abs(got[0].Volume-math.Pi*(16*3-.5625*3-1.25)) > 1e-8 || math.Abs(got[0].Volume-got[1].Volume) > 1e-8 || math.Abs(got[0].Area-got[1].Area) > 1e-8 {
		t.Fatal(got)
	}
	for i := 0; i < 3; i++ {
		if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
			t.Fatal(got)
		}
	}
}

func TestAnalyticDiskRejects(t *testing.T) {
	t.Parallel()
	data, e := os.ReadFile("testdata/disk-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"outer-inward", "two-outer", "outer-radius", "outer-rim", "cap-sense", "floor-sense", "seam", "pcurve", "extra"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(data), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			id := int64(0)
			for _, candidate := range f.Order {
				en := f.Entities[candidate]
				if len(en.Records) == 1 && en.Records[0].Type == "MANIFOLD_SOLID_BREP" {
					id = candidate
				}
			}
			g, e := readCircularSourceGraph(f, id)
			if e != nil {
				t.Fatal(e)
			}
			outer := -1
			inner := -1
			floor := -1
			for i, face := range g.faces {
				if face.kind == "CYLINDER" {
					if face.sense {
						outer = i
					} else {
						inner = i
					}
				} else if len(face.loops) == 1 {
					floor = i
				}
			}
			if outer < 0 || inner < 0 || floor < 0 {
				t.Fatal("fixture roles absent")
			}
			wall := g.faces[outer]
			var rim circularSourceEdge
			var seam circularSourceEdge
			for _, u := range wall.loops[0] {
				edge := g.edges[u.id]
				if edge.kind == "CIRCLE" {
					rim = edge
				} else {
					seam = edge
				}
			}
			owners := g.owners[rim.id]
			cap := owners[0]
			if cap == outer {
				cap = owners[1]
			}
			switch kind {
			case "outer-inward":
				r := f.Entities[wall.id].Records[0]
				r.Args[3] = Value{Kind: Enumeration, Text: "F"}
			case "two-outer":
				r := f.Entities[g.faces[inner].id].Records[0]
				r.Args[3] = Value{Kind: Enumeration, Text: "T"}
			case "outer-radius":
				r := f.Entities[wall.surface].Records[0]
				r.Args[2] = Value{Kind: Number, Text: "0.25"}
			case "outer-rim":
				r := f.Entities[rim.id].Records[0]
				r.Args[2] = Value{Kind: Reference, Ref: seam.a}
				if seam.a == rim.a {
					r.Args[2] = Value{Kind: Reference, Ref: seam.b}
				}
			case "cap-sense", "floor-sense":
				i := cap
				if kind == "floor-sense" {
					i = floor
				}
				r := f.Entities[g.faces[i].id].Records[0]
				sense, _ := boolean(r.Args[3])
				v := "T"
				if sense {
					v = "F"
				}
				r.Args[3] = Value{Kind: Enumeration, Text: v}
			case "seam":
				r := f.Entities[seam.geometry].Records[0]
				r.Args[2].Items[1] = r.Args[2].Items[0]
			case "pcurve":
				r := f.Entities[rim.pcs[0]].Records[0]
				r.Args[1] = Value{Kind: Reference, Ref: g.faces[inner].surface}
			case "extra":
				newID := int64(10000)
				copy := f.Entities[wall.surface]
				copy.ID = newID
				f.Entities[newID] = copy
				f.Order = append(f.Order, newID)
			}
			if _, e = ReadAnalyticDiskHoleExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("accepted malformed source")
			}
		})
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e = ReadAnalyticDiskHoleExtrusions(bytes.NewReader(data), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
}
func FuzzReadAnalyticDiskHoles(f *testing.F) {
	data, e := os.ReadFile("testdata/disk-pocket-occt.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(data)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadAnalyticDiskHoleExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}
