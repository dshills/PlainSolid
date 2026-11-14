package document

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func mappedDocumentSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("../exchange/step/testdata/occt-multiple-mapped-planar-boxes.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func checkMappedDocument(t *testing.T, s State, source step.MappedPlanarMaterials) {
	t.Helper()
	if len(s.Planar) != 2 || len(s.Nodes) != 4 || s.LengthUnit != .001 {
		t.Fatal(s)
	}
	refs := map[string]int{}
	bounds := map[string]geom.Box{"first translated": {Min: geom.Vec3{X: 10}, Max: geom.Vec3{X: 12, Y: 3, Z: 4}}, "first rotated": {Min: geom.Vec3{Y: 20}, Max: geom.Vec3{X: 4, Y: 22, Z: 3}}, "second translated": {Min: geom.Vec3{X: -10}, Max: geom.Vec3{X: -9, Y: 2, Z: 3}}}
	for id, n := range s.Nodes {
		if n.Asset == "" {
			continue
		}
		refs[n.Asset]++
		tr, e := s.WorldTransform(id)
		if e != nil {
			t.Fatal(e)
		}
		points := s.Planar[n.Asset].Vertices()
		for i := range points {
			points[i] = tr.Apply(points[i])
		}
		b, e := geom.Bounds(points)
		if e != nil || b != bounds[n.Name] {
			t.Fatal(id, b, e)
		}
		label := s.Labels["source:"+id]
		mid := label.Attributes["step.source.map"].Integer
		item := label.Attributes["step.source.item"].Integer
		found := false
		for _, u := range source.Materials[mid].Occurrences {
			if u.ItemID == item && u.Transform == n.Transform && u.Name == n.Name {
				found = true
			}
		}
		if !found {
			t.Fatal("node ancestry", id)
		}
	}
	if len(refs) != 2 {
		t.Fatal(refs)
	}
	counts := map[int]int{}
	for _, n := range refs {
		counts[n]++
	}
	if counts[2] != 1 || counts[1] != 1 {
		t.Fatal(refs)
	}
	faces := 0
	for _, label := range s.Labels {
		if label.Target != FaceLabel {
			continue
		}
		faces++
		sid := label.Attributes["step.source.surface"].Integer
		fid := label.Attributes["step.source.face"].Integer
		if source.Source.Entities[fid].Records[0].Args[2].Ref != sid {
			t.Fatal("surface ancestry")
		}
		surface, e := step.DecodeAnalyticSurface(source.Source, sid)
		if e != nil {
			t.Fatal(e)
		}
		plane, ok := surface.(geom.PlaneSurface)
		if !ok {
			t.Fatal(surface)
		}
		solid := s.Planar[label.TargetID]
		vs, es, fs := solid.Vertices(), solid.Edges(), solid.Faces()
		for _, use := range fs[*label.Face].Boundary {
			edge := es[use.Edge]
			if plane.Frame().ToLocal(vs[edge.Start]).Z != 0 || plane.Frame().ToLocal(vs[edge.End]).Z != 0 {
				t.Fatal("native/source face mismatch")
			}
		}
	}
	if faces != 12 {
		t.Fatal(faces)
	}
	mesh, e := s.Flatten()
	if e != nil {
		t.Fatal(e)
	}
	b, e := mesh.Bounds()
	if e != nil || b.Min != (geom.Vec3{X: -10}) || b.Max != (geom.Vec3{X: 12, Y: 22, Z: 4}) {
		t.Fatal(b, e)
	}
	p, e := mesh.MassProperties()
	if e != nil || math.Abs(p.Volume-54) > 1e-10 || math.Abs(p.SurfaceArea-126) > 1e-10 {
		t.Fatal(p, e)
	}
}
func TestMappedPlanarDocument(t *testing.T) {
	raw := mappedDocumentSource(t)
	a, e := step.ReadMappedPlanarMaterials(bytes.NewReader(raw), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := FromMappedPlanarMaterialsSTEP(a, STEPAssemblyOptions{}, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	checkMappedDocument(t, s, a)
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	again, e := Read(&saved, 4<<20, 2)
	if e != nil {
		t.Fatal(e)
	}
	n, _ := again.Snapshot()
	checkMappedDocument(t, n, a)
	if !reflect.DeepEqual(s.Labels, n.Labels) || !reflect.DeepEqual(s.Nodes, n.Nodes) {
		t.Fatal("metadata changed")
	}
	if _, e = ReadMappedPlanarMaterialsSTEP(bytes.NewReader(raw), STEPAssemblyOptions{}, 0); e != nil {
		t.Fatal(e)
	}
}
func TestMappedPlanarDocumentRejects(t *testing.T) {
	raw := mappedDocumentSource(t)
	for _, mode := range []string{"geometry", "transform", "body-id", "map-order", "name", "unit"} {
		a, e := step.ReadMappedPlanarMaterials(bytes.NewReader(raw), step.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		id := a.MapOrder[0]
		m := a.Materials[id]
		switch mode {
		case "geometry":
			m.Body.Solid, _ = brep.Box(geom.Vec3{}, geom.Vec3{X: 9, Y: 9, Z: 9})
		case "transform":
			m.Occurrences[0].Transform = geom.Identity()
		case "body-id":
			m.SourceBodyID++
		case "map-order":
			a.MapOrder[0], a.MapOrder[1] = a.MapOrder[1], a.MapOrder[0]
		case "name":
			m.Body.Name = "forged"
		case "unit":
			a.LengthUnit = 1
		}
		a.Materials[id] = m
		if d, e := FromMappedPlanarMaterialsSTEP(a, STEPAssemblyOptions{}, 0); e == nil || d != nil {
			t.Fatal(mode)
		}
	}
	for _, o := range []STEPAssemblyOptions{{MaxNodes: 3}, {MaxDepth: 1}, {Geometry: step.Limits{MaxFaces: 11}}} {
		if d, e := ReadMappedPlanarMaterialsSTEP(bytes.NewReader(raw), o, 0); e == nil || d != nil {
			t.Fatal(o)
		}
	}
}
func TestOCCTMappedPlanarDocument(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	d, e := ReadMappedPlanarMaterialsSTEP(bytes.NewReader(mappedDocumentSource(t)), STEPAssemblyOptions{}, 0)
	if e != nil {
		t.Fatal(e)
	}
	var saved bytes.Buffer
	if e = d.Write(&saved); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&saved, 4<<20, 0)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	var out bytes.Buffer
	if e = s.WriteSTEPAssembly(&out, "Native mapped geometry export"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(b))
	}
	at := bytes.LastIndex(b, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(b))
	}
	var p struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	if e = json.Unmarshal(b[at:], &p); e != nil {
		t.Fatal(e)
	}
	if p.Faces != 18 || p.Solids != 3 || math.Abs(p.Volume-54) > 1e-9 || math.Abs(p.Area-126) > 1e-9 {
		t.Fatal(p)
	}
	for i, x := range []float64{85. / 18, 91. / 9, 31. / 18} {
		if math.Abs(p.Center[i]-x) > 1e-9 {
			t.Fatal(fmt.Sprint(p))
		}
	}
}
