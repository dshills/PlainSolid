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
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func biquadVertices() [8]geom.Vec3 {
	var v [8]geom.Vec3
	for i := range v {
		v[i] = geom.Vec3{X: float64(i & 1), Y: float64((i >> 1) & 1), Z: float64(i >> 2)}
	}
	return v
}
func biquadTop(t testing.TB) *geom.BSplineSurface {
	t.Helper()
	p := make([]geom.Vec3, 9)
	for u := 0; u < 3; u++ {
		for v := 0; v < 3; v++ {
			z := 1.
			if u == 1 && v == 1 {
				z = 1.25
			}
			p[3*u+v] = geom.Vec3{X: float64(u) / 2, Y: float64(v) / 2, Z: z}
		}
	}
	g, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return g
}
func TestBiquadraticSolid(t *testing.T) {
	raw, e := os.ReadFile("testdata/biquadratic-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadBiquadraticHexahedronSolid(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !m.Embedding.Complete || m.CurvedSurfaceID == 0 || len(m.Geometry) != 42 {
		t.Fatal(m)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-37.0/36) > 1e-9 || math.Abs(p.Centroid.Z-951.0/1850) > 1e-9 {
		t.Fatal(p, e)
	}
	var out bytes.Buffer
	if e = WriteBiquadraticHexahedronSolid(context.Background(), &out, biquadVertices(), 1, biquadTop(t), .001); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadBiquadraticHexahedronSolid(context.Background(), &out, Limits{}); e != nil {
		t.Fatal(e)
	}
}

func TestBiquadraticSolidRejects(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/biquadratic-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		k, i int
		s    string
	}{{186, 2, "0"}, {514, 3, "0"}, {508, 5, "0"}, {126, 14, "0.25"}, {502, 2, "0.25"}, {504, 7, "3"}, {508, 8, "3"}} {
		b := tetraMutate(t, raw, tc.k, tc.i, tc.s)
		m, e := ReadBiquadraticHexahedronSolid(context.Background(), bytes.NewReader(b), Limits{})
		if e == nil || m.SourceSolidID != 0 || len(m.Geometry) != 0 {
			t.Fatal(tc, e)
		}
	}
	for _, l := range []Limits{{MaxEntities: 57}, {MaxControlPoints: 100}, {MaxBytes: 100}} {
		if _, e = ReadBiquadraticHexahedronSolid(context.Background(), bytes.NewReader(raw), l); e == nil {
			t.Fatal(l)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, e := ReadBiquadraticHexahedronSolid(ctx, bytes.NewReader(raw), Limits{}); e == nil || m.SourceSolidID != 0 {
		t.Fatal("cancel")
	}
	for _, bad := range []string{"rim", "fold", "below"} {
		d := biquadTop(t).Data()
		switch bad {
		case "rim":
			d.Poles[1].Z += .25
		case "fold":
			d.Poles[4].X = 20
		case "below":
			d.Poles[4].Z = -20
		}
		g, e := geom.NewBSplineSurface(2, 2, 3, 3, d.Poles, d.KnotsU, d.KnotsV, nil)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteBiquadraticHexahedronSolid(context.Background(), &b, biquadVertices(), 1, g, .001); e == nil || b.Len() != 0 {
			t.Fatal(bad, e)
		}
	}
	// Directory metadata must not be erased by the temporary planar scaffold.
	lines := bytes.Split(raw, []byte("\n"))
	for i, line := range lines {
		if len(line) == 80 && line[72] == 'D' {
			copy(line[24:32], []byte("       1"))
			lines[i] = line
			break
		}
	}
	if _, e = ReadBiquadraticHexahedronSolid(context.Background(), bytes.NewReader(bytes.Join(lines, []byte("\n"))), Limits{}); e == nil {
		t.Fatal("style discarded")
	}
}
func TestOCCTBiquadraticSolid(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_IGES_SOLID")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	raw, e := os.ReadFile("testdata/biquadratic-solid.igs")
	if e != nil {
		t.Fatal(e)
	}
	var sourceArea float64
	for _, native := range []bool{false, true} {
		b := raw
		if native {
			var out bytes.Buffer
			if e = WriteBiquadraticHexahedronSolid(context.Background(), &out, biquadVertices(), 1, biquadTop(t), .001); e != nil {
				t.Fatal(e)
			}
			b = out.Bytes()
		}
		path := filepath.Join(t.TempDir(), "dome.igs")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, e := exec.CommandContext(ctx, bin, path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatal(e, string(out))
		}
		at := bytes.LastIndex(out, []byte("{\"Faces\""))
		if at < 0 {
			t.Fatal(string(out))
		}
		var got struct {
			Faces, Solids int
			Volume, Area  float64
			Center        [3]float64
		}
		if e = json.Unmarshal(bytes.TrimSpace(out[at:]), &got); e != nil {
			t.Fatal(e)
		}
		if got.Faces != 6 || got.Solids != 1 || math.Abs(got.Volume-37.0/36) > 1e-10 || math.Abs(got.Center[2]-951.0/1850) > 1e-10 || math.Abs(got.Center[0]-.5) > 1e-10 || math.Abs(got.Center[1]-.5) > 1e-10 {
			t.Fatal(got)
		}
		if native && math.Abs(got.Area-sourceArea) > 1e-10 {
			t.Fatal("curved area changed", got.Area, sourceArea)
		}
		sourceArea = got.Area
	}
}
func FuzzBiquadraticSolid(f *testing.F) {
	raw, e := os.ReadFile("testdata/biquadratic-solid.igs")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		m, e := ReadBiquadraticHexahedronSolid(ctx, bytes.NewReader(b), Limits{})
		if e != nil && (m.SourceSolidID != 0 || len(m.Geometry) != 0) {
			t.Fatal("partial")
		}
	})
}
