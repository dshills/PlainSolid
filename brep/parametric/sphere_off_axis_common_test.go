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

func TestSphereOffAxisCylinderCommon(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	const R, r, d = 5., 1., 2.
	for _, c := range []struct {
		name         string
		lo, hi       float64
		lower, upper bool
	}{
		{"two flat caps", -1, 2, false, false},
		{"lower sphere", -6, 2, true, false},
		{"upper sphere", -2, 6, false, true},
		{"two spherical caps", -6, 6, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, e := SphereOffAxisCylinderCommon(ctx, f, R, r, d, c.lo, c.hi, 1e-9)
			if e != nil || len(out.CylindricalFaces) != 4 || len(out.LowerEndFaces) != 4 || len(out.UpperEndFaces) != 4 {
				t.Fatal(e, out)
			}
			faces := out.Solid.Faces()
			if l, u := faces[out.LowerEndFaces[0]].Surface.(offsetCommonChart), faces[out.UpperEndFaces[0]].Surface.(offsetCommonChart); l.lowerSpherical != c.lower || u.upperSpherical != c.upper {
				t.Fatal("wrong end support")
			}
			const n = 256
			var volume float64
			for i := range n {
				u := (float64(i) + .5) * 2 * math.Pi / n
				for j := range n {
					a := (float64(j) + .5) * r / n
					h := math.Sqrt(R*R - d*d - a*a - 2*d*a*math.Cos(u))
					volume += (min(c.hi, h) - max(c.lo, -h)) * a * 2 * math.Pi * r / (n * n)
				}
			}
			p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
			if e != nil || math.Abs(p.Volume-volume) > 3e-6 || math.Abs(p.Centroid.Y) > 1e-10 {
				t.Fatal(p, volume, e)
			}
			if !c.lower || !c.upper {
				return
			}
			var buf bytes.Buffer
			if e = WriteJSON(&buf, out.Solid); e != nil {
				t.Fatal(e)
			}
			s, e := ReadJSON(&buf, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .55, AnalyticBound: true})
			if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
				t.Fatal("invalid mesh", e)
			}
			result, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
			if e != nil || !result.Complete || len(result.Contacts) != 0 {
				t.Fatal("common boundary has contacts", result, e)
			}
		})
	}
	out, e := SphereOffAxisCylinderCommon(ctx, f, R, r, d, -6, 4.25, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal("partial cap accepted", e)
	}
}
