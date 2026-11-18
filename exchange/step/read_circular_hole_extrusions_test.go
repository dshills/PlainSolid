package step

import (
	"bytes"
	"math"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestReadCircularExtrusions(t *testing.T) {
	t.Parallel()
	for _, height := range []float64{3, -3} {
		for _, mode := range []int{0, 1, 2} {
			m := circularHoleExtrusionFixture(t, height)
			if mode > 0 {
				m = circularBlindExtrusionFixture(t, height, mode == 2)
			}
			var raw bytes.Buffer
			if e := WriteCircularHoleExtrusions(&raw, m); e != nil {
				t.Fatal(e)
			}
			got, e := ReadCircularHoleExtrusions(bytes.NewReader(raw.Bytes()), Limits{})
			if e != nil {
				t.Fatal(height, mode, e)
			}
			if got.LengthUnit != m.LengthUnit || len(got.Bodies) != 1 || len(got.Bodies[0].Holes) != len(m.Bodies[0].Holes) || len(got.Bodies[0].Blind) != len(m.Bodies[0].Blind) {
				t.Fatal(got)
			}
			if mode > 0 && got.Bodies[0].Height != height {
				t.Fatal("blind entry lost")
			}
			if mode == 0 && got.Bodies[0].Height != math.Abs(height) {
				t.Fatal("through normalization")
			}
			var out bytes.Buffer
			if e = WriteCircularHoleExtrusions(&out, got); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(raw.Bytes(), out.Bytes()) {
				t.Fatal("canonical source graph changed")
			}
		}
	}
}
func TestReadCircularExtrusionRejects(t *testing.T) {
	t.Parallel()
	m := circularBlindExtrusionFixture(t, -3, false)
	var raw bytes.Buffer
	if e := WriteCircularHoleExtrusions(&raw, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"pcurve", "floor", "sense", "owner", "cloned-owner", "duplicate", "extra", "number", "exponent"} {
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
				case "floor":
					if r.Type == "PLANE" {
						r.Args[1] = Value{Kind: Reference, Ref: id}
						changed = true
					}
				case "sense":
					if r.Type == "ADVANCED_FACE" {
						r.Args[3] = Value{Kind: Enumeration, Text: "F"}
						changed = true
					}
				case "owner":
					if r.Type == "EDGE_CURVE" {
						r.Args[2] = r.Args[1]
						changed = true
					}
				case "cloned-owner":
					if r.Type == "PCURVE" {
						owner, _ := ref(r.Args[1])
						newID := int64(len(f.Entities) + 100)
						clone := f.Entities[owner]
						clone.ID = newID
						f.Entities[newID] = clone
						f.Order = append(f.Order, newID)
						r.Args[1] = Value{Kind: Reference, Ref: newID}
						changed = true
					}
				case "duplicate":
					if r.Type == "CLOSED_SHELL" {
						r.Args[1].Items = append(r.Args[1].Items, r.Args[1].Items[0])
						changed = true
					}
				case "extra":
					if r.Type == "CYLINDRICAL_SURFACE" {
						copyID := int64(len(f.Entities) + 100)
						f.Entities[copyID] = Entity{ID: copyID, Records: []Record{r}}
						f.Order = append(f.Order, copyID)
						changed = true
					}
				case "exponent":
					if r.Type == "VECTOR" {
						r.Args[2] = Value{Kind: Number, Text: "1e-999999999"}
						changed = true
					}
				case "number":
					if r.Type == "VECTOR" {
						r.Args[2] = Value{Kind: Number, Text: "1.00000000000000000001"}
						changed = true
					}
				}
				if changed {
					f.Entities[id] = Entity{ID: id, Records: []Record{r}}
					break
				}
			}
			if !changed {
				t.Fatal("mutation absent")
			}
			if _, e = ReadCircularHoleExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
				t.Fatal("altered source accepted", kind)
			}
		})
	}
	for _, l := range []Limits{{MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}, {MaxBytes: 32}} {
		if _, e := ReadCircularHoleExtrusions(bytes.NewReader(raw.Bytes()), l); e == nil {
			t.Fatal("limit ignored")
		}
	}
}
func FuzzReadCircularHoleExtrusions(f *testing.F) {
	var b bytes.Buffer
	// Minimal independently specified rectangle with a single through circle.
	frame := circularExtrusionFuzzFrame()
	if e := WriteCircularHoleExtrusions(&b, frame); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadCircularHoleExtrusions(bytes.NewReader(data), Limits{MaxBytes: 128 << 10, MaxEntities: 4000, MaxValues: 30000, MaxVertices: 256, MaxFaces: 128, MaxSolids: 4, MaxEdgeUses: 1024})
	})
}

func circularExtrusionFuzzFrame() CircularHoleExtrusionModel {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	return CircularHoleExtrusionModel{LengthUnit: .001, Bodies: []CircularHoleExtrusionBody{{Frame: f, Outer: []geom.Vec2{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}, Holes: []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 2, Y: 2}, Radius: .5}}, Height: 2}}}
}

func TestReadCircularExtrusionRenumbering(t *testing.T) {
	t.Parallel()
	m := circularBlindExtrusionFixture(t, -3, false)
	var raw bytes.Buffer
	if e := WriteCircularHoleExtrusions(&raw, m); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&raw, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var remap func(Value) Value
	remap = func(v Value) Value {
		if v.Kind == Reference {
			v.Ref += 10000
		}
		for i := range v.Items {
			v.Items[i] = remap(v.Items[i])
		}
		return v
	}
	entities := map[int64]Entity{}
	for i, id := range f.Order {
		en := f.Entities[id]
		en.ID += 10000
		for k := range en.Records {
			for j := range en.Records[k].Args {
				en.Records[k].Args[j] = remap(en.Records[k].Args[j])
			}
		}
		entities[en.ID] = en
		f.Order[i] = en.ID
	}
	f.Entities = entities
	got, e := ReadCircularHoleExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{})
	if e != nil || len(got.Bodies) != 1 || got.Bodies[0].Height != -3 {
		t.Fatal(got, e)
	}
}

func TestIndependentOCCTCircularExtrusionBoundary(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/circular-pocket-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Parse(bytes.NewReader(raw), Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCircularHoleExtrusions(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("alternate OCCT frames silently normalized")
	}
}
