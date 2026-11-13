package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFiniteCylinderPairSetsRetainRawCaps(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 4, Y: -3, Z: 2}
	a := geom.RawCylinder{Origin: center.Add(geom.Vec3{X: 3, Y: 4}), Axis: geom.Vec3{X: -3, Y: -4}, Radius: 2, Min: -2, Max: 4}
	b := geom.RawCylinder{Origin: center.Add(geom.Vec3{X: -4, Y: 3}), Axis: geom.Vec3{X: 4, Y: -3}, Radius: 2, Min: -2, Max: 4}
	union, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	for owner, c := range [2]geom.RawCylinder{a, b} {
		for j, ids := range [2][]int{union.LowerCapFaces[owner], union.UpperCapFaces[owner]} {
			if len(ids) == 0 {
				t.Fatal("lost cap")
			}
			at := [2]float64{c.Min, c.Max}[j]
			for _, id := range ids {
				f := union.Solid.Faces()[id]
				p, e := f.Surface.Eval(0, 0)
				if e != nil || math.Abs(p.Sub(c.Origin).Dot(c.Axis)/25-at) > 1e-12 {
					t.Fatal(owner, at, p, e)
				}
			}
		}
	}
	m, e := union.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || math.Abs(m.Volume-(240*math.Pi-128.0/3)) > 1e-6 {
		t.Fatal(m, e)
	}
	difference, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
	if e != nil || difference.ContactNodes == nil || difference.MinimumGapSquared != nil {
		t.Fatal(e)
	}
	for i, s := range difference.Solids {
		p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || p.Volume <= 0 || (i == 0) != (p.Centroid.Sub(center).Dot(a.Axis) < 0) {
			t.Fatal(i, p, e)
		}
	}
	b.Radius = 3
	difference, e = FiniteCylinderPairDifference(ctx, a, b, 1e-8)
	if e != nil || difference.ContactNodes != nil || difference.MinimumGapSquared == nil || difference.MinimumGapSquared.RatString() != "20" {
		t.Fatal(e, difference.MinimumGapSquared)
	}
	b.Origin.Z++
	_, e = FiniteCylinderPairDifference(ctx, a, b, 1e-8)
	if !errors.Is(e, geom.ErrDomain) {
		t.Fatal("unsupported skew must fail", e)
	}
}
