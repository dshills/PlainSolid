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

func TestSphereOffAxisCylinderUnion(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	const R, r, d, lo, hi = 5., 1., 2., -1., 5.5
	out, e := SphereOffAxisCylinderUnion(ctx, f, R, r, d, lo, hi, 1e-9)
	if e != nil || len(out.SphericalFaces) != 8 || len(out.CylindricalFaces) != 4 || len(out.CapFaces) != 4 {
		t.Fatal(e, out)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || p.Centroid.X <= 0 || p.Centroid.Z <= 0 || math.Abs(p.Centroid.Y) > 1e-10 {
		t.Fatal(p, e)
	}
	c, e := SphereOffAxisCylinderCommon(ctx, f, R, r, d, lo, hi, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	common, e := c.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || math.Abs(p.Volume+common.Volume-(4*math.Pi*R*R*R/3+math.Pi*r*r*(hi-lo))) > 1e-8 {
		t.Fatal(p.Volume, common.Volume, e)
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
		t.Fatal("native reload changed union", rp, e)
	}
	m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("invalid certified union mesh", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("union mesh contacts", contacts, e)
	}
	for _, c := range []struct{ lo, hi float64 }{{-5, 6}, {0, 4.5}, {0, 4.75}} {
		s, e := SphereOffAxisCylinderUnion(ctx, f, R, r, d, c.lo, c.hi, 1e-9)
		if !errors.Is(e, geom.ErrConvergence) || len(s.Solid.Faces()) != 0 {
			t.Fatal(c, e)
		}
	}
}
