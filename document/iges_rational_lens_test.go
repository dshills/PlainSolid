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
	"reflect"
	"strings"
	"testing"
	"time"
)

func rawIGESDocument(t *testing.T, name string) *Document {
	t.Helper()
	b, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadRationalSplineLensIGES(bytes.NewReader(b), iges.Limits{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestRationalIGESDocumentChain(t *testing.T) {
	for _, name := range []string{"refined-curve-lens", "raw-curve-lens"} {
		t.Run(name, func(t *testing.T) {
			source, e := os.ReadFile("../exchange/iges/testdata/" + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			original, e := iges.ReadRationalSplineLensSolid(bytes.NewReader(source), iges.Limits{})
			if e != nil {
				t.Fatal(e)
			}
			d := rawIGESDocument(t, name)
			s, _ := d.Snapshot()
			recipe := s.Labels["iges:body-000001"].Attributes["iges.lens.rational.recipe"].Text
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
			if recipe != s.Labels["iges:body-000001"].Attributes["iges.lens.rational.recipe"].Text {
				t.Fatal("raw recipe changed")
			}
			var out bytes.Buffer
			if e = s.WriteRationalSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
			got, e := iges.ReadRationalSplineLensSolid(bytes.NewReader(out.Bytes()), iges.Limits{})
			if e != nil || got.Body.LengthUnit != .0254 || !reflect.DeepEqual(got.Curves, original.Curves) || !reflect.DeepEqual(got.Body.Upper.Data(), original.Body.Upper.Data()) || !reflect.DeepEqual(got.Body.Lower.Data(), original.Body.Lower.Data()) {
				t.Fatal("source basis lost", e)
			}
			out.Reset()
			if e = s.WriteSplineLensIGES(&out); e == nil || out.Len() != 0 {
				t.Fatal("ordinary export discarded raw recipe")
			}
			d, e = FromRationalSplineLensIGES(got, 2)
			if e != nil {
				t.Fatal(e)
			}
			s, _ = d.Snapshot()
			if e = s.WriteRationalSplineLensIGES(&out); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestRationalIGESDocumentReject(t *testing.T) {
	for _, kind := range []string{"pole", "weight", "owner", "geometry", "source-binding", "extra-owner", "extra-edge", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			d := rawIGESDocument(t, "raw-curve-lens")
			s, _ := d.Snapshot()
			label := s.Labels["iges:body-000001"]
			var p igesLensRecipe
			if e := json.Unmarshal([]byte(label.Attributes["iges.lens.rational.recipe"].Text), &p); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "pole":
				p.Curves[0][0].Poles[1].Z = math.Nextafter(2, 3)
			case "weight":
				p.Curves[0][1].Weights[0] = 0
			case "owner":
				p.Curves[0][2] = p.Curves[1][1]
			case "geometry":
				d2 := igesLensDocument(t, "sheared-spline-lens")
				s2, _ := d2.Snapshot()
				s.Trimmed["body-000001"] = s2.Trimmed["body-000001"]
			case "source-binding":
				p.Provenance = "wrong"
			case "extra-owner":
				p.Curves[0] = append(p.Curves[0], p.Curves[0][0])
			case "extra-edge":
				p.Curves = append(p.Curves, p.Curves[0])
			case "oversize":
				p.Geometry = strings.Repeat("x", 65537)
			}
			b, e := json.Marshal(p)
			if e != nil {
				t.Fatal(e)
			}
			label.Attributes["iges.lens.rational.recipe"] = Attribute{Kind: "string", Text: string(b)}
			label.Attributes["iges.lens.rational.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(b)}
			s.Labels["iges:body-000001"] = label
			var out bytes.Buffer
			if e = s.WriteRationalSplineLensIGES(&out); e == nil || out.Len() != 0 {
				t.Fatal("stale recipe emitted")
			}
		})
	}
}
func TestRationalIGESDocumentOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	d := rawIGESDocument(t, "raw-curve-lens")
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
	if e = s.WriteRationalSplineLensIGES(&b); e != nil {
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
	if e = json.Unmarshal(raw, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 4 || r.Vertices != 4 || math.Abs(r.Volume-math.Pi*math.Pow(105./512, 4)/2) > 1e-10 {
		t.Fatal(r, e)
	}
}
