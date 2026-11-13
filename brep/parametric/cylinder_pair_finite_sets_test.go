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

func TestFiniteCylinderPairSeparatedUnion(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Radius: 2, Min: -1, Max: 1}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -3}, Radius: 1, Min: -1, Max: 1}
	for _, x := range []geom.RawCylinder{b, {Origin: geom.Vec3{Z: 8}, Axis: geom.Vec3{Z: -3}, Radius: 1, Min: -1, Max: 1}} {
		u, e := FiniteCylinderPairUnion(ctx, a, x, 1e-9)
		if e != nil || u.CommonVolumeSquared.Sign() != 0 || len(u.Solid.Faces()) != 6 {
			t.Fatal(u, e)
		}
		p, e := u.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Volume-22*math.Pi) > 1e-9 {
			t.Fatal(p, e)
		}
		for side, id := range []int{u.LowerCapFaces[1][0], u.UpperCapFaces[1][0]} {
			f := u.Solid.Faces()[id]
			p, e := f.Surface.Eval(0, 0)
			if e != nil || p.Z != x.Origin.Z+[2]float64{3, -3}[side] {
				t.Fatal("raw reversed cap", p, e)
			}
		}
	}
	b.Origin.X = 3
	u, e := FiniteCylinderPairUnion(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrDomain) || len(u.Solid.Faces()) != 0 {
		t.Fatal("radial contact does not prove disjoint closures", u, e)
	}
}

func TestFiniteCylinderPairContainedUnion(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5}
	for inner, pair := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		u, e := FiniteCylinderPairUnion(ctx, pair[0], pair[1], 1e-9)
		if e != nil || u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient == nil || u.CommonVolumePiCoefficient.RatString() != "32" || len(u.Solid.Faces()) != 3 {
			t.Fatal(u, e)
		}
		for _, group := range [][2][]int{u.WallFaces, u.LowerCapFaces, u.UpperCapFaces} {
			if len(group[inner]) != 0 || len(group[1-inner]) != 1 {
				t.Fatal(group)
			}
		}
		p, e := u.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-100*math.Pi) > 1e-9 || p.Centroid.Sub(b.Origin).Length() > 1e-10 {
			t.Fatal(p, e)
		}
	}
}
