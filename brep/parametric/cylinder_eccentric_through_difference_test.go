package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestFiniteCylinderPairEccentricThroughDifference(t *testing.T) {
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: -2}, Radius: 5, Min: -2, Max: 2}
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: 2.5, Z: 5}, Axis: geom.Vec3{Z: 3}, Radius: 3, Min: -3, Max: 2}
	out, err := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
	if err != nil || len(out.Solids[0].Faces()) != 16 || len(out.Solids[1].Faces()) != 0 {
		t.Fatal("expected one annular stock", err)
	}
	s := out.Solids[0]
	mass, err := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	wantCenter := geom.Vec3{X: 1 - 9.0/16, Y: 2 - 9.0/32, Z: 3}
	if err != nil || math.Abs(mass.Volume-128*math.Pi) > 1e-8 || math.Abs(mass.Area-160*math.Pi) > 1e-8 || mass.Centroid.Sub(wantCenter).Length() > 1e-9 {
		t.Fatal(mass, err)
	}
	m, err := s.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
	if err != nil || m.ValidateSolid() != nil {
		t.Fatal("closed display", err)
	}
	contacts, err := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("embedded display", err)
	}
	for _, x := range []float64{3, 3.25} {
		bad := cutter
		bad.Origin.X, bad.Origin.Y = x, 2
		out, err := FiniteCylinderPairDifference(ctx, stock, bad, 1e-8)
		if !errors.Is(err, geom.ErrDomain) || len(out.Solids[0].Faces()) != 0 {
			t.Fatal("tangent or clipped radial cut escaped proof", x, err)
		}
	}
}
