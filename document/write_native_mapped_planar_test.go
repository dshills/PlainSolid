package document

import (
	"bytes"
	"context"
	"encoding/json"
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

func nativeMappedDocument(t *testing.T) State {
	t.Helper()
	a, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	return State{LengthUnit: .001, Planar: map[string]brep.Solid{"large": a, "small": b}, Nodes: map[string]Node{"root": {Name: "native products", Transform: geom.Identity()}, "first": {Name: "translated", Parent: "root", Asset: "large", Transform: geom.Translation(geom.Vec3{X: 10})}, "second": {Name: "rotated", Parent: "root", Asset: "large", Transform: geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 20}, {0, 1, 0, 0}}}, "third": {Name: "small use", Parent: "root", Asset: "small", Transform: geom.Translation(geom.Vec3{X: -10})}}}
}
func nativeMappedReload(t *testing.T) State {
	t.Helper()
	d, e := New(nativeMappedDocument(t), 0)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	d, e = Read(&raw, 4<<20, 0)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := d.Snapshot()
	return s
}
func TestNativeMappedDocument(t *testing.T) {
	s := nativeMappedReload(t)
	var out, again bytes.Buffer
	if e := s.WriteNativeMappedPlanarSTEP(&out, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	if e := s.WriteNativeMappedPlanarSTEP(&again, step.Limits{}); e != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
		t.Fatal(e)
	}
	a, e := step.ReadMappedPlanarMaterials(&out, step.Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if a.Name != "native products" || a.LengthUnit != .001 || len(a.Materials) != 2 {
		t.Fatal(a)
	}
	counts := map[string]int{}
	bounds := []geom.Vec3{}
	for _, m := range a.Materials {
		counts[m.Body.Name] = len(m.Occurrences)
		for _, u := range m.Occurrences {
			for _, v := range m.Body.Solid.Vertices() {
				bounds = append(bounds, u.Transform.Apply(v))
			}
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"large": 2, "small": 1}) {
		t.Fatal(counts)
	}
	box, e := geom.Bounds(bounds)
	if e != nil || box.Min != (geom.Vec3{X: -10}) || box.Max != (geom.Vec3{X: 12, Y: 22, Z: 4}) {
		t.Fatal(box, e)
	}
}
func TestNativeMappedDocumentReject(t *testing.T) {
	for _, kind := range []string{"nested", "rootpose", "unit", "orphan", "extra-root", "scale", "feature"} {
		t.Run(kind, func(t *testing.T) {
			s := nativeMappedDocument(t)
			switch kind {
			case "nested":
				n := s.Nodes["third"]
				n.Parent = "first"
				s.Nodes["third"] = n
			case "rootpose":
				n := s.Nodes["root"]
				n.Transform[0][3] = 1
				s.Nodes["root"] = n
			case "unit":
				s.LengthUnit = -1
			case "orphan":
				delete(s.Nodes, "third")
			case "extra-root":
				s.Nodes["empty"] = Node{Transform: geom.Identity()}
			case "scale":
				n := s.Nodes["first"]
				n.Transform[0][0] = 2
				s.Nodes["first"] = n
			case "feature":
				s.Features = map[string]Feature{"unsupported": {}}
			}
			var b bytes.Buffer
			if e := s.WriteNativeMappedPlanarSTEP(&b, step.Limits{}); e == nil || b.Len() != 0 {
				t.Fatal(e, b.Len())
			}
		})
	}
}
func TestOCCTNativeMappedDocumentBounds(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_MAPPED_BOUNDS")
	if bin == "" {
		t.Skip("optional pinned OCCT bounds helper")
	}
	s := nativeMappedReload(t)
	var out bytes.Buffer
	if e := s.WriteNativeMappedPlanarSTEP(&out, step.Limits{}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(p, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, e := exec.CommandContext(ctx, bin, p).CombinedOutput()
	if e != nil {
		t.Fatal(e, string(raw))
	}
	at := bytes.LastIndex(raw, []byte(`{"Min"`))
	if at < 0 {
		t.Fatal(string(raw))
	}
	var b struct{ Min, Max [3]float64 }
	if e = json.Unmarshal(raw[at:], &b); e != nil {
		t.Fatal(e)
	}
	for i, v := range []float64{-10, 0, 0} {
		if math.Abs(b.Min[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
	for i, v := range []float64{12, 22, 4} {
		if math.Abs(b.Max[i]-v) > 1e-6 {
			t.Fatal(b)
		}
	}
}
