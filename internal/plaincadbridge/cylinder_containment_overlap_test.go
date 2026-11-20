package plaincadbridge

import (
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestContainingCylinderOverlapCapture(t *testing.T) {
	b := New()
	tool := circle(t, b, vector{}, vector{0, 1, 0}, 25, 10)
	box := shape(t, b, map[string]any{"op": "box", "min": vector{10, 0, 0}, "max": vector{20, 10, 10}})
	for _, ids := range [][2]string{{tool.Handle, box.Handle}, {box.Handle, tool.Handle}} {
		r, e := b.overlapVolume(ids[0], ids[1])
		if e != nil {
			t.Fatal(r, e)
		}
		near(t, r.(map[string]float64)["volume"], 1000)
	}
	// Exact closed cap/radial inclusion: (3,4) lies on the radius-five wall.
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: -1})
	c, e := newCylinder(f, 5, 2)
	if e != nil {
		t.Fatal(e)
	}
	s, e := brep.Box(geom.Vec3{X: 2, Y: 3, Z: -2}, geom.Vec3{X: 3, Y: 4})
	if e != nil {
		t.Fatal(e)
	}
	v, ok, e := containedPlanarCylinderVolume(c, s)
	if e != nil || !ok || v != 2 {
		t.Fatal(v, ok, e)
	}
	outside, _ := brep.Box(geom.Vec3{X: 2, Y: 3, Z: -2}, geom.Vec3{X: math.Nextafter(3, math.Inf(1)), Y: 4})
	if _, ok, e = containedPlanarCylinderVolume(c, outside); e != nil || ok {
		t.Fatal("radial overrun", ok, e)
	}
	outside, _ = brep.Box(geom.Vec3{X: 2, Y: 3, Z: -2}, geom.Vec3{X: 3, Y: 4, Z: math.SmallestNonzeroFloat64})
	if _, ok, e = containedPlanarCylinderVolume(c, outside); e != nil || ok {
		t.Fatal("cap overrun", ok, e)
	}
	if _, ok, e = containedPlanarCylinderVolume(c, brep.Solid{}); e == nil || ok {
		t.Fatal("empty accepted")
	}
	// Partial overlap must not claim the entire box volume.
	partial := shape(t, b, map[string]any{"op": "box", "min": vector{24, 0, 0}, "max": vector{26, 10, 1}})
	if r, e := b.overlapVolume(tool.Handle, partial.Handle); e == nil && r.(map[string]float64)["volume"] == 20 {
		t.Fatal("partial falsely contained", r)
	}
	// The legacy proof declines the partial overlap, and the general
	// Boolean measures it: over the length of 10, the strip of the disk of
	// radius 25 between heights 0 and 1, beyond the abscissa 24.
	if _, e := legacyOverlapVolume(b.shapes[tool.Handle], b.shapes[partial.Handle]); e == nil || !strings.Contains(e.Error(), "strictly contained radial footprint") {
		t.Fatal("the legacy proof did not decline the partial overlap", e)
	}
	near(t, overlapMeasure(t, b, tool.Handle, partial.Handle), 10*((math.Sqrt(624)+625*math.Asin(1./25))/2-24))
}
