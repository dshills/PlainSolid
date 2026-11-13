package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/iges"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func igesLensDocument(t *testing.T, name string) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadSplineLensIGES(bytes.NewReader(raw), iges.Limits{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestIGESLensDocumentChain(t *testing.T) {
	for _, name := range []string{"spline-lens", "sheared-spline-lens", "refined-spline-lens"} {
		t.Run(name, func(t *testing.T) {
			source, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			originalBody, e := iges.ReadSplineLensSolid(bytes.NewReader(source), iges.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			d := igesLensDocument(t, name)
			s, _ := d.Snapshot()
			original := s.Labels["iges:body-000001"].Attributes["iges.lens.provenance"].Text
			s.LengthUnit = .0254
			d, e = New(s, 2)
			if e != nil {
				t.Fatal(e)
			}
			var native bytes.Buffer
			if e = d.Write(&native); e != nil {
				t.Fatal(e)
			}
			d, e = Read(&native, 0, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ = d.Snapshot()
			if original != s.Labels["iges:body-000001"].Attributes["iges.lens.provenance"].Text {
				t.Fatal("source recipe lost")
			}
			var out bytes.Buffer
			if e = s.WriteSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
			m, e := iges.ReadSplineLensSolid(bytes.NewReader(out.Bytes()), iges.Limits{})
			if e != nil || m.LengthUnit != .0254 || !reflect.DeepEqual(originalBody.Upper.Data(), m.Upper.Data()) || !reflect.DeepEqual(originalBody.Lower.Data(), m.Lower.Data()) {
				t.Fatal(m, e)
			}
			d, e = FromSplineLensIGES(m, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ = d.Snapshot()
			out.Reset()
			if e = s.WriteSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestIGESLensDocumentReject(t *testing.T) {
	for _, kind := range []string{"source", "cell", "extra-cell", "placement", "instances", "geometry", "descriptor"} {
		t.Run(kind, func(t *testing.T) {
			d := igesLensDocument(t, "refined-spline-lens")
			s, _ := d.Snapshot()
			label := s.Labels["iges:body-000001"]
			raw := label.Attributes["iges.lens.provenance"].Text
			var p igesLensProvenance
			if e := json.Unmarshal([]byte(raw), &p); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "source":
				label.Attributes["iges.source.solid"] = Attribute{Kind: "integer", Integer: 999}
			case "cell":
				p.Cells[1][0] = geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}
			case "extra-cell":
				p.Cells[1] = append(p.Cells[1], p.Cells[1][0])
			case "descriptor":
				p.Shear = .125
			case "placement":
				n := s.Nodes["body-000001"]
				n.Transform[0][3] = 1
				s.Nodes["body-000001"] = n
			case "instances":
				s.Nodes["extra"] = s.Nodes["body-000001"]
			case "geometry":
				d2 := igesLensDocument(t, "sheared-spline-lens")
				s2, _ := d2.Snapshot()
				s.Trimmed["body-000001"] = s2.Trimmed["body-000001"]
			}
			if kind == "cell" || kind == "extra-cell" || kind == "descriptor" {
				b, e := json.Marshal(p)
				if e != nil {
					t.Fatal(e)
				}
				label.Attributes["iges.lens.provenance"] = Attribute{Kind: "string", Text: string(b)}
				label.Attributes["iges.lens.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(b)}
			}
			s.Labels["iges:body-000001"] = label
			var b bytes.Buffer
			if e := s.WriteSplineLensIGES(&b); e == nil || b.Len() != 0 {
				t.Fatal("stale asset emitted", e)
			}
		})
	}
}
func TestIGESLensDocumentOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	d := igesLensDocument(t, "refined-spline-lens")
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var b bytes.Buffer
	if e = s.WriteSplineLensIGES(&b); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "lens.igs")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if log, e := exec.CommandContext(ctx, oracle, out, "iges-solid", path).CombinedOutput(); e != nil {
		t.Fatal(e, string(log))
	}
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Solids, Faces, Edges, Vertices int
		Volume                         float64
	}
	if e = json.Unmarshal(raw, &r); e != nil || !reflect.DeepEqual([]int{r.Solids, r.Faces, r.Edges, r.Vertices}, []int{1, 2, 4, 4}) || math.Abs(r.Volume-math.Pi/512) > 1e-10 {
		t.Fatal(r, e)
	}
}
