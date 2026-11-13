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

func TestFiniteCylinderPairUnequalCrossingUnion(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	a := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: center, Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -2, Max: 2}
	for _, p := range [][2]geom.RawCylinder{{a, b}, {b, a}} {
		u, e := FiniteCylinderPairUnion(ctx, p[0], p[1], 1e-8)
		if e != nil || u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient != nil {
			t.Fatal(e, u.CommonVolumeSquared, u.CommonVolumePiCoefficient)
		}
		for owner, c := range p {
			for cap, ids := range [2][]int{u.LowerCapFaces[owner], u.UpperCapFaces[owner]} {
				if len(ids) == 0 {
					t.Fatal("lost cap", owner, cap)
				}
				for _, id := range ids {
					f := u.Solid.Faces()[id]
					v, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, .5)
					if e != nil || v.Sub(c.Origin).Dot(c.Axis) != [2]float64{c.Min, c.Max}[cap]*c.Axis.Dot(c.Axis) {
						t.Fatal(owner, cap, v, e)
					}
				}
			}
		}
		m, e := u.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || m.Centroid.Sub(center).Length() > 1e-8 {
			t.Fatal(m, e)
		}
	}
}

func TestFiniteCylinderPairUnchangedOrCoveredDifference(t *testing.T) {
	ctx := context.Background()
	stock := geom.RawCylinder{Axis: geom.Vec3{Z: -2}, Radius: 2, Min: -1, Max: 1}
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: 3}, Radius: 1, Min: -1, Max: 1}
	d, e := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9)
	if e != nil || len(d.Solids[0].Faces()) != 3 || len(d.Solids[1].Faces()) != 0 || len(d.StockCapFaces[0]) != 2 || len(d.CutterWallFaces[0]) != 0 || d.ContactNodes != nil || d.MinimumGapSquared != nil {
		t.Fatal(d, e)
	}
	p, e := d.Solids[0].MassProperties(ctx, IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-16*math.Pi) > 1e-9 {
		t.Fatal(p, e)
	}
	stock = geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: 2}, Radius: 2, Min: -2, Max: 2}
	cutter = geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 4, Z: 4}, Axis: geom.Vec3{Z: -4}, Radius: 5, Min: -.5, Max: .5}
	d, e = FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9)
	if e != nil || len(d.Solids[0].Faces())+len(d.Solids[1].Faces()) != 0 {
		t.Fatal(d, e)
	}
}

func TestFiniteCylinderPairUnequalCrossingDifference(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 1, Y: 2, Z: 3}
	stock := geom.RawCylinder{Origin: center.Add(geom.Vec3{Z: 2}), Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -1, Max: 3}
	cutter := geom.RawCylinder{Origin: center.Add(geom.Vec3{Y: 3}), Axis: geom.Vec3{Y: 3}, Radius: 3, Min: -3, Max: 1}
	d, err := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	faces := d.Solids[0].Faces()
	if err != nil || len(faces) != 26 || len(d.Solids[1].Faces()) != 0 || d.ContactNodes != nil || d.MinimumGapSquared != nil {
		t.Fatal(d, err)
	}
	if len(d.StockWallFaces[0]) != 12 || len(d.CutterWallFaces[0]) != 2 || len(d.StockCapFaces[0]) != 12 {
		t.Fatal("lost ancestry")
	}
	for i, id := range d.StockCapFaces[0] {
		f := faces[id]
		p, err := f.Surface.Eval((f.U.Min+f.U.Max)/2, .5)
		raw := [2]float64{stock.Min, stock.Max}[i/6]
		if err != nil || p.Sub(stock.Origin).Dot(stock.Axis) != raw*stock.Axis.Dot(stock.Axis) {
			t.Fatal(i, raw, p, err)
		}
	}
	for _, id := range d.CutterWallFaces[0] {
		f := faces[id]
		p, err := f.Surface.Eval(0, 0)
		du, dv, derivativeErr := f.Surface.Partials(0, 0)
		normal := du.Cross(dv)
		if f.Reversed {
			normal = normal.Scale(-1)
		}
		radial := p.Sub(center)
		radial.Y = 0
		if err != nil || derivativeErr != nil || math.Abs(radial.Length()-3) > 1e-12 || normal.Dot(radial) >= 0 {
			t.Fatal(id, radial, normal, err, derivativeErr)
		}
	}
	for _, test := range []struct {
		cutter geom.RawCylinder
		want   error
	}{
		{geom.RawCylinder{Origin: cutter.Origin, Axis: cutter.Axis, Radius: 3, Min: -3, Max: 2}, geom.ErrDomain},
		{geom.RawCylinder{Origin: cutter.Origin.Add(geom.Vec3{X: 1}), Axis: cutter.Axis, Radius: 3, Min: -3, Max: 1}, geom.ErrDomain},
		{geom.RawCylinder{Origin: cutter.Origin, Axis: cutter.Axis, Radius: 2, Min: -3, Max: 1}, geom.ErrUnrepresentable},
	} {
		out, err := FiniteCylinderPairDifference(ctx, stock, test.cutter, 1e-8)
		if !errors.Is(err, test.want) || len(out.Solids[0].Faces()) != 0 || len(out.Solids[1].Faces()) != 0 {
			t.Fatal("unsupported cut produced a result", test.cutter, out, err)
		}
	}
}

func TestFiniteParallelCylinderPairContainmentSets(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Radius: 3, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 1}, Axis: geom.Vec3{Z: -3}, Radius: 2, Min: -1, Max: 1}
	out, e := FiniteCylinderPairUnion(ctx, b, a, 1e-9)
	if e != nil || len(out.WallFaces[0]) != 0 || len(out.WallFaces[1]) != 1 || out.CommonVolumePiCoefficient.RatString() != "24" {
		t.Fatal(out, e)
	}
	cut, e := FiniteCylinderPairDifference(ctx, b, a, 1e-9)
	if e != nil || len(cut.Solids[0].Faces())+len(cut.Solids[1].Faces()) != 0 {
		t.Fatal(cut, e)
	}
	b = geom.RawCylinder{Axis: geom.Vec3{Z: -4}, Radius: 3, Min: -1, Max: 1}
	out, e = FiniteCylinderPairUnion(ctx, a, b, 1e-9)
	if e != nil || len(out.WallFaces[0]) != 1 || len(out.WallFaces[1]) != 1 || out.LowerCapFaces[0][0] != out.UpperCapFaces[1][0] || out.UpperCapFaces[0][0] != out.LowerCapFaces[1][0] {
		t.Fatal(out, e)
	}
}
