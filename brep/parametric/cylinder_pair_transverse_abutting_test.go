package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestTransverseAbuttingCylinderUnion(t *testing.T) {
	ctx := context.Background()
	L := 25*math.Acos(.8) + 4.5*math.Pi - 12
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			for _, swap := range []bool{false, true} {
				xyz := func(z, x float64) geom.Vec3 {
					var a [3]float64
					a[axis] = z
					a[(axis+1)%3] = x
					return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}
				}
				a := geom.RawCylinder{Axis: xyz(2*sign, 0), Max: 4, Radius: 5}
				b := geom.RawCylinder{Origin: xyz(0, 4), Axis: xyz(-2*sign, 0), Min: -6, Max: -4, Radius: 3}
				if swap {
					a, b = b, a
				}
				r, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
				if e != nil || len(r.Solid.Faces()) != 12 {
					t.Fatal(axis, sign, swap, e)
				}
				for owner, c := range []geom.RawCylinder{a, b} {
					for side, ids := range [][]int{r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
						raw := c.Min
						if side == 1 {
							raw = c.Max
						}
						want := c.Origin.Add(c.Axis.Scale(raw))
						if len(ids) == 0 {
							t.Fatal("missing cap")
						}
						for _, id := range ids {
							f := r.Solid.Faces()[id]
							p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
							du, dv, _ := f.Surface.Partials(f.U.at(.5), f.V.at(.5))
							n := du.Cross(dv)
							if f.Reversed {
								n = n.Scale(-1)
							}
							out := c.Axis
							if side == 0 {
								out = out.Scale(-1)
							}
							if e != nil || math.Abs(p.Sub(want).Dot(xyz(1, 0))) > 1e-10 || n.Dot(out) <= 0 {
								t.Fatal("cap ancestry/normal", axis, sign, swap, owner, side, id, p, want, e)
							}
						}
					}
				}
				if axis == 2 {
					m, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
					v := 236 * math.Pi
					area := 172*math.Pi - 2*L
					cz := sign * (1160 * math.Pi) / v
					if e != nil || math.Abs(m.Volume-v) > 1e-5 || math.Abs(m.Area-area) > 1e-5 || math.Abs(m.Centroid.Z-cz) > 1e-6 {
						t.Fatal(m, e, v, area, cz)
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
	}
}

func TestTransverseAbuttingContactSetSemantics(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Max: 4, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: -6, Max: -4, Radius: 3}
	for _, swap := range []bool{false, true} {
		stock, cutter := a, b
		if swap {
			stock, cutter = cutter, stock
		}
		d, e := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-8)
		if e != nil || len(d.CutterCapFaces[0]) != 0 || len(d.CutterWallFaces[0]) != 0 {
			t.Fatal("no-volume difference", e)
		}
		m, e := d.Solids[0].MassProperties(ctx, IntegrationOptions{})
		want := math.Pi * stock.Radius * stock.Radius * stock.Axis.Length() * (stock.Max - stock.Min)
		if e != nil || math.Abs(m.Volume-want) > 1e-7 {
			t.Fatal("retained stock", m, e, want)
		}
		c, e := FiniteCylinderPairCommon(ctx, stock, cutter, 1e-8)
		if e != nil || len(c.Solid.Faces()) != 0 {
			t.Fatal("zero-volume intersection", e)
		}
	}
	u, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8)
	if e != nil || u.CommonVolumePiCoefficient == nil || u.CommonVolumePiCoefficient.Sign() != 0 {
		t.Fatal("exact zero common", e)
	}
	for _, x := range []float64{8, 9} {
		c := b
		c.Origin.X = x
		r, ok, e := finiteParallelTransverseAbuttingUnion(ctx, a, c, 1e-8)
		if ok || e != nil || len(r.Solid.Faces()) != 0 {
			t.Fatal("joined radial contact/disjoint", x, ok, e)
		}
	}
	tangent := b
	tangent.Origin.X = 8
	if r, e := FiniteCylinderPairUnion(ctx, a, tangent, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("singular radial tangent", e)
	}
	near := b
	near.Max = math.Nextafter(-4, 0)
	if r, e := FiniteCylinderPairUnion(ctx, a, near, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("infinitesimal overlap", e)
	}
	a.Origin.Z = .1
	b.Origin.Z = .1
	if r, e := FiniteCylinderPairUnion(ctx, a, b, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("unrepresentable cap", e)
	}
}
