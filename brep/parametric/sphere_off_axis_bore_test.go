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

func TestSphereOffAxisBlindBore(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -1, Z: 4}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	const R, r, offset, floor = 5., 2., 1., -.75
	out, e := SphereOffAxisBlindBore(ctx, f, R, r, offset, floor, 1e-9)
	if e != nil || len(out.SphericalFaces) != 4 || len(out.CylindricalFaces) != 4 || len(out.FloorFaces) != 4 {
		t.Fatal(e, out.SphericalFaces, out.CylindricalFaces, out.FloorFaces)
	}
	// The bore volume is the integral over the circular tool footprint of
	// sqrt(R²-x²-y²)-floor. Integrate in tool-centered polar coordinates,
	// independently of the kernel's sphere-centered seam chart.
	n := 512
	dAngle, dRadius := 2*math.Pi/float64(n), r/float64(n)
	var removed float64
	for i := range n {
		u := (float64(i) + .5) * dAngle
		for j := range n {
			a := (float64(j) + .5) * dRadius
			h := math.Sqrt(R*R - offset*offset - a*a - 2*offset*a*math.Cos(u))
			removed += (h - floor) * a * dAngle * dRadius
		}
	}
	for _, id := range out.FloorFaces {
		face := out.Solid.Faces()[id]
		a, b, e := face.Surface.Partials(face.U.Min+.4, .5)
		if e != nil || !face.Reversed || a.Cross(b).Dot(f.ZAxis()) >= 0 {
			t.Fatal("bore floor does not face void", e)
		}
	}
	mass := func(s Solid) {
		t.Helper()
		p, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Volume-(4*math.Pi*R*R*R/3-removed)) > 2e-5 {
			t.Fatal(p.Volume, removed, e)
		}
	}
	mass(out.Solid)
	box, e := out.Solid.Bounds()
	if e != nil || box.Min.X > -3 || box.Max.X < 6 || box.Min.Y > -6 || box.Max.Y < 4 || box.Min.Z > -1 || box.Max.Z < 9 {
		t.Fatal("boundary box omits a sphere extremum", box, e)
	}
	bound, e := out.Solid.DeflectionBound(16)
	if e != nil || bound > .18 || bound <= 0 {
		t.Fatal("invalid analytic display bound", bound, e)
	}
	m, e := out.Solid.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("invalid display", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("bore has mesh contacts", contacts, e)
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, out.Solid); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass(reloaded)
}

func TestSphereOffAxisBlindBoreRejectsUnsupportedProfiles(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, c := range []struct{ r, d, h float64 }{{2, 2, 0}, {2, 3, 0}, {3, 2, 0}, {3, 1, 3}, {3, 1, -3}} {
		out, e := SphereOffAxisBlindBore(context.Background(), f, 5, c.r, c.d, c.h, 1e-9)
		if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
			t.Fatal(c, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := SphereOffAxisBlindBore(ctx, f, 5, 2, 1, 0, 1e-9); !errors.Is(e, context.Canceled) || len(out.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
}
