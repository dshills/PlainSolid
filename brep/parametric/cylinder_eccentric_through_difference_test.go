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
	// Internal tangency remains outside the strict manifold construction.
	tangent := cutter
	tangent.Origin.X, tangent.Origin.Y = 3, 2
	if out, err := FiniteCylinderPairDifference(ctx, stock, tangent, 1e-8); !errors.Is(err, geom.ErrDomain) || len(out.Solids[0].Faces()) != 0 {
		t.Fatal("internal tangency", err)
	}
	// The formerly unsupported clipped radial disk now uses the separately
	// qualified transverse covering-cut route. Its caps belong only to stock.
	transverse := tangent
	transverse.Origin.X = 3.25
	out, err = FiniteCylinderPairDifference(ctx, stock, transverse, 1e-8)
	if err != nil || len(out.Solids[0].Faces()) != 4 || len(out.Solids[1].Faces()) != 0 || len(out.StockWallFaces[0]) != 1 || len(out.CutterWallFaces[0]) != 1 || len(out.StockCapFaces[0]) != 2 || len(out.CutterCapFaces[0]) != 0 {
		t.Fatal("transverse covering cut ancestry", out, err)
	}
	const d = 2.25
	alpha := math.Acos((d*d + 25 - 9) / (10 * d))
	beta := math.Acos((d*d + 9 - 25) / (6 * d))
	lens := 25*alpha + 9*beta - .5*math.Sqrt((-d+8)*(d+2)*(d-2)*(d+8))
	mass, err = out.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil || math.Abs(mass.Volume-8*(25*math.Pi-lens)) > 1e-7 || math.Abs(mass.Centroid.Z-3) > 1e-9 {
		t.Fatal("independent transverse volume", mass, err)
	}
	for _, id := range out.StockCapFaces[0] {
		f := out.Solids[0].Faces()[id]
		p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
		if e != nil || (p.Z != -1 && p.Z != 7) {
			t.Fatal("source cap lost", id, p, e)
		}
	}
	// A genuine independent unresolved domain stays rejected: an arbitrarily
	// narrow radial crescent cannot be sewn at the requested tolerance.
	near := tangent
	near.Origin.X = math.Nextafter(3, 4)
	if out, err := FiniteCylinderPairDifference(ctx, stock, near, 1e-8); !errors.Is(err, geom.ErrDomain) || len(out.Solids[0].Faces()) != 0 {
		t.Fatal("unresolved crescent", err)
	}
}
