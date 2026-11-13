package iges

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"plainsolid/geom"
)

func oracleCommand(t *testing.T, out string, args ...string) {
	t.Helper()
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" || os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE and PLAINSOLID_OCCT_IGES=1 for independent IGES qualification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b, e := exec.CommandContext(ctx, program, append([]string{out}, args...)...).CombinedOutput(); e != nil {
		t.Fatalf("OCCT oracle: %v\n%s", e, b)
	}
}
func checkOracleLength(t *testing.T, input string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "metrics.json")
	oracleCommand(t, out, "iges", input)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Edges  int
		Length float64
	}
	if e = json.Unmarshal(b, &result); e != nil {
		t.Fatal(e)
	}
	want := 3 + 1.5*math.Pi
	if result.Edges != 3 || math.Abs(result.Length-want) > 1e-6 {
		t.Fatalf("OCCT edges %d length %.15g; expected3 and %.15g (1e-6 quadrature budget)", result.Edges, result.Length, want)
	}
}
func writeModelFile(t *testing.T, m Model) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "curves.igs")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	e = Write(f, m)
	closeErr := f.Close()
	if e != nil || closeErr != nil {
		t.Fatal(e, closeErr)
	}
	return path
}
func TestOCCTIGESExport(t *testing.T) {
	c, e := geom.NewBSpline(2, []geom.Vec3{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, math.Sqrt(.5), 1})
	if e != nil {
		t.Fatal(e)
	}
	m := Model{LengthUnit: .001, Entities: []Entity{{Line: &Line{End: geom.Vec3{X: 1, Y: 2, Z: 2}}}, {Arc: &Arc{Center: geom.Vec3{Z: 3}, Start: geom.Vec2{X: 2}, End: geom.Vec2{Y: 2}}}, {Spline: &Spline{Curve: c, Start: 0, End: 1, Planar: true, Normal: geom.Vec3{Z: 1}}}}}
	checkOracleLength(t, writeModelFile(t, m))
}
func TestOCCTIGESImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "occt.igs")
	oracleCommand(t, path, "write-iges")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Entities) != 3 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	want := [][3]geom.Vec3{{{}, {X: .5, Y: 1, Z: 1}, {X: 1, Y: 2, Z: 2}}, {{X: 2, Z: 3}, {X: math.Sqrt(2), Y: math.Sqrt(2), Z: 3}, {Y: 2, Z: 3}}, {{X: 1}, {X: math.Sqrt(.5), Y: math.Sqrt(.5)}, {Y: 1}}}
	for i, entity := range m.Entities {
		for j, u := range []float64{0, .5, 1} {
			p, e := entity.Eval(u)
			if e != nil || p.Distance(want[i][j]) > 1e-8 {
				t.Fatalf("entity%d at%g: %v expected%v error%v", i, u, p, want[i][j], e)
			}
		}
	}
	checkOracleLength(t, writeModelFile(t, m))
}

func TestOCCTIGESPointRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "occt-point.igs")
	oracleCommand(t, path, "write-iges-point")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Read(f, Limits{})
	_ = f.Close()
	if err != nil || len(m.Entities) != 1 || m.Entities[0].Point == nil || m.LengthUnit != .001 {
		t.Fatal(m, err)
	}
	want := geom.Vec3{X: 2.5, Y: -3.25, Z: 7.5}
	p, err := m.Entities[0].Eval(0)
	if err != nil || p.Distance(want) > 1e-9 {
		t.Fatal(p, err)
	}
	// Export through PlainSolid with a directory placement, and let OCCT
	// transfer it to model coordinates.
	m.Entities[0].Transform = &geom.Affine{{1, 0, 0, 1}, {0, 1, 0, -2}, {0, 0, 1, 3}}
	out := filepath.Join(dir, "occt-point.json")
	oracleCommand(t, out, "iges-point", writeModelFile(t, m))
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got [3]float64
	if err := json.Unmarshal(b, &got); err != nil || (geom.Vec3{X: got[0], Y: got[1], Z: got[2]}).Distance(want.Add(geom.Vec3{X: 1, Y: -2, Z: 3})) > 1e-9 {
		t.Fatal(got, err)
	}
}

func checkOracleSurface(t *testing.T, path string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "surface.json")
	oracleCommand(t, out, "iges-surface", path)
	b, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got.Faces != 1 || math.Abs(got.Area-math.Pi) > 1e-6 {
		t.Fatal(got)
	}
}
func TestOCCTIGESSurfaceExport(t *testing.T) {
	w := math.Sqrt(.5)
	g, e := geom.NewBSplineSurface(2, 1, 3, 2, []geom.Vec3{{X: 1}, {X: 1, Z: 2}, {X: 1, Y: 1}, {X: 1, Y: 1, Z: 2}, {Y: 1}, {Y: 1, Z: 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, w, w, 1, 1})
	if e != nil {
		t.Fatal(e)
	}
	checkOracleSurface(t, writeModelFile(t, Model{LengthUnit: .001, Entities: []Entity{{Surface: &Surface{Geometry: g, UStart: 0, UEnd: 1, VStart: 0, VEnd: 1}}}}))
}
func TestOCCTIGESSurfaceImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "occt-surface.igs")
	oracleCommand(t, path, "write-iges-surface")
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(f, Limits{})
	_ = f.Close()
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Entities) != 1 || m.Entities[0].Surface == nil {
		t.Fatal(m)
	}
	for _, u := range []float64{0, .25, .5, 1} {
		for _, v := range []float64{0, .3, 1} {
			p, e := m.Entities[0].EvalSurface(u, v)
			if e != nil || math.Abs(math.Hypot(p.X, p.Y)-1) > 1e-9 || math.Abs(p.Z-2*v) > 1e-9 {
				t.Fatal(p, e)
			}
		}
	}
	checkOracleSurface(t, writeModelFile(t, m))
}
