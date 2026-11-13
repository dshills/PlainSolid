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

func TestFiniteCoaxialCylinderThroughDifference(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: -2}, {Y: 2}, {Z: -2}} {
		a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: axis, Radius: 5, Min: -1, Max: 2}
		b := geom.RawCylinder{Origin: a.Origin, Axis: axis.Scale(-2), Radius: 3, Min: -1, Max: .5}
		for _, extend := range []bool{false, true} {
			if extend {
				b.Min = -2
				b.Max = 2
			}
			out, e := FiniteCoaxialCylinderThroughDifference(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			mass, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			center := a.Origin.Add(axis.Scale(.5))
			if math.Abs(mass.Volume-96*math.Pi) > 1e-8 || math.Abs(mass.Area-128*math.Pi) > 1e-8 || mass.Centroid.Sub(center).Length() > 1e-9 || out.VolumePiCoefficient.RatString() != "96" {
				t.Fatal(mass, out.VolumePiCoefficient)
			}
			seen := map[int]bool{}
			for _, ids := range [][]int{out.StockWallFaces, out.CutterWallFaces, out.LowerCapFaces, out.UpperCapFaces} {
				if len(ids) != 4 {
					t.Fatal(ids)
				}
				for _, id := range ids {
					if seen[id] {
						t.Fatal("duplicate")
					}
					seen[id] = true
				}
			}
			var buf bytes.Buffer
			if e = WriteJSON(&buf, out.Solid); e != nil {
				t.Fatal(e)
			}
			s, e := ReadJSON(&buf, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := s.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			edges := map[[2]int]bool{}
			for _, tri := range m.Triangles {
				for j, x := range tri {
					y := tri[(j+1)%3]
					if x > y {
						x, y = y, x
					}
					edges[[2]int{x, y}] = true
				}
			}
			if len(m.Vertices)-len(edges)+len(m.Triangles) != 0 {
				t.Fatal("genus")
			}
			contacts, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
			if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
				t.Fatal("contacts", e)
			}
		}
	}
}
func TestFiniteCoaxialCylinderThroughDifferenceReject(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Radius: 5, Min: -1, Max: 2}
	b := geom.RawCylinder{Axis: geom.Vec3{Z: -4}, Radius: 3, Min: -1, Max: .5}
	for _, mutate := range []func(*geom.RawCylinder){func(c *geom.RawCylinder) { c.Max = .25 }, func(c *geom.RawCylinder) { c.Origin.X = 1 }, func(c *geom.RawCylinder) { c.Radius = 5 }, func(c *geom.RawCylinder) { c.Axis.X = 1 }} {
		c := b
		mutate(&c)
		out, e := FiniteCoaxialCylinderThroughDifference(context.Background(), a, c, 1e-8)
		if e == nil || len(out.Solid.Faces()) != 0 || out.VolumePiCoefficient != nil {
			t.Fatal("partial", e)
		}
	}
	// The exact endpoint 1+2^-53 cannot be stored in native binary64 topology.
	a.Origin.Z = 1
	a.Axis.Z = math.Ldexp(1, -53)
	a.Min = 0
	a.Max = 1
	b.Origin.Z = 1
	b.Axis.Z = 1
	b.Min = -1
	b.Max = 1
	out, e := FiniteCoaxialCylinderThroughDifference(context.Background(), a, b, 1e-20)
	if !errors.Is(e, geom.ErrUnrepresentable) || len(out.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = FiniteCoaxialCylinderThroughDifference(ctx, a, b, 1e-8)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
