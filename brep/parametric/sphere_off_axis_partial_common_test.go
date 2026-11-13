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

func TestSphereOffAxisCylinderPartialCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	const R, r, d, lo, hi = 5., 1., 2., -2., 4.25
	out, e := SphereOffAxisCylinderPartialCommon(ctx, f, R, r, d, lo, hi, 1e-9)
	if e != nil || len(out.SphericalFaces) != 2 || len(out.UpperCapMaterial.Events.Points) != 2 {
		t.Fatal(e, len(out.SphericalFaces), out.UpperCapMaterial.Kind)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || math.Abs(p.Centroid.Y) > 1e-10 {
		t.Fatal(p, e)
	}
	const n = 768
	var volume float64
	for i := range n {
		u := (float64(i) + .5) * 2 * math.Pi / n
		for j := range n {
			a := (float64(j) + .5) * r / n
			h := math.Sqrt(R*R - d*d - a*a - 2*d*a*math.Cos(u))
			volume += (min(hi, h) - lo) * a * 2 * math.Pi * r / (n * n)
		}
	}
	if math.Abs(p.Volume-volume) > 2e-6 {
		t.Fatal(p.Volume, volume)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, out.Solid); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	restored, e := reloaded.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || restored != p {
		t.Fatal("native reload changed the partial cap", restored, e)
	}
	m, e := out.Solid.Tessellate(ctx, TessellationOptions{Deflection: .3})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("partial common mesh invalid", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("partial common mesh contacts", contacts, e)
	}
	for _, h := range []float64{4, 4.75, 6} {
		q, e := SphereOffAxisCylinderPartialCommon(ctx, f, R, r, d, lo, h, 1e-9)
		if !errors.Is(e, geom.ErrConvergence) || len(q.Solid.Faces()) != 0 {
			t.Fatal("unsupported cap returned boundary", h, e)
		}
	}
}

func TestSphereOffAxisCylinderChordCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []struct {
		r, d, h float64
	}{
		{1, 2, 4.25}, // lens contains the cylinder center
		{1, 2, 4.75}, // lens excludes the cylinder center
		{1, 2, 4.875},
		{2, 1, 4.25}, // radical hub lies outside the two-center segment
	} {
		out, e := SphereOffAxisCylinderChordCommon(ctx, f, 5, c.r, c.d, -2, c.h, 1e-9)
		if e != nil || len(out.SphericalFaces) != 2 || len(out.UpperCapMaterial.Events.Points) != 2 {
			t.Fatal(c, e, out.UpperCapMaterial.Kind)
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || p.Volume <= 0 || math.Abs(p.Centroid.Y) > 1e-10 {
			t.Fatal(c, p, e)
		}
		if prior, e := SphereOffAxisCylinderPartialCommon(ctx, f, 5, c.r, c.d, -2, c.h, 1e-9); e == nil {
			pp, e := prior.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil || math.Abs(pp.Volume-p.Volume) > 1e-8 || math.Abs(pp.Area-p.Area) > 1e-8 || pp.Centroid.Sub(p.Centroid).Length() > 1e-8 {
				t.Fatal("the two charts disagree", c, pp, p, e)
			}
		}
		if c.h != 4.75 {
			continue
		}
		var buf bytes.Buffer
		if e := WriteJSON(&buf, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .3})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid chord-centered lens mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal("unexpected chord-centered lens mesh contacts", x, e)
		}
	}
}
