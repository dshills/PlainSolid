package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRevolvePlanarPolygonPlainCADAxisEdit(t *testing.T) {
	ctx := context.Background()
	points := []geom.Vec3{{}, {X: 20}, {X: 20, Y: 10}, {Y: 10}}
	for _, c := range []struct {
		axis geom.Vec3
		r, h float64
	}{
		{geom.Vec3{Y: 2}, 20, 10},
		{geom.Vec3{X: 2}, 10, 20},
		{geom.Vec3{X: -2}, 10, 20},
	} {
		p, e := PreparePlanarPolygonRevolution(ctx, points, geom.Vec3{}, c.axis)
		if e != nil || len(p.Meridian) != 4 {
			t.Fatal(p, e)
		}
		for i, q := range p.Meridian {
			if p.Frame.ToWorld(geom.Vec3{X: q.X, Z: q.Y}) != points[i] {
				t.Fatal("lost input geometry")
			}
		}
		for _, angle := range []float64{math.Pi / 2, -math.Pi / 2} {
			s, e := RevolvePlanarPolygon(ctx, points, geom.Vec3{}, c.axis, angle)
			if e != nil {
				t.Fatal(c, angle, e)
			}
			if len(s.faces) != 5 || s.Topology().EulerCharacteristic != 2 {
				t.Fatal(s.Topology())
			}
			m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			want := math.Pi * c.r * c.r * c.h / 4
			if e != nil || math.Abs(m.Volume-want) > 1e-8 {
				t.Fatal(c, angle, m, want, e)
			}
		}
	}
}

func TestPreparePlanarPolygonRevolutionRejectsInvalidAxis(t *testing.T) {
	ctx := context.Background()
	points := []geom.Vec3{{Z: 1}, {X: 2, Z: 1}, {X: 2, Y: 3, Z: 1}, {Y: 3, Z: 1}}
	for _, c := range []struct {
		at, axis geom.Vec3
	}{
		{geom.Vec3{Z: 1}, geom.Vec3{X: 1, Y: 1}},
		{geom.Vec3{Z: 1}, geom.Vec3{Z: 1}},
		{geom.Vec3{}, geom.Vec3{Y: 1}},
		{geom.Vec3{X: 1, Z: 1}, geom.Vec3{Y: 1}},
	} {
		if out, e := PreparePlanarPolygonRevolution(ctx, points, c.at, c.axis); !errors.Is(e, geom.ErrDomain) || len(out.Meridian) != 0 {
			t.Fatal("invalid axis accepted", c, out, e)
		}
	}
	p, e := PreparePlanarPolygonRevolution(ctx, points, geom.Vec3{X: 3, Y: 10, Z: 1}, geom.Vec3{Y: -1})
	if e != nil || p.Meridian[0] != (geom.Vec2{X: 3, Y: 10}) || p.Meridian[2] != (geom.Vec2{X: 1, Y: 7}) {
		t.Fatal("offset axis and opposite radial orientation", p, e)
	}
	if out, e := PreparePlanarPolygonRevolution(ctx, points, geom.Vec3{X: -math.SmallestNonzeroFloat64, Z: 1}, geom.Vec3{Y: 1}); !errors.Is(e, geom.ErrUnrepresentable) || len(out.Meridian) != 0 {
		t.Fatal("lossy radius accepted", out, e)
	}
}
