package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
	"time"
)

func TestSplitCylinderProduct(t *testing.T) {
	d := splitWriterFixture(t)
	var b bytes.Buffer
	if e := WriteSplitCylinderProduct(&b, d); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	m, e := ReadSplitCylinderProduct(&b, Limits{})
	if e != nil || m.Model.Name != d.Name || m.Model.Frame != d.Frame || m.Product == 0 || m.Definition == 0 || m.ShapeDefinition == 0 {
		t.Fatal(m, e)
	}
	if _, e = ReadSplitCylinderModel(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("bare reader broadened")
	}
	if e = WriteSplitCylinderProduct(&b, d); e != nil || !bytes.Equal(raw, b.Bytes()) {
		t.Fatal("unstable", e)
	}
	for _, p := range [][2]string{{"'split-cylinder'", "'foreign'"}, {"'mechanical'", "'electrical'"}, {"'design'", "'other'"}, {".NOT_KNOWN.", ".MADE."}, {"'international standard'", "'unverified'"}, {"PRODUCT('split-cylinder','split ''posed'''", "PRODUCT('split-cylinder','different'"}, {"SHAPE_DEFINITION_REPRESENTATION(", "FOREIGN_SHAPE_DEFINITION_REPRESENTATION("}} {
		x := strings.Replace(string(raw), p[0], p[1], 1)
		if x == string(raw) {
			t.Fatal("missing mutation", p)
		}
		got, e := ReadSplitCylinderProduct(strings.NewReader(x), Limits{})
		if e == nil || got.Product != 0 || got.Model.Body != 0 {
			t.Fatal("accepted", p, e)
		}
	}
	var bare bytes.Buffer
	_ = WriteSplitCylinderModel(&bare, d)
	if _, e = ReadSplitCylinderProduct(&bare, Limits{}); e == nil {
		t.Fatal("absent envelope")
	}
}
func TestSplitCylinderProductOCCTRoots(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPLIT_CYLINDER")
	if bin == "" {
		t.Skip("optional OCCT")
	}
	for _, native := range []bool{false, true} {
		var b bytes.Buffer
		d := splitWriterFixture(t)
		if native {
			solid, e := parametric.Cylinder(d.Frame, d.Radius, d.Height)
			if e != nil {
				t.Fatal(e)
			}
			if e = WriteNativeSplitCylinderProduct(&b, solid, d.Name, d.LengthUnit); e != nil {
				t.Fatal(e)
			}
		} else if e := WriteSplitCylinderProduct(&b, d); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "product.step")
		if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		raw, e := exec.CommandContext(ctx, bin, p, "roots").CombinedOutput()
		if e != nil {
			t.Fatal(e, string(raw))
		}
		var v struct {
			Faces        int
			Volume, Area float64
			Center       [3]float64
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e, string(raw))
		}
		if v.Faces != 4 || math.Abs(v.Volume-12*math.Pi) > 1e-10 || math.Abs(v.Area-20*math.Pi) > 1e-10 {
			t.Fatal(v)
		}
		for i, x := range []float64{8.5, -4, 7} {
			if math.Abs(v.Center[i]-x) > 1e-10 {
				t.Fatal(v)
			}
		}
	}
}

func TestSplitCylinderProductReferences(t *testing.T) {
	var b bytes.Buffer
	if e := WriteSplitCylinderProduct(&b, splitWriterFixture(t)); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	for _, mode := range []string{"wrong-representation", "cycle", "duplicate-product", "missing-context"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		ids := map[string]int64{}
		for id, ent := range f.Entities {
			if len(ent.Records) == 1 {
				ids[ent.Records[0].Type] = id
			}
		}
		switch mode {
		case "wrong-representation":
			id := ids["SHAPE_DEFINITION_REPRESENTATION"]
			ent := f.Entities[id]
			ent.Records[0].Args[1].Ref = ids["MANIFOLD_SOLID_BREP"]
			f.Entities[id] = ent
		case "cycle":
			id := ids["PRODUCT_DEFINITION"]
			ent := f.Entities[id]
			ent.Records[0].Args[2].Ref = id
			f.Entities[id] = ent
		case "duplicate-product":
			f.Entities[900] = f.Entities[ids["PRODUCT"]]
			f.Order = append(f.Order, 900)
		case "missing-context":
			delete(f.Entities, ids["MECHANICAL_CONTEXT"])
			order := []int64{}
			for _, id := range f.Order {
				if id != ids["MECHANICAL_CONTEXT"] {
					order = append(order, id)
				}
			}
			f.Order = order
		}
		data := splitProductFile(f, nil, nil)
		if m, e := ReadSplitCylinderProduct(bytes.NewReader(data), Limits{}); e == nil || m.Product != 0 || m.Model.Body != 0 {
			t.Fatal("accepted", mode, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 10}, {MaxBytes: 32}, {MaxFaces: 3}} {
		if _, e := ReadSplitCylinderProduct(bytes.NewReader(raw), l); e == nil {
			t.Fatal("accepted limit", l)
		}
	}
}
