package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"testing"
	"time"
)

func refinedAssemblyDocument(t testing.TB) *Document {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/refined-curve-lens-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ReadRefinedSplineLensAssemblySTEP(bytes.NewReader(raw), step.Limits{}, step.CurvedOvalOptions{}, STEPAssemblyOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func refinedAssemblyRoundtrip(t testing.TB) []byte {
	t.Helper()
	d := refinedAssemblyDocument(t)
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 1 {
		t.Fatal("lost sharing")
	}
	count, rotated := 0, false
	for _, n := range s.Nodes {
		if n.Asset != "" {
			count++
		}
		if n.Name == "Rotated" {
			rotated = n.Transform == (geom.Affine{{0, 0, 1, 8}, {0, 1, 0, 0}, {-1, 0, 0, 4}})
		}
	}
	if count != 2 || !rotated {
		t.Fatal("instances")
	}
	roles := s.Labels["step:body-000001"].Attributes["step.lens.partitioned.provenance"].Text
	var provenance partitionedLensProvenance
	if e := json.Unmarshal([]byte(roles), &provenance); e != nil || len(provenance.Pieces) != 4 {
		t.Fatal("source roles", e)
	}
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	d, e := Read(&native, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = d.Snapshot()
	if len(s.Trimmed) != 1 || s.Labels["step:body-000001"].Attributes["step.lens.partitioned.provenance"].Text != roles {
		t.Fatal("native ancestry")
	}
	var out bytes.Buffer
	if e = s.WriteRefinedSplineLensAssemblySTEP(&out); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadRefinedSplineLensAssembly(bytes.NewReader(out.Bytes()), step.Limits{}, step.CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.Assembly.Occurrences) != 2 || pid != a.Assembly.Occurrences[1].Child || len(a.ProductPieces[pid]) != 4 {
		t.Fatal("export sharing")
	}
	if len(a.ProductCurves[pid]) != 4 {
		t.Fatal("lost source bases")
	}
	for _, curves := range a.ProductCurves[pid] {
		for role, k := range []float64{.75, .5, .25} {
			if len(curves[role].Poles()) != 4 || curves[role].Knots()[3] != k {
				t.Fatal("owner basis")
			}
		}
	}
	return out.Bytes()
}
func TestRefinedLensAssemblyDocument(t *testing.T) { refinedAssemblyRoundtrip(t) }
func TestRefinedLensAssemblyDocumentStale(t *testing.T) {
	for _, kind := range []string{"source", "member", "nonrigid", "lineage"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := refinedAssemblyDocument(t).Snapshot()
			switch kind {
			case "source", "lineage":
				l := s.Labels["step:body-000001"]
				var p partitionedLensProvenance
				json.Unmarshal([]byte(l.Attributes["step.lens.partitioned.provenance"].Text), &p)
				if kind == "source" {
					p.Pieces[0].SourceEdgeID++
				} else {
					p.Pieces[0].Cells[1].UMax = .75
				}
				raw, _ := json.Marshal(p)
				l.Attributes["step.lens.partitioned.provenance"] = Attribute{Kind: "string", Text: string(raw)}
				if kind == "lineage" {
					l.Attributes["step.lens.partitioned.binding"] = Attribute{Kind: "string", Text: lensProvenanceDigest(raw)}
				}
				s.Labels["step:body-000001"] = l
			case "member":
				for id, n := range s.Nodes {
					if n.Asset != "" {
						n.Transform = geom.Translation(geom.Vec3{X: 1})
						s.Nodes[id] = n
						break
					}
				}
			case "nonrigid":
				for id, n := range s.Nodes {
					if n.Name == "Rotated" {
						n.Transform[0][2] = 2
						s.Nodes[id] = n
						break
					}
				}
			}
			var b bytes.Buffer
			if e := s.WriteRefinedSplineLensAssemblySTEP(&b); e == nil || b.Len() != 0 {
				t.Fatal("accepted "+kind, e)
			}
		})
	}
}
func TestOCCTRefinedLensAssemblyDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional STEP oracle")
	}
	raw := refinedAssemblyRoundtrip(t)
	path := t.TempDir() + "/assembly.step"
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	output := t.TempDir() + "/mass.json"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, e := exec.CommandContext(ctx, oracle, output, "step-mass-precise", path).CombinedOutput()
	if e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e = os.ReadFile(output)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	r := 105. / 512
	if math.Abs(mass.Volume-math.Pi*math.Pow(r, 4)) > 1e-9 || math.Abs(mass.Area-8*math.Pi/3*(math.Pow(1+r*r, 1.5)-1)) > 1e-8 {
		t.Fatal(mass)
	}
}
