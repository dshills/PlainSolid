package parametric

import (
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestParallelTransverseCoveringDifference(t *testing.T) {
	ctx := context.Background()
	common := 25*math.Acos(.8) + 4.5*math.Pi - 12
	for _, axis := range []geom.Vec3{{X: 2}, {Y: -2}, {Z: 2}} {
		for _, reverse := range []bool{false, true} {
			a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: axis, Min: -2, Max: 2, Radius: 5}
			offset := geom.Vec3{X: 4}
			if axis.X != 0 {
				offset = geom.Vec3{Y: 4}
			}
			b := geom.RawCylinder{Origin: a.Origin.Add(offset), Axis: axis.Scale(2), Min: -2, Max: 3, Radius: 3}
			if reverse {
				b.Axis = b.Axis.Scale(-1)
			}
			r, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8)
			if e != nil {
				t.Fatal(axis, reverse, e)
			}
			m, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil || math.Abs(m.Volume-8*(25*math.Pi-common)) > 1e-6 {
				t.Fatal(m, e)
			}
			if len(r.Solids[1].Faces()) != 0 || len(r.StockWallFaces[0]) != 1 || len(r.CutterWallFaces[0]) != 1 || len(r.StockCapFaces[0]) != 2 || len(r.CutterCapFaces[0]) != 0 || r.AxialCutCaps[0] != nil || r.AxialCutCaps[1] != nil {
				t.Fatal("false cap ownership")
			}
			for i, id := range r.StockCapFaces[0] {
				f := r.Solids[0].Faces()[id]
				p, e := f.Surface.Eval(f.U.Min, f.V.Min)
				want := a.Origin.Add(a.Axis.Scale([]float64{a.Min, a.Max}[i]))
				if e != nil || p.Sub(want).Dot(axis) != 0 {
					t.Fatal("original stock cap", i, p, want, e)
				}
			}
			if axis.Z != 0 && !reverse {
				mesh, e := r.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
				if e != nil || mesh.ValidateSolid() != nil {
					t.Fatal("not closed", e)
				}
			}
		}
	}
}
func TestParallelCoveringDifferenceBoundaries(t *testing.T) {
	ctx := context.Background()
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Min: 0, Max: 2, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -3}, Min: -2, Max: 1, Radius: 3}
	if _, e := FiniteCylinderPairDifference(ctx, a, b, 1e-8); e != nil {
		t.Fatal(e)
	}
	for _, x := range []float64{2, 8} {
		c := b
		c.Origin.X = x
		if _, ok, e := finiteParallelOverlap(ctx, a, c, 1e-8, Difference); ok || e != nil {
			t.Fatal("tangent used transverse path", ok, e)
		}
	}
	c := b
	c.Min = -1 // only covers z<=3, missing positive stock end
	if r, e := FiniteCylinderPairDifference(ctx, a, c, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("partial-depth silently clipped", e)
	}
	c = b
	c.Axis.Z = -1
	c.Min = math.Nextafter(-4, 0)
	c.Max = 2
	c.Origin.Z = 1
	if r, e := FiniteCylinderPairDifference(ctx, a, c, 1e-8); e == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("nonrepresented source endpoint", e)
	}
}
