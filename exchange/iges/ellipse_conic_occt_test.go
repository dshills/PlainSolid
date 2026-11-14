package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func TestOCCTEllipseConic(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_ELLIPSE_CONIC")
	if bin == "" {
		t.Skip("optional pinned OCCT helper")
	}
	source := "testdata/occt-ellipse-conic.igs"
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadEllipseConic(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteEllipseConic(&out, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ellipse.igs")
	if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	g, e := m.Curve.Geometry()
	if e != nil {
		t.Fatal(e)
	}
	p, _ := g.Eval(.375)
	d, _ := g.Derivative(.375)
	for _, file := range []string{source, path} {
		data, e := exec.Command(bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		i := bytes.LastIndexByte(data, '{')
		if i < 0 {
			t.Fatal(string(data))
		}
		var v struct {
			First, Last                   float64
			Point, Derivative, Start, End [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(data[i:]), &v); e != nil {
			t.Fatal(e)
		}
		vector := func(a [3]float64) geom.Vec3 { return geom.Vec3{X: a[0], Y: a[1], Z: a[2]} }
		if v.First != 0 || math.Abs(v.Last-2*math.Pi) > 1e-14 || vector(v.Point).Distance(p) > 1e-12 || vector(v.Derivative).Distance(d) > 1e-12 || vector(v.Start).Distance(geom.Vec3{X: 1, Y: 7, Z: 3}) > 1e-12 || vector(v.End).Distance(vector(v.Start)) > 1e-12 {
			t.Fatal(v)
		}
	}
}
