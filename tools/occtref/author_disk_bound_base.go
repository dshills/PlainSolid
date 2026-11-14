//go:build ignore

// Canonical baseline only; independent alternate traversal is authored by
// reverse_disk_bounds.cxx through the pinned OCCT entity model.
package main

import (
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func main() {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	b := step.DiskHoleExtrusionBody{Name: "mixed eccentric cuts", Frame: f, Outer: trimmed.ExtrusionCircle{Radius: 5}, Height: 4}
	b.Holes = []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: -2}, Radius: 1}}
	b.Blind = []trimmed.ExtrusionBlindHole{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 2}, Radius: 1}, Depth: 2}}
	out, _ := os.Create("exchange/step/testdata/disk-bound-base.step")
	if e := step.WriteDiskHoleExtrusions(out, step.DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []step.DiskHoleExtrusionBody{b}}); e != nil {
		panic(e)
	}
}
