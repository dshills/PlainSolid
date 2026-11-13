package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliqueSkewCylinderCommon(t *testing.T) {
	ctx := context.Background()
	for _, shift := range []float64{.5, 2, -2} {
		a := geom.RawCylinder{Origin: geom.Vec3{X: -2}, Axis: geom.Vec3{X: 2}, Radius: 3, Min: -4, Max: 6}
		b := geom.RawCylinder{Origin: geom.Vec3{X: -3, Y: -4, Z: shift}, Axis: geom.Vec3{X: 3, Y: 4}, Radius: 2, Min: -4, Max: 6}
		out, e := ObliqueSkewCylinderCommon(ctx, a, b, 1e-9)
		if e != nil || !out.Placement.Complete || len(out.CylinderFaces[0]) != 2 || len(out.CylinderFaces[1]) != 2 {
			t.Fatal(shift, e, out)
		}
		var local SteinmetzCommon
		if math.Abs(shift) < 1 {
			local, e = SkewOrthogonalCylinderCommon(ctx, geom.Vec3{}, 3, 2, shift, 5, 5, 1e-9)
		} else {
			local, e = SkewOrthogonalOverlapCommon(ctx, geom.Vec3{}, 3, 2, shift, 5, 5, 1e-9)
		}
		if e != nil {
			t.Fatal(e)
		}
		opts := IntegrationOptions{RelativeTolerance: 1e-7}
		p, e := out.Solid.MassProperties(ctx, opts)
		q, e2 := local.Solid.MassProperties(ctx, opts)
		if e != nil || e2 != nil || math.Abs(p.Volume-1.25*q.Volume) > 1e-8 || math.Abs(p.Centroid.Z-q.Centroid.Y) > 1e-9 {
			t.Fatal(shift, p, q, e, e2)
		}
		for j, ids := range out.CylinderFaces {
			source := [2]geom.RawCylinder{a, b}[j]
			for _, id := range ids {
				f := out.Solid.Faces()[id]
				x, e := f.Surface.Eval((f.U.Min+f.U.Max)/2, .25)
				d := x.Sub(source.Origin)
				projected := d.Dot(source.Axis)
				if e != nil || math.Abs(d.Dot(d)-projected*projected/source.Axis.Dot(source.Axis)-source.Radius*source.Radius) > 1e-10 {
					t.Fatal("face left owning cylinder", shift, j, x, e)
				}
			}
		}
		if shift != 2 {
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
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: 3, AnalyticBound: true, MaxSubdivisions: 32})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid oblique skew mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal(x, e)
		}
	}
	a := geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 3, Min: -4, Max: 4}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 1, Y: 1}, Radius: 2, Min: -6, Max: 6}
	if out, e := ObliqueSkewCylinderCommon(ctx, a, b, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || len(out.Solid.Faces()) != 0 {
		t.Fatal("irrational basis accepted by exact-map path", e)
	}
}
