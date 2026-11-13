package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliqueEqualCylinderUnion(t *testing.T) {
	ctx := context.Background()
	for _, v := range []geom.Vec3{{X: 4, Y: 3}, {X: -4, Y: 3}, {Y: 3, Z: 4}} {
		u := geom.Vec3{X: 3, Y: 4}
		center := geom.Vec3{X: 1, Y: 2, Z: 3}
		c := [2]float64{-4, 4}
		out, e := ObliqueEqualCylinderUnion(ctx, center, u, v, 2, c, c, 1e-9)
		if e != nil || len(out.Solid.Faces()) != 16 || len(out.WallFaces[0]) != 4 {
			t.Fatal(v, e, out)
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		x, _ := out.CommonVolumeSquared.Float64()
		want := 2*math.Pi*4*40 - math.Sqrt(x)
		if e != nil || math.Abs(p.Volume-want) > 1e-7 || p.Centroid.Sub(center).Length() > 1e-8 {
			t.Fatal(v, p, want, e)
		}
		for j, ids := range out.WallFaces {
			n := [2]geom.Vec3{u, v}[j]
			for _, i := range ids {
				q, e := out.Solid.Faces()[i].Surface.Eval(.3, .4)
				d := q.Sub(center)
				if e != nil || math.Abs(d.Dot(d)-d.Dot(n)*d.Dot(n)/25-4) > 1e-10 {
					t.Fatal("cylindrical support", q, e)
				}
			}
		}
		if v.X != 4 {
			continue
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rp, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || rp != p {
			t.Fatal("reload changed oblique union", rp, e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: 3, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid display", e)
		}
		r, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !r.Complete || len(r.Contacts) != 0 {
			t.Fatal(r, e)
		}
	}
}
