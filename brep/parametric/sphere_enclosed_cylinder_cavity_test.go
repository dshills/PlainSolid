package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSphereEnclosedCylinderCavity(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	r, e := SphereEnclosedCylinderCavity(ctx, center, 5, 1, geom.Vec2{X: 2, Y: .5}, -1, 2, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 4 || r.SphereFace != 0 || r.CylinderWall != 1 || r.LowerFloor != 2 || r.UpperFloor != 3 {
		t.Fatal(r)
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, r.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSON(&b, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []Solid{r.Solid, loaded} {
		m, e := s.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		coef, _ := r.VolumePiCoefficient.Float64()
		volume := math.Pi * (500.0/3 - 3)
		area := math.Pi * (100 + 6 + 2)
		if math.Abs(m.Volume-volume) > 1e-7 || math.Abs(m.Volume-math.Pi*coef) > 1e-7 || math.Abs(m.Area-area) > 1e-7 {
			t.Fatal(m, volume, area)
		}
		// Removed volume has center offset (2,.5,.5); signed centroid checks
		// cavity orientation independently of shell incidence.
		want := center.Sub(geom.Vec3{X: 2, Y: .5, Z: .5}.Scale(3 / (500.0/3 - 3)))
		if m.Centroid.Sub(want).Length() > 1e-8 {
			t.Fatal(m.Centroid, want)
		}
	}
	mesh, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .3, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal(e, len(contacts.Contacts))
	}
}
func TestSphereEnclosedCylinderCavityRejects(t *testing.T) {
	for _, c := range []struct{ r, d, lo, hi float64 }{{3, 2, -1, 1}, {1, 0, -5, 1}, {1, 0, 0, 0}} {
		r, e := SphereEnclosedCylinderCavity(context.Background(), geom.Vec3{}, 5, c.r, geom.Vec2{X: c.d}, c.lo, c.hi, 1e-9)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(c, e)
		}
	}
	r, e := SphereEnclosedCylinderCavity(context.Background(), geom.Vec3{X: .1}, 5, 1, geom.Vec2{X: .2}, -1, 1, 1e-9)
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("rounded placement", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SphereEnclosedCylinderCavity(ctx, geom.Vec3{}, 5, 1, geom.Vec2{}, -1, 1, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
