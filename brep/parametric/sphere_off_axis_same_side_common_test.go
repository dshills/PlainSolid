package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestSphereOffAxisCylinderSameSideCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, ends := range [][2]float64{{4.125, 4.75}, {-4.75, -4.125}, {4.5, 4.875}} {
		lo, hi := ends[0], ends[1]
		out, e := SphereOffAxisCylinderSameSideCommon(ctx, f, 5, 1, 2, lo, hi, 1e-9)
		if e != nil || !out.Incidence.Complete || len(out.CylindricalFaces) != 4 || len(out.SphericalFaces) != 4 {
			t.Fatal(ends, e, len(out.CylindricalFaces), len(out.SphericalFaces))
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Centroid.Y) > 1e-10 || p.Centroid.Z <= lo || p.Centroid.Z >= hi {
			t.Fatal(ends, p, e)
		}
		const n = 1024
		var volume float64
		for i := range n {
			u := (float64(i) + .5) * 2 * math.Pi / n
			for j := range n {
				a := (float64(j) + .5) / n
				h := math.Sqrt(21 - a*a - 4*a*math.Cos(u))
				volume += max(0, min(hi, h)-max(lo, -h)) * a * 2 * math.Pi / (n * n)
			}
		}
		if math.Abs(p.Volume-volume) > 4e-6 {
			t.Fatal(ends, p.Volume, volume)
		}
		if lo != 4.125 {
			continue
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rp, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || rp != p {
			t.Fatal("native reload changed the same-side intersection", rp, e)
		}
		m, e := out.Solid.Tessellate(ctx, TessellationOptions{Deflection: .3})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid same-side mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal("same-side mesh has contacts", x, e)
		}
	}
}
