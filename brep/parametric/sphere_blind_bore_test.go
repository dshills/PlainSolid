package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSphereBlindBoreTransverse(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, floor := range []float64{-2, 0, 2} {
		r, e := SphereBlindBore(context.Background(), f, 5, 3, floor, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if r.CrossingHeight != 4 {
			t.Fatal("exact crossing")
		}
		mass, e := r.Solid.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 4*math.Pi*125/3 - math.Pi*(9*(4-floor)+14.0/3)
		coefficient, _ := r.VolumePiCoefficient.Float64()
		area := 2*math.Pi*5*9 + 2*math.Pi*3*(4-floor) + math.Pi*9
		if math.Abs(mass.Volume-want) > 1e-6 || math.Abs(mass.Volume-math.Pi*coefficient) > 1e-6 || math.Abs(mass.Area-area) > 1e-6 {
			t.Fatal("analytic mass", mass, want, area)
		}
		m, e := r.Solid.Tessellate(context.Background(), TessellationOptions{Deflection: .15, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal("contacts", e, len(report.Contacts))
		}
	}
}
func TestSphereBlindBoreRejects(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, c := range [][3]float64{{5, 5, 0}, {5, 0, 0}, {-5, 3, 0}, {5, 3, 4}, {5, 3, -4}, {5, 2, 0}, {5, 3, math.Nextafter(4, 0)}} {
		r, e := SphereBlindBore(context.Background(), f, c[0], c[1], c[2], 1e-9)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("accepted", c)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SphereBlindBore(ctx, f, 5, 3, 0, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}
