package document

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/exchange/step"
	"reflect"
	"strings"
	"testing"
)

func rationalLensDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/concurrent-rational-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadRationalSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestRationalLensDocument(t *testing.T) {
	d := rationalLensDocument(t)
	s, _ := d.Snapshot()
	recipe := s.Labels["step:body-000001"].Attributes["step.lens.rational.recipe"].Text
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if s.Labels["step:body-000001"].Attributes["step.lens.rational.recipe"].Text != recipe {
		t.Fatal("source recipe changed")
	}
	var b bytes.Buffer
	if e = s.WriteRationalSplineLensSTEP(&b); e != nil {
		t.Fatal(e)
	}
	got, e := step.ReadRationalSplineLensBody(bytes.NewReader(b.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var r refinedLensRecipe
	if e = json.Unmarshal([]byte(recipe), &r); e != nil {
		t.Fatal(e)
	}
	for i, cs := range got.Curves {
		for role, c := range cs {
			a := r.Curves[i][role]
			if !reflect.DeepEqual(a.Poles, c.Poles) || !reflect.DeepEqual(a.Knots, c.Knots) || !reflect.DeepEqual(a.Weights, c.Weights) {
				t.Fatal("lost basis")
			}
		}
	}
	b.Reset()
	if e = s.WritePartitionedSplineLensSTEP(&b); e == nil || b.Len() != 0 {
		t.Fatal("old export silently discarded recipe")
	}
	if _, e = FromRationalSplineLensSTEP(got, 2); e != nil {
		t.Fatal(e)
	}
}
func TestRationalLensDocumentStale(t *testing.T) {
	for _, kind := range []string{"binding", "model", "upper", "lower", "weight", "geometry", "ownership"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := rationalLensDocument(t).Snapshot()
			l := s.Labels["step:body-000001"]
			var r refinedLensRecipe
			if e := json.Unmarshal([]byte(l.Attributes["step.lens.rational.recipe"].Text), &r); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "binding", "model":
				r.Curves[0][0].Poles[1].X = math.Nextafter(r.Curves[0][0].Poles[1].X, math.Inf(1))
			case "upper":
				r.Curves[0][1].Poles[1].X = math.Nextafter(r.Curves[0][1].Poles[1].X, math.Inf(1))
			case "lower":
				r.Curves[0][2].Poles[1].X = math.Nextafter(r.Curves[0][2].Poles[1].X, math.Inf(1))
			case "weight":
				r.Curves[0][1].Weights[0] = 3
			case "geometry":
				r.Geometry = "stale"
			case "ownership":
				r.Provenance = "stale"
			}
			raw, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			l.Attributes["step.lens.rational.recipe"] = Attribute{Kind: "string", Text: string(raw)}
			if kind != "binding" {
				l.Attributes["step.lens.rational.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
			}
			s.Labels["step:body-000001"] = l
			var b bytes.Buffer
			if e = s.WriteRationalSplineLensSTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("accepted stale "+kind, e)
			}
		})
	}
}

func TestRationalLensChangedAssetAfterReload(t *testing.T) {
	s, _ := rationalLensDocument(t).Snapshot()
	other, _ := lensDocument(t).Snapshot()
	s.Trimmed["body-000001"] = other.Trimmed["body-000001"]
	d, e := New(s, 2)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = d.Write(&b); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&b, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteRationalSplineLensSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("reloaded stale asset accepted", e)
	}
}

func TestRationalLensDocumentBoundsAndStyle(t *testing.T) {
	for _, kind := range []string{"recipe-bound", "bad-color", "extra-face-color"} {
		s, _ := rationalLensDocument(t).Snapshot()
		l := s.Labels["step:body-000001"]
		if kind == "recipe-bound" {
			raw := strings.Repeat(" ", 65537)
			l.Attributes["step.lens.rational.recipe"] = Attribute{Kind: "string", Text: raw}
			l.Attributes["step.lens.rational.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest([]byte(raw))}
		} else if kind == "extra-face-color" {
			l.Attributes["step.lens.rational.style"] = Attribute{Kind: "string", Text: `{"FaceColors":[null,null,{"R":1,"G":0,"B":0}]}`}
		} else {
			l.Attributes["step.lens.rational.style"] = Attribute{Kind: "string", Text: `{"SurfaceColor":{"R":2,"G":0,"B":0}}`}
		}
		s.Labels["step:body-000001"] = l
		var b bytes.Buffer
		if e := s.WriteRationalSplineLensSTEP(&b); e == nil || b.Len() != 0 {
			t.Fatal(kind, e)
		}
	}
}

func TestRationalLensRejectsExtraOwnerRecipe(t *testing.T) {
	s, _ := rationalLensDocument(t).Snapshot()
	l := s.Labels["step:body-000001"]
	var r refinedLensRecipe
	json.Unmarshal([]byte(l.Attributes["step.lens.rational.recipe"].Text), &r)
	r.Curves[0] = append(r.Curves[0], r.Curves[0][0])
	raw, _ := json.Marshal(r)
	l.Attributes["step.lens.rational.recipe"] = Attribute{Kind: "string", Text: string(raw)}
	l.Attributes["step.lens.rational.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
	s.Labels["step:body-000001"] = l
	var b bytes.Buffer
	if e := s.WriteRationalSplineLensSTEP(&b); e == nil || b.Len() != 0 {
		t.Fatal("extra curve owner discarded", e)
	}
}
