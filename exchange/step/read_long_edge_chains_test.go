package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestReadLongEdgeRecipes(t *testing.T) {
	for _, setback := range [][2]float64{{1, 1}, {.5, 1.5}, {1, 2}} {
		for _, full := range []bool{false, true} {
			m := longEdgeChainFixture()
			m.Bodies[0].TopSetback, m.Bodies[0].SideSetback = setback[0], setback[1]
			var raw, out bytes.Buffer
			if full {
				if e := WriteLongEdgeChains(&raw, m); e != nil {
					t.Fatal(e)
				}
				got, e := ReadLongEdgeChains(bytes.NewReader(raw.Bytes()), Limits{})
				if e != nil {
					t.Fatal(full, setback, e)
				}
				if got.LengthUnit != m.LengthUnit || len(got.Bodies) != 1 || got.Bodies[0] != m.Bodies[0] {
					t.Fatal(got, m)
				}
				if e = WriteLongEdgeChains(&out, got); e != nil {
					t.Fatal(e)
				}
			} else {
				source := LongEdgeTreatmentModel{LengthUnit: m.LengthUnit, Bodies: []LongEdgeTreatmentBody{m.Bodies[0].LongEdgeTreatmentBody}}
				if e := WriteLongEdgeTreatments(&raw, source); e != nil {
					t.Fatal(e)
				}
				got, e := ReadLongEdgeTreatments(bytes.NewReader(raw.Bytes()), Limits{})
				if e != nil {
					t.Fatal(full, setback, e)
				}
				if got.LengthUnit != source.LengthUnit || len(got.Bodies) != 1 || got.Bodies[0] != source.Bodies[0] {
					t.Fatal(got, source)
				}
				if e = WriteLongEdgeTreatments(&out, got); e != nil {
					t.Fatal(e)
				}
			}
			if !bytes.Equal(raw.Bytes(), out.Bytes()) {
				t.Fatal("canonical graph changed")
			}
		}
	}
}

func TestReadLongEdgeRecipeRejects(t *testing.T) {
	m := longEdgeChainFixture()
	var raw bytes.Buffer
	if e := WriteLongEdgeChains(&raw, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"pcurve", "model-radius", "edge-sense", "bound-sense", "face-order", "owner-clone", "extra", "decimal", "exponent"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
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
				case "pcurve":
					if r.Type == "PCURVE" {
						r.Args[1] = Value{Kind: Reference, Ref: id}
						changed = true
					}
				case "model-radius":
					if r.Type == "CIRCLE" {
						r.Args[2] = Value{Kind: Number, Text: "0.125"}
						changed = true
					}
				case "edge-sense":
					if r.Type == "EDGE_CURVE" {
						sense, _ := boolean(r.Args[4])
						v := "T"
						if sense {
							v = "F"
						}
						r.Args[4] = Value{Kind: Enumeration, Text: v}
						changed = true
					}
				case "bound-sense":
					if r.Type == "FACE_BOUND" {
						sense, _ := boolean(r.Args[2])
						v := "T"
						if sense {
							v = "F"
						}
						r.Args[2] = Value{Kind: Enumeration, Text: v}
						changed = true
					}
				case "face-order":
					if r.Type == "CLOSED_SHELL" {
						r.Args[1].Items[0], r.Args[1].Items[1] = r.Args[1].Items[1], r.Args[1].Items[0]
						changed = true
					}
				case "owner-clone":
					if r.Type == "PCURVE" {
						owner, _ := ref(r.Args[1])
						newID := int64(10000)
						clone := f.Entities[owner]
						clone.ID = newID
						f.Entities[newID] = clone
						f.Order = append(f.Order, newID)
						r.Args[1] = Value{Kind: Reference, Ref: newID}
						changed = true
					}
				case "extra":
					if r.Type == "PLANE" {
						newID := int64(10000)
						f.Entities[newID] = Entity{ID: newID, Records: []Record{r}}
						f.Order = append(f.Order, newID)
						changed = true
					}
				case "decimal":
					if r.Type == "VECTOR" {
						r.Args[2] = Value{Kind: Number, Text: "1.00000000000000000000000000001"}
						changed = true
					}
				case "exponent":
					if r.Type == "VECTOR" {
						r.Args[2] = Value{Kind: Number, Text: "1E9999999999999999999999999"}
						changed = true
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("missing mutation")
			}
			if _, e = ReadLongEdgeChains(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("altered source accepted")
			}
		})
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e := ReadLongEdgeChains(bytes.NewReader(raw.Bytes()), l); e == nil {
			t.Fatal("ignored limit")
		}
	}
	if _, e := ReadLongEdgeTreatments(bytes.NewReader(raw.Bytes()), Limits{}); e == nil {
		t.Fatal("full chain accepted as stock")
	}
}
func TestReadLongEdgeRenumbering(t *testing.T) {
	m := longEdgeChainFixture()
	var raw bytes.Buffer
	if e := WriteLongEdgeChains(&raw, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var rewrite func(Value) Value
	rewrite = func(v Value) Value {
		if v.Kind == Reference {
			v.Ref += 10000
		}
		for i := range v.Items {
			v.Items[i] = rewrite(v.Items[i])
		}
		return v
	}
	entities := map[int64]Entity{}
	for i, id := range f.Order {
		en := f.Entities[id]
		en.ID += 10000
		for j := range en.Records {
			for k := range en.Records[j].Args {
				en.Records[j].Args[k] = rewrite(en.Records[j].Args[k])
			}
		}
		entities[en.ID] = en
		f.Order[i] = en.ID
	}
	f.Entities = entities
	got, e := ReadLongEdgeChains(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Bodies[0] != m.Bodies[0] {
		t.Fatal(got)
	}
}
func FuzzReadLongEdgeRecipes(f *testing.F) {
	var raw bytes.Buffer
	if e := WriteLongEdgeChains(&raw, longEdgeChainFixture()); e != nil {
		f.Fatal(e)
	}
	f.Add(raw.Bytes())
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		limits := Limits{MaxBytes: 128 << 10, MaxEntities: 5000, MaxValues: 40000, MaxVertices: 256, MaxFaces: 64, MaxSolids: 1, MaxEdgeUses: 1024}
		_, _ = ReadLongEdgeChains(bytes.NewReader(data), limits)
		_, _ = ReadLongEdgeTreatments(bytes.NewReader(data), limits)
	})
}

