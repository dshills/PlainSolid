package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestObliqueAsymmetricTransverseSets(t *testing.T) {
	ctx := context.Background()
	L := 25*math.Acos(.8) + 4.5*math.Pi - 12
	a := geom.RawCylinder{Axis: geom.Vec3{X: 6, Z: 8}, Max: 4, Radius: 25}
	b := geom.RawCylinder{Origin: geom.Vec3{Y: 20}, Axis: geom.Vec3{X: -6, Z: -8}, Radius: 15}
	for _, tc := range []struct {
		name            string
		lo, hi, removed float64
		faces, caps     int
	}{{"cover", -5, 1, 8, 4, 0}, {"blind", -5, -1.5, 5, 10, 2}, {"interior", -3, -1, 4, 16, 4}} {
		t.Run(tc.name, func(t *testing.T) {
			c := b
			c.Min, c.Max = tc.lo, tc.hi
			r, e := FiniteCylinderPairDifference(ctx, a, c, 1e-8)
			if e != nil || len(r.Solids[0].Faces()) != tc.faces || len(r.CutterCapFaces[0]) != tc.caps {
				t.Fatal(r, e)
			}
			m, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
			want := 125 * (200*math.Pi - tc.removed*L)
			if e != nil || math.Abs(m.Volume-want) > 1e-3 {
				t.Fatal(m, e, want)
			}
			checkObliqueSourceCaps(t, r.Solids[0], r.StockCapFaces[0], a)
			checkObliqueSourceCaps(t, r.Solids[0], r.CutterCapFaces[0], c)
			if tc.name == "blind" {
				mesh, e := r.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: 2.5, AnalyticBound: true})
				if e != nil || mesh.ValidateSolid() != nil {
					t.Fatal("mesh", e)
				}
				hits, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
				if e != nil || !hits.Complete || len(hits.Contacts) != 0 {
					t.Fatal("embedding", e)
				}
			}
		})
	}
	for _, span := range [][2]float64{{-2.5, 0}, {-5, -1}, {-2.5, -.5}} {
		for _, swap := range []bool{false, true} {
			c := b
			c.Min, c.Max = span[0], span[1]
			first, second := a, c
			if swap {
				first, second = second, first
			}
			r, e := FiniteCylinderPairUnion(ctx, first, second, 1e-8)
			if e != nil {
				t.Fatal(span, swap, e)
			}
			want := 125 * (245*math.Pi - 5*L)
			if span[0] == -5 {
				want = 125 * (272*math.Pi - 6*L)
			} else if span[1] == -.5 {
				want = 125 * (236*math.Pi - 4*L)
			}
			m, e := r.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
			if e != nil || math.Abs(m.Volume-want) > 1e-3 {
				t.Fatal("union volume", span, swap, m, e, want)
			}
			var encoded bytes.Buffer
			if e := WriteJSON(&encoded, r.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSON(&encoded, JSONLimits{})
			if e != nil || len(loaded.Faces()) != len(r.Solid.Faces()) {
				t.Fatal("reload", e)
			}
			for owner, source := range []geom.RawCylinder{first, second} {
				for side, ids := range [][]int{r.LowerCapFaces[owner], r.UpperCapFaces[owner]} {
					param := source.Min
					if side == 1 {
						param = source.Max
					}
					for _, id := range ids {
						f := r.Solid.Faces()[id]
						p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
						expected := source.Origin.Add(source.Axis.Scale(param))
						if e != nil || math.Abs(p.Sub(expected).Dot(source.Axis)) > 1e-7 {
							t.Fatal("ordered source cap", span, swap, owner, side, id, p, e)
						}
					}
				}
			}
		}
	}
}
func checkObliqueSourceCaps(t *testing.T, s Solid, ids []int, c geom.RawCylinder) {
	t.Helper()
	for _, id := range ids {
		f := s.Faces()[id]
		p, e := f.Surface.Eval(f.U.at(.5), f.V.at(.5))
		lo := c.Origin.Add(c.Axis.Scale(c.Min))
		hi := c.Origin.Add(c.Axis.Scale(c.Max))
		if e != nil || math.Min(math.Abs(p.Sub(lo).Dot(c.Axis)), math.Abs(p.Sub(hi).Dot(c.Axis))) > 1e-7 {
			t.Fatal("source cap", id, p, e)
		}
	}
}

func TestObliqueAsymmetricSourceGuards(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{X: 6, Z: 8}, Max: 4, Radius: 25}
	b := geom.RawCylinder{Origin: geom.Vec3{Y: 20}, Axis: geom.Vec3{X: -6, Z: -8}, Min: -5, Max: -1.5, Radius: 15}
	a.Origin.X = .1
	b.Origin.X = .1
	if r, e := FiniteCylinderPairDifference(context.Background(), a, b, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("unrepresentable world cap", e)
	}
	a.Origin = geom.Vec3{}
	b.Origin = geom.Vec3{Y: 20}
	a.Axis = geom.Vec3{X: 1, Z: 1}
	b.Axis = geom.Vec3{X: -2, Z: -2}
	if r, e := FiniteCylinderPairUnion(context.Background(), a, b, 1e-8); e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("irrational similarity norm", e)
	}
}
