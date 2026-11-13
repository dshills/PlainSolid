package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestSphereOffAxisCylinderDoubleCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	const R, r, d = 5., 1., 2.
	for _, c := range []struct {
		lo, hi float64
		bands  int
	}{{-4.25, 4.25, 4}, {-4.25, 4.75, 6}, {-4.875, 4.5, 6}} {
		out, e := SphereOffAxisCylinderDoubleCommon(ctx, f, R, r, d, c.lo, c.hi, 1e-9)
		if e != nil || !out.AngularEvents.Complete || len(out.CylindricalFaces) != c.bands || len(out.CapMaterial[0].Events.Points) != 2 || len(out.CapMaterial[1].Events.Points) != 2 {
			t.Fatal(c, e, len(out.CylindricalFaces))
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Centroid.Y) > 1e-10 {
			t.Fatal(c, p, e)
		}
		// Integral over the original disk, using clipped positive and negative
		// sphere heights, independently of all angular edge and hub choices.
		const n = 768
		var volume float64
		for i := range n {
			u := (float64(i) + .5) * 2 * math.Pi / n
			for j := range n {
				a := (float64(j) + .5) * r / n
				h := math.Sqrt(R*R - d*d - a*a - 2*d*a*math.Cos(u))
				volume += (min(c.hi, h) - max(c.lo, -h)) * a * 2 * math.Pi * r / (n * n)
			}
		}
		if math.Abs(p.Volume-volume) > 4e-6 {
			t.Fatal(c, p.Volume, volume)
		}
		if c.bands == 4 || c.lo != -4.25 {
			continue
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, out.Solid); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rp, e := restored.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || rp != p {
			t.Fatal("native reload changed the double cap", rp, p, e)
		}
		m, e := out.Solid.Tessellate(ctx, TessellationOptions{Deflection: .35})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid asymmetric lens display", e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("double lens has contacts", contacts, e)
		}
	}
}
