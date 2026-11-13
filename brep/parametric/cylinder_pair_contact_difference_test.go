package parametric

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestFiniteCylinderDifferenceContactOnly(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			av, bv := [3]float64{}, [3]float64{}
			av[axis] = 2 * sign
			bv[axis] = -3 * sign
			stock := geom.RawCylinder{Axis: geom.Vec3{X: av[0], Y: av[1], Z: av[2]}, Min: 0, Max: 3, Radius: 2}
			for _, kind := range []string{"radial", "cap", "both"} {
				p := [3]float64{}
				p[axis] = 6 * sign
				if kind != "cap" {
					p[(axis+1)%3] = 3
				}
				if kind != "radial" {
					p[axis] = 9 * sign
				}
				cutter := geom.RawCylinder{Origin: geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, Axis: geom.Vec3{X: bv[0], Y: bv[1], Z: bv[2]}, Min: 0, Max: 1, Radius: 1}
				r, e := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9)
				if e != nil {
					t.Fatalf("%d %g %s: %v", axis, sign, kind, e)
				}
				if len(r.Solids[0].Faces()) != 3 || len(r.Solids[1].Faces()) != 0 || len(r.StockWallFaces[0]) != 1 || len(r.StockCapFaces[0]) != 2 || len(r.CutterWallFaces[0]) != 0 || len(r.CutterCapFaces[0]) != 0 || r.AxialCutCaps[0] != nil || r.AxialCutCaps[1] != nil || r.ContactNodes != nil || r.MinimumGapSquared != nil {
					t.Fatal("unexpected contact ancestry", r)
				}
				mass, e := r.Solids[0].MassProperties(ctx, IntegrationOptions{})
				if e != nil || math.Abs(mass.Volume-24*math.Pi) > 1e-8 {
					t.Fatal(mass, e)
				}
				want := [3]float64{}
				want[axis] = 3 * sign
				if mass.Centroid.Distance(geom.Vec3{X: want[0], Y: want[1], Z: want[2]}) > 1e-9 {
					t.Fatal("stock moved")
				}
				common, e := FiniteCylinderPairCommon(ctx, stock, cutter, 1e-9)
				if e != nil || len(common.Solid.Faces()) != 0 {
					t.Fatal("contact has volume", e)
				}
				if axis == 2 && sign == 1 && kind == "radial" {
					mesh, e := r.Solids[0].Tessellate(ctx, TessellationOptions{Deflection: .1})
					if e != nil || mesh.ValidateSolid() != nil {
						t.Fatal("unchanged mesh", e)
					}
					if _, e = FiniteCylinderPairUnion(ctx, stock, cutter, 1e-9); e == nil {
						t.Fatal("radial tangent union accepted")
					}
				}
			}
		}
	}
}
func TestFiniteCylinderDifferenceContactExactBoundary(t *testing.T) {
	s := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Min: 0, Max: 4, Radius: 2}
	for _, radial := range []bool{false, true} {
		for _, inside := range []bool{false, true} {
			c := geom.RawCylinder{Origin: geom.Vec3{Z: 4}, Axis: geom.Vec3{Z: 1}, Min: 0, Max: 1, Radius: 1}
			if radial {
				c.Origin = geom.Vec3{X: 3, Z: 1}
			}
			value := 4.
			if radial {
				value = 3
			}
			toward := math.Inf(1)
			if inside {
				toward = 0
			}
			value = math.Nextafter(value, toward)
			if radial {
				c.Origin.X = value
			} else {
				c.Origin.Z = value
			}
			r, e := FiniteCylinderPairDifference(context.Background(), s, c, 1e-9)
			if !inside {
				if e != nil || len(r.Solids[0].Faces()) != 3 {
					t.Fatal("separated neighbor", e)
				}
			} else if e == nil {
				t.Fatal("tiny positive overlap must not be classified as contact", r)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := s
	c.Origin.X = 4
	if _, e := FiniteCylinderPairDifference(ctx, s, c, 1e-9); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestFiniteCylinderContactKeepsExactSourceCapGate(t *testing.T) {
	s := geom.RawCylinder{Origin: geom.Vec3{Z: 1}, Axis: geom.Vec3{Z: 1}, Min: 0x1p-54, Max: 1, Radius: 2}
	c := geom.RawCylinder{Origin: geom.Vec3{X: 3}, Axis: geom.Vec3{Z: 1}, Min: 0, Max: 3, Radius: 1}
	r, e := FiniteCylinderPairDifference(context.Background(), s, c, 1e-9)
	if e == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("rounded original cap on contact path", e)
	}
}
