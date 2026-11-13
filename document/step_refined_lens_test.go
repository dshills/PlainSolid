package document

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/exchange/step"
	"reflect"
	"testing"
)

func refinedLensDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/refined-curve-lens.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadRefinedSplineLensSTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestRefinedLensDocument(t *testing.T) {
	d := refinedLensDocument(t)
	s, _ := d.Snapshot()
	recipe := s.Labels["step:body-000001"].Attributes["step.lens.curve.recipe"].Text
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if s.Labels["step:body-000001"].Attributes["step.lens.curve.recipe"].Text != recipe {
		t.Fatal("source recipe changed")
	}
	var b bytes.Buffer
	if e = s.WriteRefinedSplineLensSTEP(&b); e != nil {
		t.Fatal(e)
	}
	got, e := step.ReadRefinedSplineLensBody(bytes.NewReader(b.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
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
			if !reflect.DeepEqual(a.Poles, c.Poles()) || !reflect.DeepEqual(a.Knots, c.Knots()) || !reflect.DeepEqual(a.Weights, c.Weights()) {
				t.Fatal("lost basis")
			}
		}
	}
	b.Reset()
	if e = s.WritePartitionedSplineLensSTEP(&b); e == nil || b.Len() != 0 {
		t.Fatal("old export silently discarded recipe")
	}
	if _, e = FromRefinedSplineLensSTEP(got, 2); e != nil {
		t.Fatal(e)
	}
}
func TestRefinedLensDocumentStale(t *testing.T) {
	for _, kind := range []string{"binding", "model", "upper", "lower", "weight", "geometry", "ownership"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := refinedLensDocument(t).Snapshot()
			l := s.Labels["step:body-000001"]
			var r refinedLensRecipe
			if e := json.Unmarshal([]byte(l.Attributes["step.lens.curve.recipe"].Text), &r); e != nil {
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
			l.Attributes["step.lens.curve.recipe"] = Attribute{Kind: "string", Text: string(raw)}
			if kind != "binding" {
				l.Attributes["step.lens.curve.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
			}
			s.Labels["step:body-000001"] = l
			var b bytes.Buffer
			if e = s.WriteRefinedSplineLensSTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("accepted stale "+kind, e)
			}
		})
	}
}

func TestRefinedLensChangedAssetAfterReload(t *testing.T) {
	s, _ := refinedLensDocument(t).Snapshot()
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
	if e = s.WriteRefinedSplineLensSTEP(&out); e == nil || out.Len() != 0 {
		t.Fatal("reloaded stale asset accepted", e)
	}
}
