package parametric

import (
	"context"
	"errors"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestTransverseInteriorCylinderNotch(t *testing.T) {
	ctx := context.Background()
	L := 25*math.Acos(.8) + 4.5*math.Pi - 12
	wantV := 200*math.Pi - 4*L
	wantArea := 130*math.Pi + 4*(3*math.Pi-10*math.Acos(.8)) + 2*L
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, reverse := range []bool{false, true} {
				n := [3]float64{}
				n[axis] = sign
				unit := geom.Vec3{X: n[0], Y: n[1], Z: n[2]}
				p := [3]float64{1, 2, 3}
				bp := p
				bp[(axis+1)%3] += 4
				a := geom.RawCylinder{Origin: geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, Axis: unit.Scale(2), Min: 0, Max: 4, Radius: 5}
				b := geom.RawCylinder{Origin: geom.Vec3{X: bp[0], Y: bp[1], Z: bp[2]}, Axis: unit.Scale(-2), Min: -3, Max: -1, Radius: 3}
				if reverse {
					b.Axis = b.Axis.Scale(-1)
					b.Min, b.Max = -b.Max, -b.Min
				}
				r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
				if e != nil {
					t.Fatal(axis, sign, reverse, e)
				}
				if len(r.Solids[0].Faces()) != 16 || len(r.Solids[1].Faces()) != 0 || len(r.CutterCapFaces[0]) != 4 || len(r.StockCapFaces[0]) != 6 {
					t.Fatal("lineage")
				}
				m, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-wantV) > 1e-5 || math.Abs(m.Area-wantArea) > 1e-5 || math.Abs(m.Centroid.Sub(a.Origin).Dot(unit)-4) > 1e-7 {
					t.Fatal(m, e)
				}
				for j, id := range r.CutterCapFaces[0] {
					f := r.Solids[0].Faces()[id]
					u, v := f.U.at(.5), f.V.at(.5)
					point, e := f.Surface.Eval(u, v)
					du, dv, e2 := f.Surface.Partials(u, v)
					normal := du.Cross(dv)
					if f.Reversed {
						normal = normal.Scale(-1)
					}
					raw := b.Min
					wantSign := 1.
					if j >= 2 {
						raw = b.Max
						wantSign = -1
					}
					expected := b.Origin.Add(b.Axis.Scale(raw))
					if e != nil || e2 != nil || math.Abs(point.Sub(expected).Dot(unit)) > 1e-10 || normal.Dot(b.Axis)*wantSign <= 0 {
						t.Fatal("original cap owner/normal", j, point, expected, normal, e, e2)
					}
				}
				if axis == 2 && sign == 1 && !reverse {
					mesh, e := r.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
					if e != nil || mesh.ValidateSolid() != nil {
						t.Fatal("mesh", e)
					}
					contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
					if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
						t.Fatal("embedding", e, len(contacts.Contacts))
					}
				}
			}
		}
	}
}
func TestTransverseInteriorNotchGuards(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Min: 0, Max: 8, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: 1}, Min: 2, Max: 6, Radius: 3}
	ctx := context.Background()
	for _, span := range [][2]float64{{2, math.Nextafter(2, 3)}, {math.Nextafter(0, 1), 6}, {2, math.Nextafter(8, 0)}} {
		c := b
		c.Min, c.Max = span[0], span[1]
		if r, e := FiniteCylinderPairDifference(ctx, a, c, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
			t.Fatal("thin accepted", span, e)
		}
	}
	c := b
	c.Origin.X = 2 // Internal tangency is outside the transverse assembler.
	if _, ok, e := finiteParallelTransverseBlind(ctx, a, c, 1e-8); ok || e != nil {
		t.Fatal("tangent accepted", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := FiniteCylinderPairDifference(canceled, a, b, 1e-8); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
