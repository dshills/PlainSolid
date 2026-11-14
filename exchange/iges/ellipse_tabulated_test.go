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

func TestEllipseTabulated(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ellipse-tabulated.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseTabulated(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.End != (geom.Vec3{X: 4, Y: 7, Z: 3}) {
		t.Fatal(m.End)
	}
	p, e := m.ExactPatches(context.Background(), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range p {
		a, e := p[i].Eval(0, 0)
		if e != nil {
			t.Fatal(e)
		}
		b, e := p[i].Eval(0, 1)
		if e != nil || b.Sub(a).Distance(geom.Vec3{X: 3}) > 1e-14 {
			t.Fatal(a, b, e)
		}
	}

	for _, text := range []string{"122,1,4.,7.,3.;", "122,5,4.,7.,3.;", "122,3,1.,7.,3.;", "122,3,0.1,7.,3.;"} {
		bad := append([]byte(nil), raw...)
		off := bytes.Index(bad, []byte("122,3,4.,7.,3.;"))
		if off < 0 {
			t.Fatal("fixture root")
		}
		copy(bad[off:off+64], []byte(text+strings.Repeat(" ", 64-len(text))))
		if _, e = ReadEllipseTabulated(bytes.NewReader(bad), Limits{}); e == nil {
			t.Fatal("forged terminal/owner")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = m.ExactPatches(ctx, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("cancellation")
	}
	var out bytes.Buffer
	if e = WriteEllipseTabulated(&out, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseTabulated(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseConicRing(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("ring accepted surface")
	}
	if _, e = ReadTabulatedSurfaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("spline-only accepted ring")
	}
	for _, bad := range []geom.Vec3{{X: 1, Y: 7, Z: 3}, {X: math.NaN()}} {
		x := m
		x.End = bad
		var b bytes.Buffer
		if WriteEllipseTabulated(&b, x) == nil || b.Len() != 0 {
			t.Fatal("invalid displacement")
		}
	}
	x := m
	x.End.X = -2
	var neg bytes.Buffer
	if e = WriteEllipseTabulated(&neg, x); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadEllipseTabulated(&neg, Limits{}); e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxEntities: 6}, {MaxBytes: 80}, {MaxRecords: 2}, {MaxValues: 4}} {
		if _, e = ReadEllipseTabulated(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPSE_TABULATED"); oracle != "" {
		path := filepath.Join(t.TempDir(), "surface.igs")
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{"testdata/occt-ellipse-tabulated.igs", path} {
			b, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v", b, e)
			}
			var v struct {
				Faces int
				Area  float64
			}
			at := bytes.LastIndex(b, []byte("{"))
			if at < 0 || json.Unmarshal(bytes.TrimSpace(b[at:]), &v) != nil || v.Faces != 4 || math.Abs(v.Area-3*23.01311259606501) > 1e-7 {
				t.Fatalf("%s", b)
			}
		}
	}
}
func FuzzEllipseTabulated(f *testing.F) {
	b, e := os.ReadFile("testdata/occt-ellipse-tabulated.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 32768 {
			return
		}
		m, e := ReadEllipseTabulated(bytes.NewReader(b), Limits{MaxBytes: 32768, MaxEntities: 7, MaxRecords: 256, MaxValues: 1024})
		if e == nil {
			var out bytes.Buffer
			if e = WriteEllipseTabulated(&out, m); e != nil {
				t.Fatal(e)
			}
		}
	})
}
