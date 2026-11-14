package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestEllipticalPrismTopology(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/elliptical-prism-topology.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.SourceEdgeListID != 23 || m.SourceVertexListID != 21 || m.Descriptor.LowerZ != 3 || m.Descriptor.UpperZ != 7 {
		t.Fatal(m)
	}
	patches, e := m.ExactLateralPatches(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range patches {
		a, e := p.Eval(1, 0)
		if e != nil {
			t.Fatal(e)
		}
		b, e := patches[(i+1)%4].Eval(0, 0)
		if e != nil || a != b {
			t.Fatal(a, b, e)
		}
		top, e := p.Eval(1, 1)
		if e != nil || top.Z-a.Z != 4 {
			t.Fatal(top, a, e)
		}
	}
	var out bytes.Buffer
	if e = WriteEllipticalPrismTopology(ctx, &out, m.Descriptor, m.LengthUnit); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{"122,9,5,0,7;", "122,11,5,0,7;"}, {"504,3,11,21,1,21,1,13,21,2,21,1,9,21,2,21,2;", "504,3,11,21,1,21,1,13,21,2,21,1,9,21,1,21,1;"}, {"514,3,31,1,33,1,35,0;", "514,3,31,1,33,1,35,1;"}, {"508,4,0,23,1,0,0,0,23,2,0,0,0,23,3,1,0,0,23,2,1,0;", "508,4,0,23,1,0,0,0,23,2,0,0,0,23,3,1,0,0,23,2,0,0;"}} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte(pair[0]))
		if off < 0 {
			t.Fatal("fixture value", pair)
		}
		copy(bad[off:off+64], []byte(pair[1]+strings.Repeat(" ", 64-len(pair[1]))))
		if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("topology mutation accepted")
		}
	}
	vendor, e := os.ReadFile("testdata/occt-vendor-elliptical-prism.igs")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(vendor), Limits{}); e == nil {
		t.Fatal("approximated vendor source accepted")
	}
	if !bytes.Contains(vendor, []byte("-1.110223025E-16")) {
		t.Fatal("vendor witness changed")
	}
	for _, l := range []Limits{{MaxEntities: 19}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
		if _, e = ReadEllipticalPrismTopology(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadEllipticalPrismTopology(c, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancellation")
	}
	d := m.Descriptor
	d.UpperZ = d.LowerZ
	var empty bytes.Buffer
	if WriteEllipticalPrismTopology(ctx, &empty, d, .001) == nil || empty.Len() != 0 {
		t.Fatal("empty sweep")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPTICAL_PRISM_TOPOLOGY"); oracle != "" {
		p := filepath.Join(t.TempDir(), "prism.igs")
		if e = os.WriteFile(p, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, p := range []string{"testdata/elliptical-prism-topology.igs", p} {
			b, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", b, e)
			}
			var v struct {
				Faces, Solids int
				Volume, Area  float64
				Center        [3]float64
			}
			at := bytes.LastIndex(b, []byte("{"))
			if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &v) != nil || v.Faces != 3 || v.Solids != 1 || math.Abs(v.Volume-40*math.Pi) > 1e-8 || math.Abs(v.Area-(20*math.Pi+4*23.01311259606501)) > 1e-7 || math.Abs(v.Center[2]-5) > 1e-12 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzEllipticalPrismTopology(f *testing.F) {
	b, e := os.ReadFile("testdata/elliptical-prism-topology.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipticalPrismTopology(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipticalPrismTopology(context.Background(), &out, m.Descriptor, m.LengthUnit); e != nil {
				t.Fatal(e)
			}
		}
	})
}
