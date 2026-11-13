package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestUnequalPerpendicularAsymmetric(t *testing.T) {
	ctx := context.Background()
	base, e := UnequalPerpendicularCylinderUnion(ctx, geom.Vec3{}, 5, 3, 4, 6, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	same, e := UnequalPerpendicularCylinderUnionWindows(ctx, geom.Vec3{}, 5, 3, Interval{-4, 4}, Interval{-6, 6}, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	var a, b bytes.Buffer
	WriteJSON(&a, base.Solid)
	WriteJSON(&b, same.Solid)
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("symmetric serialization changed")
	}
	union, e := UnequalPerpendicularCylinderUnionWindows(ctx, geom.Vec3{}, 5, 3, Interval{-4, 7}, Interval{-8, 6}, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	diff, e := UnequalPerpendicularCylinderDifferenceWindows(ctx, geom.Vec3{}, 5, 3, Interval{-4, 7}, Interval{-8, 6}, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	bp, e := base.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	d0, e := UnequalPerpendicularCylinderDifference(ctx, geom.Vec3{}, 5, 3, 4, 6, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	dp, e := d0.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	for i, s := range []Solid{union.Solid, diff.Solid} {
		p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		v, area, mx, mz := dp.Volume+75*math.Pi, dp.Area+30*math.Pi, 412.5*math.Pi, 0.
		if i == 0 {
			v = bp.Volume + 93*math.Pi
			area = bp.Area + 42*math.Pi
			mz = -126 * math.Pi
		}
		if math.Abs(p.Volume-v) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Sub(geom.Vec3{X: mx / v, Z: mz / v}).Length() > 1e-8 {
			t.Fatal(i, p, v, area)
		}
		var encoded bytes.Buffer
		if e = WriteJSON(&encoded, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&encoded, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: 1, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
	// Signed raw placement maps local X to -Z and local Z to +Y.
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 3.5}
	cutter := geom.RawCylinder{Origin: stock.Origin, Axis: geom.Vec3{Y: 2}, Radius: 3, Min: -4, Max: 3}
	for _, pair := range [][2]geom.RawCylinder{{stock, cutter}, {cutter, stock}} {
		u, e := FiniteCylinderPairUnion(ctx, pair[0], pair[1], 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		p, e := u.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(p.Volume-(bp.Volume+93*math.Pi)) > 1e-7 {
			t.Fatal(e, p)
		}
	}
	d, e := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if e != nil || len(d.Solids[1].Faces()) != 0 {
		t.Fatal(e)
	}
	for _, w := range []Interval{{-3, 7}, {-4, 3}, {1, 7}} {
		bad, e := UnequalPerpendicularCylinderUnionWindows(ctx, geom.Vec3{}, 5, 3, w, Interval{-8, 6}, 1e-8)
		if e == nil || len(bad.Solid.Faces()) != 0 {
			t.Fatal("invalid caps")
		}
	}
}