func TestReadLongEdgeOCCT(t *testing.T) {
	for _, full := range []bool{false, true} {
		m := longEdgeChainFixture()
		var raw, out bytes.Buffer
		if full {
			if e := WriteLongEdgeChains(&raw, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadLongEdgeChains(&raw, Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteLongEdgeChains(&out, got); e != nil {
				t.Fatal(e)
			}
		} else {
			source := LongEdgeTreatmentModel{LengthUnit: m.LengthUnit, Bodies: []LongEdgeTreatmentBody{m.Bodies[0].LongEdgeTreatmentBody}}
			if e := WriteLongEdgeTreatments(&raw, source); e != nil {
				t.Fatal(e)
			}
			got, e := ReadLongEdgeTreatments(&raw, Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteLongEdgeTreatments(&out, got); e != nil {
				t.Fatal(e)
			}
		}
		path := filepath.Join(t.TempDir(), "roundtrip.step")
		if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		reference := filepath.Join(t.TempDir(), "independent.step")
		mode := "write-step-long-edge-treatment"
		if full {
			mode = "write-step-long-edge-chain"
		}
		stepOracle(t, reference, mode)
		data, e := os.ReadFile(reference)
		if e != nil {
			t.Fatal(e)
		}
		if full {
			_, e = ReadLongEdgeChains(bytes.NewReader(data), Limits{})
		} else {
			_, e = ReadLongEdgeTreatments(bytes.NewReader(data), Limits{})
		}
		if e == nil {
			t.Fatal("unexpected generic OCCT graph acceptance")
		}
		type measure struct {
			Area, Volume float64
			Centroid     [3]float64
		}
		var got [2]measure
		for i, p := range []string{reference, path} {
			dest := filepath.Join(t.TempDir(), "mass.json")
			stepOracle(t, dest, "step-mass-precise", p)
			data, e := os.ReadFile(dest)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &got[i]); e != nil {
				t.Fatal(e)
			}
		}
		if math.Abs(got[0].Area-got[1].Area) > 1e-7 || math.Abs(got[0].Volume-got[1].Volume) > 1e-7 {
			t.Fatal(full, got)
		}
		for i := 0; i < 3; i++ {
			if math.Abs(got[0].Centroid[i]-got[1].Centroid[i]) > 1e-8 {
				t.Fatal(full, got)
			}
		}
	}
}

func TestReadLongEdgeFeatureParameters(t *testing.T) {
	for _, unit := range []float64{.001, 1} {
		m := longEdgeChainFixture()
		m.LengthUnit = unit
		b := &m.Bodies[0]
		b.Name = "chain 'renamed'"
		b.Bore.Center = geom.Vec2{X: 4.5, Y: 4}
		b.Bore.Radius = .75
		b.Pocket.Min = geom.Vec2{X: 9, Y: 4}
		b.Pocket.Max = geom.Vec2{X: 12, Y: 7}
		b.Boss.Footprint.Min = geom.Vec2{X: 15, Y: 4}
		b.Boss.Footprint.Max = geom.Vec2{X: 17, Y: 6}
		b.Boss.Height = 2.5
		var raw bytes.Buffer
		if e := WriteLongEdgeChains(&raw, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadLongEdgeChains(&raw, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if got.LengthUnit != unit || got.Bodies[0] != *b {
			t.Fatal(got, m)
		}
	}
}
