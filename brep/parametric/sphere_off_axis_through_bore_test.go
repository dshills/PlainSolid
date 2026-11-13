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

func TestSphereOffAxisThroughBore(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	out, e := SphereOffAxisThroughBore(ctx, f, 5, 1, 2, 1e-9)
	if e != nil || len(out.LowerSphericalFaces) != 4 || len(out.UpperSphericalFaces) != 4 || len(out.CylindricalFaces) != 4 {
		t.Fatal(e, out)
	}
	p, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || p.Centroid.X >= 0 || math.Abs(p.Centroid.Y)+math.Abs(p.Centroid.Z) > 1e-10 {
		t.Fatal(p, e)
	}
	// Each half of the removed material equals the corresponding half
	// removed by a floor-at-equator bore. This checks global orientation
	// and preservation of both annular exteriors.
	blind, e := SphereOffAxisBlindBore(ctx, f, 5, 1, 2, 0, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	bp, e := blind.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || math.Abs(p.Volume-(2*bp.Volume-500*math.Pi/3)) > 1e-8 {
		t.Fatal(p.Volume, bp.Volume, e)
	}
	m, e := out.Solid.Tessellate(ctx, TessellationOptions{Deflection: .8, AnalyticBound: true})
	if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
		t.Fatal("invalid through bore mesh", e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("through bore mesh has contacts", contacts, e)
	}
	var data bytes.Buffer
	if e := WriteJSON(&data, out.Solid); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(&data, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	rp, e := reloaded.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
	if e != nil || rp != p {
		t.Fatal(rp, p, e)
	}
	if out, e := SphereOffAxisThroughBore(ctx, f, 5, 2, 3, 1e-9); !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal("tangent through bore was not rejected", e)
	}
}
