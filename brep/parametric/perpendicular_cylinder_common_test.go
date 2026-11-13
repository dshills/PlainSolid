package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPerpendicularCylinderCommonRadii(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 2, Y: -1, Z: 3}
	for _, r := range [][2]float64{{3, 2}, {2, 3}, {2.5, 2}} {
		out, e := PerpendicularCylinderCommonRadii(ctx, center, r[0], r[1], 4, 5, 1e-9)
		if e != nil || len(out.XAxisFaces) != 2 || len(out.ZAxisFaces) != 2 || len(out.Solid.Vertices()) != 4 {
			t.Fatal(r, e, len(out.Solid.Vertices()))
		}
		p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || p.Centroid.Sub(center).Length() > 1e-8 {
			t.Fatal(r, p, e)
		}
		const n = 2048
		small, large := min(r[0], r[1]), max(r[0], r[1])
		var volume float64
		for i := range n {
			u := ((float64(i)+.5)/n - .5) * math.Pi
			sn, cs := math.Sincos(u)
			volume += 4 * small * small * cs * cs * math.Sqrt(large*large-small*small*sn*sn) * math.Pi / n
		}
		if math.Abs(volume-p.Volume) > 1e-8 {
			t.Fatal(r, volume, p.Volume)
		}
		if r[0] != 3 {
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
			t.Fatal("native reload changed the common", rp, e)
		}
		m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid certified display", e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("common mesh contacts", contacts, e)
		}
	}
}
