package document

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func splineAssemblyDocument(t *testing.T) (*Document, step.SplineShellAssembly) {
	t.Helper()
	raw, e := os.ReadFile("../exchange/step/testdata/spline-shell-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := step.ReadSplineShellAssembly(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := FromSplineShellAssemblySTEP(m, STEPAssemblyOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	return d, m
}
func TestSplineAssemblyDocumentRoundtrip(t *testing.T) {
	d, m := splineAssemblyDocument(t)
	s, _ := d.Snapshot()
	if len(s.Trimmed) != 1 || len(s.Nodes) != 3 || len(s.Assets) != 0 || s.LengthUnit != m.Assembly.LengthUnit {
		t.Fatal(s)
	}
	var native bytes.Buffer
	if e := d.Write(&native); e != nil {
		t.Fatal(e)
	}
	restored, e := Read(bytes.NewReader(native.Bytes()), 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := restored.Snapshot()
	if !reflect.DeepEqual(s.Nodes, r.Nodes) || !reflect.DeepEqual(s.Labels, r.Labels) || len(r.Trimmed) != 1 {
		t.Fatal("lost assembly state")
	}
	var out bytes.Buffer
	if e = r.WriteSplineShellAssemblySTEP(&out, "Export root"); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadSplineShellAssembly(&out, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bs := range a.ProductBodies {
		count += len(bs)
	}
	if count != 1 || len(a.Assembly.Occurrences) != 3 || len(a.Assembly.Products) != 3 {
		t.Fatal(a)
	}
	names := map[string]bool{}
	for _, p := range a.Assembly.Products {
		names[p.Name] = true
	}
	for _, name := range []string{"Export root", "Spline assembly", "Warped part"} {
		if !names[name] {
			t.Fatal(names)
		}
	}
	for _, expected := range m.Assembly.Occurrences {
		found := false
		for _, got := range a.Assembly.Occurrences {
			if got.Name == expected.Name {
				found = true
				for i := 0; i < 3; i++ {
					for j := 0; j < 4; j++ {
						if math.Abs(got.Transform[i][j]-expected.Transform[i][j]) > 1e-12 {
							t.Fatal(got, expected)
						}
					}
				}
			}
		}
		if !found {
			t.Fatal(expected)
		}
	}
}
func TestSplineAssemblyDocumentReject(t *testing.T) {
	_, m := splineAssemblyDocument(t)
	if _, e := FromSplineShellAssemblySTEP(m, STEPAssemblyOptions{MaxNodes: 2}, 1); e == nil {
		t.Fatal("expansion limit")
	}
	if _, e := FromSplineShellAssemblySTEP(m, STEPAssemblyOptions{Geometry: step.Limits{MaxVertices: 23}}, 1); e == nil {
		t.Fatal("geometry limit")
	}
	for id, bs := range m.ProductBodies {
		if len(bs) > 0 {
			bs[0].Faces[0].Domain.UMin = .25
			m.ProductBodies[id] = bs
			break
		}
	}
	if _, e := FromSplineShellAssemblySTEP(m, STEPAssemblyOptions{}, 1); e == nil {
		t.Fatal("mutated source descriptors")
	}
	d, _ := splineAssemblyDocument(t)
	s, _ := d.Snapshot()
	for id, n := range s.Nodes {
		if n.Asset != "" {
			n.Transform[0][0] = 2
			s.Nodes[id] = n
			break
		}
	}
	var b bytes.Buffer
	if e := s.WriteSplineShellAssemblySTEP(&b, ""); e == nil || b.Len() != 0 {
		t.Fatal(e, b.Len())
	}
}
func TestSplineAssemblyFeatureRegeneration(t *testing.T) {
	d, _ := splineAssemblyDocument(t)
	s, _ := d.Snapshot()
	asset := ""
	for id := range s.Trimmed {
		asset = id
	}
	for id, n := range s.Nodes {
		if n.Asset == asset {
			n.Asset = "moved"
			s.Nodes[id] = n
		}
	}
	for _, shift := range []float64{2, 7} {
		s.Features = map[string]Feature{"moved": {Kind: TransformFeature, Input: asset, Transform: geom.Translation(geom.Vec3{X: shift})}}
		var b bytes.Buffer
		if e := s.WriteSplineShellAssemblySTEP(&b, ""); e != nil {
			t.Fatal(e)
		}
		m, e := step.ReadSplineShellAssembly(&b, step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		count := 0
		for _, bs := range m.ProductBodies {
			for _, body := range bs {
				count++
				minimum := math.Inf(1)
				for _, v := range body.Solid.Vertices() {
					minimum = math.Min(minimum, v.X)
				}
				if minimum != shift {
					t.Fatal(minimum, shift)
				}
			}
		}
		if count != 1 {
			t.Fatal("lost sharing", count)
		}
	}
}
func TestOCCTSplineAssemblyDocument(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if oracle == "" || os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT STEP oracle")
	}
	d, _ := splineAssemblyDocument(t)
	s, _ := d.Snapshot()
	var b bytes.Buffer
	if e := s.WriteSplineShellAssemblySTEP(&b, "Export root"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "document.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "result.json")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if data, e := exec.CommandContext(ctx, oracle, out, "step-assembly", path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, data)
	}
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Volume      float64
		Centroid    [3]float64
		Products    []string
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
	}
	if e = json.Unmarshal(raw, &result); e != nil || len(result.Products) != 3 || len(result.Occurrences) != 3 || math.Abs(result.Volume-10) > 1e-7 {
		t.Fatal(result, e)
	}
	for i, want := range []float64{5, 10 + 16.0/15, 29.0 / 45} {
		if math.Abs(result.Centroid[i]-want) > 1e-7 {
			t.Fatal(result)
		}
	}
	for _, name := range []string{"Export root", "Spline assembly", "Warped part"} {
		found := false
		for _, got := range result.Products {
			found = found || got == name
		}
		if !found {
			t.Fatal(result)
		}
	}
}
