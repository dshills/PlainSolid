package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
	"time"
)

func circularCylinderSource(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/circular-cylinder-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestCircularCylinderSolid(t *testing.T) {
	ctx := context.Background()
	m, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(circularCylinderSource(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Descriptor != (CircularCylinderDescriptor{Radius: 2, Height: 3}) || len(m.Parameters) != 24 || m.SourceSolidID != 47 || m.SourceSurfaceIDs != [3]int{9, 11, 13} {
		t.Fatal(m.Descriptor)
	}
	p, e := m.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-12*math.Pi) > 1e-8 || math.Abs(p.Area-20*math.Pi) > 1e-8 || p.Centroid.Distance(geom.Vec3{Z: 1.5}) > 1e-9 {
		t.Fatal(p, e)
	}
	for _, d := range []CircularCylinderDescriptor{{Radius: 2, Height: 3}, {Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 3}} {
		var out bytes.Buffer
		if e = WriteCircularCylinderSolid(ctx, &out, d, .001); e != nil {
			t.Fatal(e)
		}
		got, e := ReadCircularCylinderSolid(ctx, &out, Limits{})
		if e != nil || got.Descriptor != d {
			t.Fatal(got.Descriptor, e)
		}
	}
}
func mutateCircularCylinder(t testing.TB, raw []byte, id, index int, value string) []byte {
	t.Helper()
	doc, e := readPlacedTopologyRecords(bytes.NewReader(raw), Limits{}, true, false, false, false, false, false, false, true)
	if e != nil {
		t.Fatal(e)
	}
	a := topologyAuthor{}
	a.geometry.Entities = []Entity{{Point: &geom.Vec3{}}}
	changed := false
	for _, r := range doc.entities {
		v := make([]string, len(r.values))
		for j, p := range r.values {
			v[j] = p.text
		}
		if r.id == id {
			if v[index] == value {
				t.Fatal("ineffective mutation")
			}
			v[index] = value
			changed = true
		}
		form := 0
		if r.kind >= 500 || r.kind == 190 || r.kind == 192 {
			form = 1
		}
		a.add(r.kind, form, strings.Join(v, ",")+";")
	}
	if !changed {
		t.Fatal("missing entity")
	}
	out, e := a.file(doc.unit)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCircularCylinderSolidRejects(t *testing.T) {
	ctx := context.Background()
	b := circularCylinderSource(t)
	for _, tc := range []struct {
		id, index int
		value     string
	}{{9, 1, "1.0"}, {9, 3, "3"}, {17, 2, "0.25"}, {17, 6, "0"}, {15, 3, "0.5"}, {21, 1, "2.25"}, {25, 1, "359"}, {25, 1, "6.283185307179586"}, {33, 5, "1"}, {45, 7, "1"}, {47, 2, "0"}, {23, 2, "19"}} {
		m, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(mutateCircularCylinder(t, b, tc.id, tc.index, tc.value)), Limits{})
		if e == nil || len(m.Parameters) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 23}, {MaxValues: 20}, {MaxRecords: 40}} {
		if _, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(b), l); e == nil {
			t.Fatal(l)
		}
	}
	for _, id := range []int{9, 17, 47} {
		if _, e := ReadCircularCylinderSolid(ctx, bytes.NewReader(placedDir(b, id, 6, 5)), Limits{}); e == nil {
			t.Fatal("transform", id)
		}
	}
	vendor, e := os.ReadFile("testdata/occt-rounded-circular-cylinder.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadCircularCylinderSolid(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("vendor graph admitted")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadCircularCylinderSolid(c, bytes.NewReader(b), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, d := range []CircularCylinderDescriptor{{Radius: 0, Height: 3}, {Radius: 2, Height: -3}, {Origin: geom.Vec3{X: math.Ldexp(1, 53)}, Radius: 1, Height: 3}, {Origin: geom.Vec3{Z: 1}, Radius: 2, Height: math.SmallestNonzeroFloat64}} {
		var out bytes.Buffer
		if e = WriteCircularCylinderSolid(ctx, &out, d, .001); e == nil || out.Len() != 0 {
			t.Fatal("invalid writer", d, e)
		}
	}
}
func TestOCCTCircularCylinderSolid(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw := circularCylinderSource(t)
	for _, d := range []CircularCylinderDescriptor{{Radius: 2, Height: 3}, {Origin: geom.Vec3{X: .25, Y: -2, Z: 3}, Radius: 2, Height: 3}} {
		var b bytes.Buffer
		if e := WriteCircularCylinderSolid(context.Background(), &b, d, .001); e != nil {
			t.Fatal(e)
		}
		cases := [][]byte{b.Bytes()}
		if d.Origin == (geom.Vec3{}) {
			cases = append(cases, raw)
		}
		for _, input := range cases {
			path := filepath.Join(t.TempDir(), "cylinder.igs")
			if e := os.WriteFile(path, input, 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			out, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
			cancel()
			if e != nil {
				t.Fatal(e, string(out))
			}
			at := bytes.LastIndex(out, []byte(`{"Faces"`))
			if at < 0 {
				t.Fatal(string(out))
			}
			var p struct {
				Faces, Solids int
				Volume, Area  float64
				Center        [3]float64
			}
			if e = json.Unmarshal(out[at:], &p); e != nil {
				t.Fatal(e)
			}
			if p.Faces != 3 || p.Solids != 1 || math.Abs(p.Volume-12*math.Pi) > 1e-8 || math.Abs(p.Area-20*math.Pi) > 1e-8 || math.Abs(p.Center[0]-d.Origin.X) > 1e-9 || math.Abs(p.Center[1]-d.Origin.Y) > 1e-9 || math.Abs(p.Center[2]-d.Origin.Z-1.5) > 1e-9 {
				t.Fatal(p)
			}
		}
	}
}
func FuzzCircularCylinderSolid(f *testing.F) {
	f.Add(circularCylinderSource(f))
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			t.Skip()
		}
		_, _ = ReadCircularCylinderSolid(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 32768, MaxEntities: 24, MaxValues: 2048})
	})
}
