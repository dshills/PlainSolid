package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"testing"
)

func TestIndependentExactMaterials(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 4 || m.Members[0].Planar == nil || m.LengthUnit != .001 {
		t.Fatal("classification")
	}
	var sources []*File
	var area, volume float64
	for i, p := range m.Members {
		if i > 0 && p.Elliptical == nil {
			t.Fatal("ellipse")
		}
		if p.SourceBodyID == 0 || p.Source == nil || p.SourceRootID == 0 || p.SourceProductID == 0 || p.SourceDefinitionID == 0 || p.SourceRepresentationID == 0 || p.ProductName == "" {
			t.Fatal("provenance")
		}
		sources = append(sources, p.Source)
		var b bytes.Buffer
		if e = trimmed.WriteJSON(&b, p.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := trimmed.ReadJSON(&b, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.Faces()) != len(p.Solid.Faces()) {
			t.Fatal("native topology")
		}

	}
	volume = 24 + 5*math.Pi*(10+10-.75+3)
	perimeter := func(a, b float64) float64 {
		n := 32768
		h := 2 * math.Pi / float64(n)
		sum := 0.
		for i := 0; i <= n; i++ {
			t := h * float64(i)
			weight := 2.
			if i == 0 || i == n {
				weight = 1
			} else if i%2 == 1 {
				weight = 4
			}
			sum += weight * math.Sqrt(a*a*math.Sin(t)*math.Sin(t)+b*b*math.Cos(t)*math.Cos(t))
		}
		return sum * h / 3
	}
	area = 52 + 2*math.Pi*(10+10-.75+3) + 5*(2*perimeter(5, 2)+perimeter(1.5, .5)+perimeter(3, 1))
	if math.Abs(volume-(24+5*math.Pi*(10+10-.75+3))) > 1e-7 {
		t.Fatal(volume)
	}
	var out bytes.Buffer
	if e = WriteIndependentExactMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	q, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || len(q.Members) != 4 {
		t.Fatal(e)
	}
	if _, e = ReadExactEllipticalPrismCollection(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("legacy widened")
	}
	for _, l := range []Limits{{MaxSolids: 3}, {MaxEntities: 607}, {MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}} {
		v, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), l, 1e-9)
		if e == nil || len(v.Members) != 0 {
			t.Fatal("partial limit")
		}
	}
	for _, which := range []string{"cross-body", "orphan", "unit", "support", "neutral", "product"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if which == "orphan" {
			id := int64(9999)
			f.Entities[id] = Entity{ID: id, Records: []Record{{Type: "CARTESIAN_POINT", Args: []Value{{Kind: String, Text: "orphan"}, {Kind: Aggregate, Items: []Value{{Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}, {Kind: Number, Text: "0"}}}}}}}
			f.Order = append(f.Order, id)
		} else if which == "cross-body" {
			target := m.Members[0].SourceBodyID
			for _, id := range f.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "ADVANCED_BREP_SHAPE_REPRESENTATION" {
					items := en.Records[0].Args[1].Items
					if len(items) == 2 && items[1].Ref == m.Members[1].SourceBodyID {
						items[1].Ref = target
						en.Records[0].Args[1].Items = items
						f.Entities[id] = en
					}
				}
			}
		} else if which == "unit" {
			for _, id := range m.Members[0].Source.Order {
				en := f.Entities[id]
				for j, r := range en.Records {
					if r.Type == "SI_UNIT" && len(r.Args) == 2 && r.Args[1].Text == "METRE" {
						en.Records[j].Args[0] = Value{Kind: Omitted}
						f.Entities[id] = en
					}
				}
			}
		} else if which == "neutral" {
			rep, _ := simple(f, m.Members[0].SourceRepresentationID, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
			frameID := rep.Args[1].Items[0].Ref
			fr, _ := simple(f, frameID, "AXIS2_PLACEMENT_3D", 4)
			id := fr.Args[1].Ref
			en := f.Entities[id]
			en.Records[0].Args[1].Items[0].Text = "1"
			f.Entities[id] = en
		} else if which == "product" {
			id := m.Members[0].SourceDefinitionID
			en := f.Entities[id]
			en.Records[0].Args[2] = Value{Kind: Reference, Ref: m.Members[1].SourceProductID}
			f.Entities[id] = en

		} else {
			for _, id := range m.Members[0].Source.Order {
				en := f.Entities[id]
				if len(en.Records) == 1 && en.Records[0].Type == "PLANE" {
					en.Records[0].Type = "CYLINDRICAL_SURFACE"
					f.Entities[id] = en
					break
				}
			}
		}
		bad, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		v, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(bad), Limits{}, 1e-9)
		if e == nil || len(v.Members) != 0 {
			t.Fatal("mutation", which)
		}
	}
	var reject bytes.Buffer
	if e = WriteIndependentExactMaterials(ctx, &reject, append(sources, nil), Limits{}, 1e-9); e == nil || reject.Len() != 0 {
		t.Fatal("partial write")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_MATERIALS"); oracle != "" {
		for _, b := range [][]byte{raw, out.Bytes()} {
			path := filepath.Join(t.TempDir(), "materials.step")
			os.WriteFile(path, b, 0600)
			data, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatal(string(data), e)
			}
			var r struct {
				Solids, Faces int
				Area, Volume  float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &r) != nil || r.Solids != 4 || r.Faces != 16 || math.Abs(r.Area-area) > 1e-6 || math.Abs(r.Volume-volume) > 1e-6 {
				t.Fatal(string(data), area, volume)
			}
		}
	}
}
func FuzzIndependentExactMaterials(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadIndependentExactMaterials(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16, MaxSolids: 4}, 1e-9)
		if e != nil && len(m.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
