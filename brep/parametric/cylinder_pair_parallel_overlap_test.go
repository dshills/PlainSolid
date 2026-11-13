package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestFiniteParallelTransverseSets(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 2}, {Y: -2}, {Z: 2}} {
		for _, reverse := range []bool{false, true} {
			a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: axis, Radius: 5, Min: -2, Max: 2}
			offset := geom.Vec3{X: 4}
			if axis.X != 0 {
				offset = geom.Vec3{Y: 4}
			}
			b := geom.RawCylinder{Origin: a.Origin.Add(offset), Axis: axis.Scale(2), Radius: 3, Min: -1, Max: 1}
			if reverse {
				b.Axis = b.Axis.Scale(-1)
			}
			u, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			d, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			common := 25*math.Acos(.8) + 4.5*math.Pi - 12
			for i, s := range []Solid{u.Solid, d.Solids[0]} {
				m, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil {
					t.Fatal(e)
				}
				area := 34*math.Pi - common
				lat := 10*(math.Pi-math.Acos(.8)) + 3*math.Pi
				if i == 1 {
					area = 25*math.Pi - common
				}
				if math.Abs(m.Volume-8*area) > 1e-6 || math.Abs(m.Area-(2*area+8*lat)) > 1e-6 {
					t.Fatal(i, m, area)
				}
				mesh, e := s.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
				if e != nil {
					t.Fatal(e)
				}
				if e = mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
				if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
					t.Fatal("embedded", i, e, len(contacts.Contacts))
				}
			}
			if u.CommonVolumeSquared != nil || u.CommonVolumePiCoefficient != nil || len(d.Solids[1].Faces()) != 0 {
				t.Fatal("false proof or split")
			}
			for owner, c := range []geom.RawCylinder{a, b} {
				for side, ids := range [][]int{u.LowerCapFaces[owner], u.UpperCapFaces[owner]} {
					tpar := c.Min
					if side == 1 {
						tpar = c.Max
					}
					point := c.Origin.Add(c.Axis.Scale(tpar))
					for _, id := range ids {
						f := u.Solid.Faces()[id]
						p, e := f.Surface.Eval(f.U.Min, f.V.Min)
						if e != nil || math.Abs(p.Sub(point).Dot(axis)) > 1e-12 {
							t.Fatal("raw cap order", owner, side, e)
						}
					}
				}
			}
		}
	}
}
func TestFiniteParallelTransverseRejects(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: 5, Min: 0, Max: 8}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Radius: 3, Min: -4, Max: 0}
	for _, bad := range []geom.RawCylinder{{Origin: b.Origin, Axis: b.Axis, Radius: 3, Min: -3, Max: 0}, {Origin: geom.Vec3{X: 8 - 1e-9}, Axis: b.Axis, Radius: 3, Min: -4, Max: 0}, {Origin: geom.Vec3{X: 2 + 1e-9}, Axis: b.Axis, Radius: 3, Min: -4, Max: 0}} {
		if r, e := FiniteCylinderPairUnion(context.Background(), a, bad, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("union accepted", e)
		}
		if r, e := FiniteCylinderPairDifference(context.Background(), a, bad, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
			t.Fatal("difference accepted", e)
		}
	}
}
