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

func TestWrappedBoreDocumentRecovery(t *testing.T) {
	for _, name := range []string{"wrapped-frustum", "wrapped-tube", "wrapped-blind-bore", "wrapped-cavity", "wrapped-shoulder", "wrapped-toroidal-void"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("../exchange/step/testdata/" + name + ".step")
			if e != nil {
				t.Fatal(e)
			}
			m, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			d, e := FromWrappedMaterialsSTEP(m, 1)
			if e != nil {
				t.Fatal(e)
			}
			var native bytes.Buffer
			if e = d.Write(&native); e != nil {
				t.Fatal(e)
			}
			d, e = Read(&native, 0, 1)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := d.Snapshot()
			if name != "wrapped-frustum" && name != "wrapped-shoulder" && name != "wrapped-toroidal-void" {
				p, e := s.Trimmed["body-000001"].ValidateCanonicalWrappedBore(context.Background(), geom.SurfacePatchOptions{})
				if e != nil || !p.Complete {
					t.Fatal("proof", e)
				}
			}
			s.Nodes = map[string]Node{"a": {Name: "Moved", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 16, Z: 8})}}
			var out bytes.Buffer
			if e = s.WriteWrappedMaterialsSTEP(&out); e != nil {
				t.Fatal(e)
			}
			got, e := step.ReadWrappedMaterials(&out, step.Limits{}, step.WrappedSurfaceOptions{})
			if e != nil || len(got.Bodies) != 1 {
				t.Fatal(e)
			}
			bad := s.Labels["step:body-000001"]
			bad.Attributes["step.wrapped.family"] = Attribute{Kind: "string", Text: "cylinder"}
			s.Labels["step:body-000001"] = bad
			var rejected bytes.Buffer
			if e = s.WriteWrappedMaterialsSTEP(&rejected); e == nil || rejected.Len() != 0 {
				t.Fatal("wrong family accepted", e)
			}
		})
	}
}

func TestOCCTWrappedBoreDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	for _, name := range []string{"wrapped-frustum", "wrapped-tube", "wrapped-blind-bore", "wrapped-cavity", "wrapped-shoulder", "wrapped-toroidal-void"} {
		t.Run(name, func(t *testing.T) {
			source := "../exchange/step/testdata/" + name + ".step"
			raw, e := os.ReadFile(source)
			if e != nil {
				t.Fatal(e)
			}
			m, e := step.ReadWrappedMaterials(bytes.NewReader(raw), step.Limits{}, step.WrappedSurfaceOptions{})
			if e != nil {
				t.Fatal(e)
			}
			d, e := FromWrappedMaterialsSTEP(m, 1)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := d.Snapshot()
			s.LengthUnit = .001
			s.Nodes = map[string]Node{"one": {Name: "Moved body", Asset: "body-000001", Transform: geom.Translation(geom.Vec3{X: 16, Z: 8})}}
			var b bytes.Buffer
			if e = s.WriteWrappedMaterialsSTEP(&b); e != nil {
				t.Fatal(e)
			}
			path := t.TempDir() + "/out.step"
			if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			var results [2]struct {
				Area, Volume float64
				Centroid     [3]float64
			}
			for i, p := range []string{source, path} {
				out := t.TempDir() + "/mass.json"
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				log, e := exec.CommandContext(ctx, oracle, out, "step-mass-precise", p).CombinedOutput()
				cancel()
				if e != nil {
					t.Fatalf("%v %s", e, log)
				}
				data, e := os.ReadFile(out)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(data, &results[i]); e != nil {
					t.Fatal(e)
				}
			}
			a, bm := results[0], results[1]
			if math.Abs(a.Volume-bm.Volume) > 1e-7 || math.Abs(a.Area-bm.Area) > 1e-7 || math.Abs(bm.Centroid[0]-a.Centroid[0]-16) > 1e-7 || math.Abs(bm.Centroid[2]-a.Centroid[2]-8) > 1e-7 {
				t.Fatal(results)
			}
		})
	}
}
