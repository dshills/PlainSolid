package iges

import (
	"bytes"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func ovalAnnulus(t *testing.T) Model {
	t.Helper()
	s := planarMappingModel(t).Entities[0].Surface.Geometry
	outer := QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, QuarterWeight: .75}
	hole := outer
	hole.Radii = geom.Vec2{X: .125, Y: .125}
	m, e := NewQuadraticOvalSurfaceWithHoles(s, outer, []QuadraticOval{hole}, .001)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestQuadraticOvalHoles(t *testing.T) {
	m := ovalAnnulus(t)
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	_, holes, e := got.QuadraticOvalBoundaries(got.TrimmedSurfaces[0])
	if e != nil || len(holes) != 1 || holes[0].Radii.X != .125 {
		t.Fatal(holes, e)
	}
	if _, e = got.QuadraticOvalBoundary(got.TrimmedSurfaces[0]); e == nil {
		t.Fatal("old API accepted holes")
	}
	s := m.Entities[0].Surface.Geometry
	o := QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .375, Y: .375}, QuarterWeight: .75}
	h := QuadraticOval{Center: geom.Vec2{X: .375, Y: .5}, Radii: geom.Vec2{X: .03125, Y: .03125}, QuarterWeight: .5}
	j := h
	j.Center.X = .625
	two, e := NewQuadraticOvalSurfaceWithHoles(s, o, []QuadraticOval{h, j}, .001)
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = Write(&b, two); e != nil {
		t.Fatal(e)
	}
	two, e = Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	_, hs, e := two.QuadraticOvalBoundaries(two.TrimmedSurfaces[0])
	if e != nil || len(hs) != 2 {
		t.Fatal(hs, e)
	}
	for _, bad := range [][]QuadraticOval{{o}, {h, h}, {h, {Center: h.Center, Radii: geom.Vec2{X: .015625, Y: .015625}, QuarterWeight: .5}}, {h, {Center: geom.Vec2{X: .4375, Y: .5}, Radii: h.Radii, QuarterWeight: .5}}, {{Center: geom.Vec2{X: .875, Y: .875}, Radii: h.Radii, QuarterWeight: .5}}} {
		if _, e = NewQuadraticOvalSurfaceWithHoles(s, o, bad, .001); e == nil {
			t.Fatal("invalid hole arrangement")
		}
	}
	m = ovalAnnulus(t)
	m.TrimmedSurfaces[0].Inner = []int{0}
	if e = m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
		t.Fatal("positive/reused hole")
	}
}
func TestOCCTQuadraticOvalAnnulus(t *testing.T) {
	source := t.TempDir() + "/source.igs"
	oracleCommand(t, source, "write-iges-quadratic-annulus")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	_, holes, e := m.QuadraticOvalBoundaries(m.TrimmedSurfaces[0])
	if e != nil || len(holes) != 1 {
		t.Fatal(holes, e)
	}
	var b bytes.Buffer
	if e = Write(&b, ovalAnnulus(t)); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/native.igs"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := rectangleOracleArea(t, source), rectangleOracleArea(t, path)
	if math.Abs(a-c) > 1e-9 || math.Abs(a-.75*1.1905776125224596) > 1e-7 {
		t.Fatal(a, c)
	}
}

func TestIndependentQuadraticAnnulusFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-annulus.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	_, holes, e := m.QuadraticOvalBoundaries(m.TrimmedSurfaces[0])
	if e != nil || len(holes) != 1 || holes[0].Radii.X != .125 {
		t.Fatal(holes, e)
	}
}
