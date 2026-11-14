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

func editedMappedState(t *testing.T) (State, step.MappedPlanarMaterials) {
	t.Helper()
	a, e := step.ReadMappedPlanarMaterials(bytes.NewReader(mappedDocumentSource(t)), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	d, e := FromMappedPlanarMaterialsSTEP(a, STEPAssemblyOptions{}, 0)
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
	r := s.Nodes["step-mapped-root"]
	r.Name = "edited shared mapped uses"
	s.Nodes["step-mapped-root"] = r
	for id, n := range s.Nodes {
		if n.Asset != "" {
			n.Name = "edited " + n.Name
			n.Transform[0][3] += 2.5
			s.Nodes[id] = n
		}
	}
	return s, a
}
func TestEditedMappedPlanarDocument(t *testing.T) {
	s, a := editedMappedState(t)
	var before, out, after bytes.Buffer
	if e := step.WriteMappedPlanarMaterialsSource(&before, a.Source); e != nil {
		t.Fatal(e)
	}
	if e := s.WriteEditedMappedPlanarMaterialsSTEP(&out, a.Source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	b, e := step.ReadMappedPlanarMaterials(bytes.NewReader(out.Bytes()), step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if b.Name != "edited shared mapped uses" || !reflect.DeepEqual(a.MapOrder, b.MapOrder) {
		t.Fatal(b.Name)
	}
	for mid, m := range b.Materials {
		old := a.Materials[mid]
		if !reflect.DeepEqual(m.Body.Solid.Vertices(), old.Body.Solid.Vertices()) || !reflect.DeepEqual(m.Body.Solid.Faces(), old.Body.Solid.Faces()) {
			t.Fatal("local geometry changed")
		}
		for _, u := range m.Occurrences {
			n := s.Nodes[fmt.Sprintf("step-item-%d", u.ItemID)]
			if n.Transform != u.Transform || n.Name != u.Name {
				t.Fatal(u, n)
			}
		}
	}
	if e = step.WriteMappedPlanarMaterialsSource(&after, a.Source); e != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("source mutated", e)
	}
	var twice bytes.Buffer
	if e = s.WriteEditedMappedPlanarMaterialsSTEP(&twice, a.Source, step.Limits{}); e != nil || !bytes.Equal(twice.Bytes(), out.Bytes()) {
		t.Fatal("nondeterministic", e)
	}
	for _, test := range []string{"geometry", "units", "asset", "hierarchy", "scale", "reflection", "node", "limits"} {
		t.Run(test, func(t *testing.T) {
			v, src := editedMappedState(t)
			limits := step.Limits{}
			var key string
			for id, n := range v.Nodes {
				if n.Asset != "" {
					key = id
					break
				}
			}
			n := v.Nodes[key]
			switch test {
			case "geometry":
				v.Planar[n.Asset], _ = brep.Box(geom.Vec3{}, geom.Vec3{X: 9, Y: 9, Z: 9})
			case "units":
				v.LengthUnit = 1
			case "asset":
				n.Asset = "missing"
				v.Nodes[key] = n
			case "hierarchy":
				n.Parent = ""
				v.Nodes[key] = n
			case "scale":
				n.Transform[0][0] = 2
				v.Nodes[key] = n
			case "reflection":
				n.Transform = geom.Scaling(geom.Vec3{X: -1, Y: 1, Z: 1})
				v.Nodes[key] = n
			case "node":
				delete(v.Nodes, key)
			case "limits":
				limits.MaxEntities = 10
			}
			var dst bytes.Buffer
			if e := v.WriteEditedMappedPlanarMaterialsSTEP(&dst, src.Source, limits); e == nil || dst.Len() != 0 {
				t.Fatal("accepted", e, dst.Len())
			}
		})
	}
}
func TestOCCTEditedMappedPlanarDocument(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_DISK_BOUND_MASS")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	s, a := editedMappedState(t)
	var out bytes.Buffer
	if e := s.WriteEditedMappedPlanarMaterialsSTEP(&out, a.Source, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "edited.step")
	if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	at := bytes.LastIndex(raw, []byte(`{"Faces"`))
	if at < 0 {
		t.Fatal(string(raw))
	}
	var p struct {
		Faces, Solids int
		Volume, Area  float64
		Center        [3]float64
	}
	if e = json.Unmarshal(raw[at:], &p); e != nil {
		t.Fatal(e)
	}
	if p.Faces != 18 || p.Solids != 3 || math.Abs(p.Volume-54) > 1e-9 || math.Abs(p.Area-126) > 1e-9 {
		t.Fatal(p)
	}
	for i, x := range []float64{65. / 9, 91. / 9, 31. / 18} {
		if math.Abs(p.Center[i]-x) > 1e-9 {
			t.Fatal(p)
		}
	}
}
