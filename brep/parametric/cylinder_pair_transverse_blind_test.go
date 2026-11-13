package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestTransverseBlindCylinderDifference(t *testing.T) {
	ctx := context.Background()
	common := 25*math.Acos(.8) + 4.5*math.Pi - 12
	for _, lower := range []bool{false, true} {
		a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Min: 0, Max: 4, Radius: 5}
		b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: -5, Max: -1.5, Radius: 3}
		floor := 3.
		cut := 5.
		if lower {
			b.Min = -2.5
			b.Max = 1
			floor = 5
			cut = 5
		}
		r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if e != nil {
			t.Fatal(lower, e)
		}
		m, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(m.Volume-(200*math.Pi-cut*common)) > 1e-5 {
			t.Fatal(lower, m, e)
		}
		for _, id := range r.CutterCapFaces[0] {
			f := r.Solids[0].Faces()[id]
			p, e := f.Surface.Eval(.5, .5)
			if e != nil || math.Abs(p.Z-floor) > 1e-12 {
				t.Fatal("floor", p, e)
			}
		}
		mesh, e := r.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) > 0 {
			t.Fatal("self contact", e, len(contacts.Contacts))
		}
	}
}

func TestTransverseBlindCylinderFramesAndBounds(t *testing.T) {
	ctx := context.Background()
	L := 25*math.Acos(.8) + 4.5*math.Pi - 12
	wantV := 200*math.Pi - 5*L
	wantArea := 130*math.Pi + 5*(3*math.Pi-10*math.Acos(.8))
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, reverse := range []bool{false, true} {
				origin := [3]float64{1, 2, 3}
				toolOrigin := origin
				toolOrigin[(axis+1)%3] += 4
				n := [3]float64{}
				n[axis] = sign
				a := geom.RawCylinder{Origin: geom.Vec3{X: origin[0], Y: origin[1], Z: origin[2]}, Axis: geom.Vec3{X: 2 * n[0], Y: 2 * n[1], Z: 2 * n[2]}, Min: 0, Max: 4, Radius: 5}
				b := geom.RawCylinder{Origin: geom.Vec3{X: toolOrigin[0], Y: toolOrigin[1], Z: toolOrigin[2]}, Axis: geom.Vec3{X: -2 * n[0], Y: -2 * n[1], Z: -2 * n[2]}, Min: -5, Max: -1.5, Radius: 3}
				if reverse {
					b.Axis = b.Axis.Scale(-1)
					b.Min, b.Max = -b.Max, -b.Min
				}
				r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
				if e != nil {
					t.Fatal(axis, sign, reverse, e)
				}
				m, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
				unit := geom.Vec3{X: n[0], Y: n[1], Z: n[2]}
				wantCenter := (800*math.Pi - 5*L*5.5) / wantV
				if e != nil || math.Abs(m.Volume-wantV) > 1e-5 || math.Abs(m.Area-wantArea) > 1e-5 || math.Abs(m.Centroid.Sub(a.Origin).Dot(unit)-wantCenter) > 1e-6 {
					t.Fatal("independent integrals", axis, sign, reverse, m, e)
				}
				for _, id := range r.CutterCapFaces[0] {
					f := r.Solids[0].Faces()[id]
					u, v := f.U.at(.5), f.V.at(.5)
					p, e := f.Surface.Eval(u, v)
					du, dv, e2 := f.Surface.Partials(u, v)
					normal := du.Cross(dv)
					if f.Reversed {
						normal = normal.Scale(-1)
					}
					if e != nil || e2 != nil || math.Abs(p.Sub(a.Origin).Dot(unit)-3) > 1e-10 || normal.Dot(unit) <= 0 {
						t.Fatal("original cutter cap/normal", p, normal, e, e2)
					}
				}
			}
		}
	}
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Min: 0, Max: 8, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: 1}, Min: 3, Max: 10, Radius: 3}
	for _, floor := range []float64{math.Nextafter(0, 1), math.Nextafter(8, 0)} {
		b.Min = floor
		r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
		if e == nil || len(r.Solids[0].Faces()) != 0 {
			t.Fatal("unresolved thin slab accepted", e)
		}
	}
	b.Min, b.Max = 2, 6 // Fully internal transverse groove remains out of this one-ended slice.
	if r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("two-ended groove accepted", e)
	}
}
