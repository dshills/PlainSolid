package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestTransverseCommonCapCylinderUnion(t *testing.T) {
	ctx := context.Background()
	L := 25*math.Acos(.8) + 4.5*math.Pi - 12
	for _, upper := range []bool{false, true} {
		for _, swap := range []bool{false, true} {
			a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Min: 0, Max: 4, Radius: 5}
			b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: -2.5, Max: 0, Radius: 3}
			if upper {
				b.Min, b.Max = -4, -1.5
			}
			if swap {
				a, b = b, a
			}
			r, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(upper, swap, e)
			}
			if len(r.Solid.Faces()) != 12 {
				t.Fatal(len(r.Solid.Faces()))
			}
			m, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
			want := 245*math.Pi - 5*L
			area := 178*math.Pi - 2*L - 5*(10*math.Acos(.8)+3*math.Pi)
			if e != nil || math.Abs(m.Volume-want) > 1e-5 || math.Abs(m.Area-area) > 1e-5 {
				t.Fatal(m, e, want, area)
			}
			for owner, c := range []geom.RawCylinder{a, b} {
				for side, ids := range [][]int{r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
					raw := c.Min
					if side == 1 {
						raw = c.Max
					}
					want := c.Origin.Add(c.Axis.Scale(raw))
					for _, id := range ids {
						f := r.Solid.Faces()[id]
						p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
						if e != nil || math.Abs(p.Z-want.Z) > 1e-10 {
							t.Fatal("source cap", owner, side, p, want, e)
						}
					}
				}
			}
			for _, id := range []int{6, 7} {
				for owner := 0; owner < 2; owner++ {
					found := false
					for _, ids := range [][]int{r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
						for _, v := range ids {
							found = found || v == id
						}
					}
					if !found {
						t.Fatal("missing dual shared lens ancestry")
					}
				}
			}
			if !swap {
				mesh, e := r.Solid.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
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

func TestTransverseCommonCapUnionFramesAndGuards(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			xyz := func(axial, radial float64) geom.Vec3 {
				var x [3]float64
				x[axis] = axial
				x[(axis+1)%3] = radial
				return geom.Vec3{X: x[0], Y: x[1], Z: x[2]}
			}
			a := geom.RawCylinder{Origin: xyz(16, 2), Axis: xyz(2*sign, 0), Min: 0, Max: 4, Radius: 5}
			b := geom.RawCylinder{Origin: xyz(16, 6), Axis: xyz(-2*sign, 0), Min: -2.5, Max: 0, Radius: 3}
			r, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(axis, sign, e)
			}
			for owner, c := range []geom.RawCylinder{a, b} {
				for side, ids := range [][]int{r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
					raw := c.Min
					if side == 1 {
						raw = c.Max
					}
					want := c.Origin.Add(c.Axis.Scale(raw))
					for _, id := range ids {
						f := r.Solid.Faces()[id]
						p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
						if e != nil || math.Abs(p.Sub(want).Dot(xyz(1, 0))) > 1e-10 {
							t.Fatal("cap frame", axis, sign, owner, side, p, want, e)
						}
					}
				}
			}
		}
	}
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Max: 8, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: 1}, Max: 5, Radius: 3}
	cases := []geom.RawCylinder{b, b, b, b}
	cases[0].Max = math.Nextafter(8, 0) // unresolved tiny exposed shoulder
	cases[1].Min = 2
	cases[1].Max = 6                         // nested spans remain outside this supported subset
	cases[2].Origin.X = 8                    // external side tangency
	cases[3].Origin.X = math.Nextafter(8, 0) // transverse but below sewing resolution
	for i, c := range cases {
		r, e := FiniteCylinderPairUnion(ctx, a, c, 1e-8)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("unguarded", i, e)
		}
	}
	// Exact physical caps must not silently round during local recharting.
	a.Origin.Z = .1
	b.Origin.Z = .1
	if r, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("unrepresentable cap", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := FiniteCylinderPairUnion(canceled, a, b, 1e-8); e != context.Canceled || len(r.Solid.Faces()) != 0 {
		t.Fatal("cancellation", e)
	}
}
