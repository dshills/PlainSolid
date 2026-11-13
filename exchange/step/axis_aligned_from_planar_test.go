package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"testing"
)

func planarHoledL(t testing.TB) brep.Solid {
	t.Helper()
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: .5, Y: .5}, {X: 1.5, Y: .5}, {X: 1.5, Y: 1.5}, {X: .5, Y: 1.5}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestAxisAlignedFromPlanar(t *testing.T) {
	native := planarHoledL(t)
	source := Model{LengthUnit: .001, Bodies: []Body{{Name: "holed L", Solid: native}}}
	m, e := AxisAlignedSplineShellsFromPlanar(context.Background(), source)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].Faces) != 12 || m.Bodies[0].Name != "holed L" {
		t.Fatal(m)
	}
	holes := 0
	for _, h := range m.Bodies[0].FaceHoles {
		holes += len(h)
	}
	if holes != 2 {
		t.Fatal(holes)
	}
	var b bytes.Buffer
	if e = WriteAxisAlignedSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	round, e := ReadAxisAlignedSplineShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := round.Bodies[0].Solid.MassProperties()
	if e != nil || math.Abs(mass.Volume-33) > 1e-12 || math.Abs(mass.SurfaceArea-82) > 1e-12 {
		t.Fatal(mass, e)
	}
	moved, e := native.Transformed(geom.Translation(geom.Vec3{X: 10}))
	if e != nil {
		t.Fatal(e)
	}
	source.Bodies = append(source.Bodies, Body{Name: "second", Solid: moved})
	m, e = AxisAlignedSplineShellsFromPlanar(context.Background(), source)
	if e != nil || len(m.Bodies) != 2 {
		t.Fatal(e)
	}
	seen := map[int64]bool{}
	for _, body := range m.Bodies {
		for _, id := range body.FaceIDs {
			if seen[id] {
				t.Fatal("colliding IDs")
			}
			seen[id] = true
		}
	}
}
func TestAxisAlignedFromPlanarReject(t *testing.T) {
	native := planarHoledL(t)
	skew, e := native.Transformed(geom.Affine{{1, 1, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []brep.Solid{native.Reversed(), skew, {}} {
		if _, e := AxisAlignedSplineShellsFromPlanar(context.Background(), Model{LengthUnit: 1, Bodies: []Body{{Solid: s}}}); e == nil {
			t.Fatal("unsupported planar source accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := AxisAlignedSplineShellsFromPlanar(ctx, Model{LengthUnit: 1, Bodies: []Body{{Solid: native}}}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestOCCTAxisAlignedFromPlanar(t *testing.T) {
	m, e := AxisAlignedSplineShellsFromPlanar(context.Background(), Model{LengthUnit: .001, Bodies: []Body{{Name: "native holed prism", Solid: planarHoledL(t)}}})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteAxisAlignedSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var measure struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &measure); e != nil || math.Abs(measure.Volume-33) > 1e-9 || math.Abs(measure.Area-82) > 1e-9 {
		t.Fatal(measure, e)
	}
}
