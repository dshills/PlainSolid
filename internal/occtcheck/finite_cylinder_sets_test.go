package occtcheck

import (
	"context"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOCCTFiniteRawCylinderSets(t *testing.T) {
	refs := runOracle(t, "finite-cylinder-sets")
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 7, Y: 1, Z: 2}, Axis: geom.Vec3{X: -3, Y: -4}, Radius: 2, Min: -2, Max: 4}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 4, Y: -3}, Radius: 2, Min: -2, Max: 4}
	union, e := parametric.FiniteCylinderPairUnion(ctx, a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	p, e := union.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	compare(t, refs["finite-raw-union"], p.Area, p.Volume, p.Centroid)
	for _, wide := range []bool{false, true} {
		prefix := "finite-raw-"
		if wide {
			b.Radius = 3
			prefix = "finite-raw-wide-"
		}
		difference, e := parametric.FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		if wide == (difference.MinimumGapSquared == nil) || wide == (difference.ContactNodes != nil) {
			t.Fatal("component contact semantics")
		}
		var area, volume float64
		var moment geom.Vec3
		for i, s := range difference.Solids {
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			suffix := "minus"
			if i == 1 {
				suffix = "plus"
			}
			compare(t, refs[prefix+suffix], mass.Area, mass.Volume, mass.Centroid)
			area += mass.Area
			volume += mass.Volume
			moment = moment.Add(mass.Centroid.Scale(mass.Volume))
		}
		compare(t, refs[prefix+"cut"], area, volume, moment.Scale(1/volume))
	}
}
