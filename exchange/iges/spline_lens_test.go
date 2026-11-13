package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestIGESSplineLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
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
	if q.Center != m.Center || q.Radii != m.Radii {
		t.Fatal(q)
	}
}
func TestIGESSplineLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-spline-lens")
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
	target := filepath.Join(t.TempDir(), "native.igs")
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
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 4 || r.Vertices != 4 {
			t.Fatal(r, e)
		}
		if math.Abs(r.Volume-p.Volume) > 1e-10 || math.Abs(r.Area-p.Area) > 1e-9 {
			t.Fatal(r, p)
		}
	}
}
func TestIGESSplineLensReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/spline-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"solid-sense", "void", "same-face-sense", "loop-sense", "wrong-edge", "vertex", "UV-owner", "model", "bounds"} {
		t.Run(kind, func(t *testing.T) {
			d, e := readTopologySource(bytes.NewReader(raw), Limits{}, true)
			if e != nil {
				t.Fatal(e)
			}
			for i := range d.entities {
				r := &d.entities[i]
				switch {
				case kind == "solid-sense" && r.kind == 186:
					r.values[2].text = "0"
				case kind == "void" && r.kind == 186:
					r.values[3].text = "1"
				case kind == "same-face-sense" && r.kind == 514:
					r.values[5].text = "1"
				case kind == "loop-sense" && r.kind == 508:
					r.values[5].text = "0"
				case kind == "wrong-edge" && r.kind == 504:
					r.values[4].text = "2"
				case kind == "vertex" && r.kind == 502:
					r.values[2].text = "0.9"
				case kind == "UV-owner" && r.id == 35:
					r.values[8].text = "23"
				case kind == "model" && r.id == 13:
					r.values[17].text = "0.9"
				case kind == "bounds" && r.id == 7:
					r.values[len(r.values)-3].text = "0.6"
				}
			}
			if _, e = readIGESLens(d, Limits{}); e == nil {
				t.Fatal("invalid source lens accepted")
			}
		})
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Upper, m.Lower = m.Lower, m.Upper
	var b bytes.Buffer
	if e = WriteSplineLensSolid(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("inward material emitted")
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxRecords: 2}, {MaxEntities: 5}, {MaxValues: 5}, {MaxControlPoints: 2}} {
		if _, e = ReadSplineLensSolid(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit ignored")
		}
	}
	if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("generic API broadened")
	}
	if _, e = ReadCurvedOvalShell(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("open shell accepted solid")
	}
}
func FuzzIGESSplineLens(f *testing.F) {
	for _, name := range []string{"spline-lens", "sheared-spline-lens", "refined-spline-lens"} {
		b, e := os.ReadFile("testdata/" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineLensSolid(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 64, MaxValues: 20000, MaxControlPoints: 1024})
	})
}
func TestIGESSplineLensNative(t *testing.T) {
	var charts [2]*geom.BSplineSurface
	for side, sign := range []float64{1, -1} {
		var poles []geom.Vec3
		sq := []float64{.25, -.25, .25}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: 2 + sign*(1./16-sq[i]-sq[j])/2})
			}
		}
		var e error
		charts[side], e = geom.NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	m := SplineLensSolid{Upper: charts[0], Lower: charts[1], Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, LengthUnit: .0254}
	var b bytes.Buffer
	if e := WriteSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if q.LengthUnit != .0254 || !reflect.DeepEqual(q.Upper.Data(), m.Upper.Data()) || !reflect.DeepEqual(q.Lower.Data(), m.Lower.Data()) {
		t.Fatal("support or units changed")
	}
	m.Radii.X = .125
	b.Reset()
	if e = WriteSplineLensSolid(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("false material ellipse emitted")
	}
}
