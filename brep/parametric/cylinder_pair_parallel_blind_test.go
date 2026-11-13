package parametric

import (
	"context"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestFiniteCylinderBlindAssembly(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 2}, {Y: -2}, {Z: 2}} {
		a := geom.RawCylinder{Axis: axis, Radius: 5, Min: 0, Max: 4}
		offset := geom.Vec3{X: 1}
		if axis.X != 0 {
			offset = geom.Vec3{Y: 1}
		}
		b := geom.RawCylinder{Origin: offset, Axis: axis, Radius: 2, Min: -1, Max: 2}
		d, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		m, e := d.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		c, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !c.Complete || len(c.Contacts) != 0 {
			t.Fatal("embedding", e, len(c.Contacts))
		}
		for _, bad := range []geom.RawCylinder{
			{Origin: offset.Scale(3), Axis: axis, Radius: 2, Min: -1, Max: 2},
			{Origin: offset, Axis: axis, Radius: 2, Min: -1, Max: 4 - 1e-9},
			{Origin: offset, Axis: axis, Radius: 2, Min: -1, Max: 1e-9},
		} {
			r, e := FiniteCylinderPairDifference(ctx, a, bad, 1e-8)
			if e == nil || len(r.Solids[0].Faces()) != 0 {
				t.Fatal("unproved cut", e)
			}
		}
	}
}
