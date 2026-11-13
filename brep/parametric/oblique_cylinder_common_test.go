package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliqueCylinderCommon(t *testing.T) {
	ctx := context.Background()
	a, b := geom.Vec3{X: 2}, geom.Vec3{X: 3, Y: 4}
	c := geom.Vec3{X: 1, Y: -2, Z: 3}
	caps := [2]float64{-3, 3}
	for _, rr := range [][2]float64{{1, 2}, {2, 1}, {2, 2}} {
		r, e := ObliqueCylinderCommon(ctx, c, a, b, rr[0], rr[1], caps, caps, 1e-9)
		if e != nil || !r.CapCoverage.Covers || len(r.CylinderFaces[0]) != 2 || len(r.CylinderFaces[1]) != 2 {
			t.Fatal(rr, e, r)
		}
		p, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || p.Centroid.Sub(c).Length() > 1e-8 {
			t.Fatal(rr, p, e)
		}
		const n = 2048
		var volume float64
		s, l := min(rr[0], rr[1]), max(rr[0], rr[1])
		for i := range n {
			u := ((float64(i)+.5)/n - .5) * math.Pi
			sn, cs := math.Sincos(u)
			volume += 5 * s * s * cs * cs * math.Sqrt(l*l-s*s*sn*sn) * math.Pi / n
		}
		if math.Abs(volume-p.Volume) > 1e-8 {
			t.Fatal(rr, volume, p.Volume)
		}
		if rr != [2]float64{1, 2} {
			continue
		}
		var buf bytes.Buffer
		if e := WriteJSON(&buf, r.Solid); e != nil {
			t.Fatal(e)
		}
		reloaded, e := ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		rp, e := reloaded.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || rp != p {
			t.Fatal("general cylinder reload changed mass", rp, e)
		}
		m, e := reloaded.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true, MaxSubdivisions: 32})
		if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
			t.Fatal("invalid general cylinder mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal("general cylinder mesh contacts", x, e)
		}
	}
}
