package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"reflect"
	"testing"
)

func TestIGESShearedLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/sheared-spline-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Shear != -.125 || m.Radii.X != .25 || m.Radii.Y != .25 {
		t.Fatal(m)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil || math.Abs(p.Volume-math.Pi/512) > 1e-10 {
		t.Fatal(p, e)
	}
	var b bytes.Buffer
	if e = WriteSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Shear != m.Shear || !reflect.DeepEqual(q.Upper.Data(), m.Upper.Data()) || !reflect.DeepEqual(q.Lower.Data(), m.Lower.Data()) {
		t.Fatal("sheared support changed")
	}
	for _, shear := range []float64{0, .125, math.Nextafter(-.125, 0)} {
		m.Shear = shear
		b.Reset()
		if e = WriteSplineLensSolid(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("wrong shear emitted", shear)
		}
	}
}
func TestIGESShearedLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-sheared-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "target.igs")
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{source, target} {
		out := filepath.Join(t.TempDir(), "mass.json")
		oracleCommand(t, out, "iges-solid", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var r struct {
			Solids, Faces, Edges, Vertices int
			Volume, Area                   float64
		}
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 4 || r.Vertices != 4 || math.Abs(r.Volume-p.Volume) > 1e-10 || math.Abs(r.Area-p.Area) > 1e-9 {
			t.Fatal(r, p, e)
		}
	}
}
