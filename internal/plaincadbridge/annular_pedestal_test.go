package plaincadbridge

import (
	"math"
	"testing"
)

func TestPlainCADAnnularPedestalAndThroughBores(t *testing.T) {
	b := New()
	profile := func(kind string, z, height float64) map[string]any {
		return map[string]any{"kind": kind, "origin": vector{0, 0, z}, "normal": vector{0, 0, 1},
			"xDirection": vector{1, 0, 0}, "height": height}
	}
	slab := profile("roundedRectangle", 0, 12)
	slab["min"], slab["max"], slab["radii"] = [2]float64{-100, -85}, [2]float64{100, 85}, [4]float64{12, 12, 12, 12}
	base := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": slab})
	base = shape(t, b, map[string]any{"op": "capFillet", "handle": base.Handle, "origin": vector{0, 0, 12}, "normal": vector{0, 0, 1}, "radius": 1})
	annulus := profile("disk", 11, 21)
	annulus["center"], annulus["radius"] = [2]float64{}, 62
	annulus["circles"] = []map[string]any{{"center": [2]float64{}, "radius": 32}}
	ring := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": annulus})
	joined := shape(t, b, map[string]any{"op": "fuse", "a": base.Handle, "b": ring.Handle})
	near(t, joined.GeometryAssertions.Volume-base.GeometryAssertions.Volume, math.Pi*(62*62-32*32)*20)
	center := shape(t, b, map[string]any{"op": "circle", "origin": vector{0, 0, 0}, "normal": vector{0, 0, 1}, "radius": 24, "height": 33})
	probe, err := b.overlapVolume(joined.Handle, center.Handle)
	if err != nil {
		t.Fatal(err)
	}
	near(t, probe.(map[string]float64)["volume"], math.Pi*24*24*12)
	bored := shape(t, b, map[string]any{"op": "cut", "a": joined.Handle, "b": center.Handle})
	near(t, joined.GeometryAssertions.Volume-bored.GeometryAssertions.Volume, math.Pi*24*24*12)
	var mounts []string
	for _, xy := range [][2]float64{{80, 70}, {-80, 70}, {-80, -70}, {80, -70}} {
		hole := shape(t, b, map[string]any{"op": "circle", "origin": vector{xy[0], xy[1], 0}, "normal": vector{0, 0, 1}, "radius": 3.2, "height": 33})
		mounts = append(mounts, hole.Handle)
	}
	finished := shape(t, b, map[string]any{"op": "cutMany", "a": bored.Handle, "tools": mounts})
	near(t, bored.GeometryAssertions.Volume-finished.GeometryAssertions.Volume, 4*math.Pi*3.2*3.2*12)
	origin, normal := vector{0, 0, 12}, vector{0, 0, 1}
	rim, err := b.shapes[finished.Handle].holeRim(request{Center: &origin, Normal: &normal, Radius: 24})
	if err != nil || rim == nil || rim.Radius != 24 {
		t.Fatal("retained center rim", rim, err)
	}
	m, err := b.shapes[finished.Handle].tessellate(request{
		LinearDeflection: optionalNumber{value: .5, set: true}, AngularDeflection: optionalNumber{value: .2, set: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
