package parametric

import (
	"context"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestFiniteCylinderPairCommon(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		x, z [2]float64
		want CylinderCommonConstruction
		l, u [2]bool
	}{
		{[2]float64{-6, 6}, [2]float64{-6, 6}, CylinderCommonIntersectingAxes, [2]bool{}, [2]bool{}},
		{[2]float64{-6, 2}, [2]float64{-6, 6}, CylinderCommonPerpendicularCap, [2]bool{}, [2]bool{true, false}},
		{[2]float64{-2, 6}, [2]float64{-6, 6}, CylinderCommonPerpendicularCap, [2]bool{true, false}, [2]bool{}},
		{[2]float64{-6, 6}, [2]float64{-6, 0}, CylinderCommonPerpendicularCap, [2]bool{}, [2]bool{false, true}},
		{[2]float64{-3, 2}, [2]float64{-6, 6}, CylinderCommonPerpendicularSlab, [2]bool{true, false}, [2]bool{true, false}},
		{[2]float64{0, 6}, [2]float64{-6, 0}, CylinderCommonPerpendicularQuadrant, [2]bool{true, false}, [2]bool{false, true}},
		{[2]float64{5, 6}, [2]float64{-6, 6}, CylinderCommonEmpty, [2]bool{}, [2]bool{}},
	} {
		a := geom.RawCylinder{Origin: geom.Vec3{Y: -1, Z: 2}, Axis: geom.Vec3{X: 1}, Radius: 5, Min: c.x[0], Max: c.x[1]}
		b := geom.RawCylinder{Origin: geom.Vec3{Y: -1, Z: 2}, Axis: geom.Vec3{Z: 1}, Radius: 5, Min: c.z[0], Max: c.z[1]}
		out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-8)
		if e != nil || out.Construction != c.want {
			t.Fatal(c, out.Construction, e)
		}
		if c.want == CylinderCommonEmpty {
			if len(out.Solid.Faces()) != 0 {
				t.Fatal(c, "not empty")
			}
			continue
		}
		seen := map[int]bool{}
		for j := range 2 {
			if (len(out.LowerCapFaces[j]) > 0) != c.l[j] || (len(out.UpperCapFaces[j]) > 0) != c.u[j] {
				t.Fatal(c, out.LowerCapFaces, out.UpperCapFaces)
			}
			for _, g := range [][]int{out.SideFaces[j], out.LowerCapFaces[j], out.UpperCapFaces[j]} {
				for _, id := range g {
					if seen[id] || id < 0 || id >= len(out.Solid.Faces()) {
						t.Fatal(c, id)
					}
					seen[id] = true
				}
			}
		}
		if len(seen) != len(out.Solid.Faces()) {
			t.Fatal(c, "incomplete ancestry")
		}
	}
	a := geom.RawCylinder{Axis: geom.Vec3{X: 2}, Radius: 3, Min: -5, Max: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{Z: 2}, Axis: geom.Vec3{X: 3, Y: 4}, Radius: 2, Min: -5, Max: 5}
	out, e := FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if e != nil || out.Construction != CylinderCommonSkewAxes || len(out.Solid.Faces()) != 4 {
		t.Fatal(out, e)
	}
	b.Min, b.Max = 0, 1
	out, e = FiniteCylinderPairCommon(ctx, a, b, 1e-9)
	if !errors.Is(e, geom.ErrConvergence) || len(out.Solid.Faces()) != 0 {
		t.Fatal(out, e)
	}
}
