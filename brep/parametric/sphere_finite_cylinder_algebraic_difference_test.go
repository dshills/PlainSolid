package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAlgebraicFiniteCylinderDifferenceBodies(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, lower := range []float64{-2, 0, 2} {
		r, e := SphereFiniteCylinderDifferenceAlgebraic(context.Background(), f, 5, 2, lower, 4.75, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Bodies) != 2 {
			t.Fatal("components")
		}
		sumLo, sumHi := new(big.Rat).Set(r.CommonProof.VolumePiInterval[0]), new(big.Rat).Set(r.CommonProof.VolumePiInterval[1])
		nativeSum := 0.
		for i, b := range r.Bodies {
			p, e := b.Solid.MassProperties(context.Background(), IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			lo, _ := b.VolumePiInterval[0].Float64()
			hi, _ := b.VolumePiInterval[1].Float64()
			if p.Volume <= 0 || math.Abs(p.Volume-math.Pi*(lo+hi)/2) > 1e-6 {
				t.Fatal("native volume", i, p.Volume, lo, hi)
			}
			sumLo.Add(sumLo, b.VolumePiInterval[0])
			sumHi.Add(sumHi, b.VolumePiInterval[1])
			nativeSum += p.Volume
			for _, id := range b.FloorFaces {
				if b.Solid.Faces()[id].Reversed != (i == 0) {
					t.Fatal("floor orientation", i)
				}
			}
			for _, id := range b.CylindricalFaces {
				if !b.Solid.Faces()[id].Reversed {
					t.Fatal("cavity orientation")
				}
			}
			if lower == 0 {
				m, e := b.Solid.Tessellate(context.Background(), TessellationOptions{Deflection: .15, MaxSubdivisions: 128, MaxTriangles: 100000})
				if e != nil {
					t.Fatal(e)
				}
				if e = m.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				if m.SignedVolume() <= 0 {
					t.Fatal("display orientation")
				}
			}
		}
		full := big.NewRat(500, 3)
		if sumLo.Cmp(full) > 0 || sumHi.Cmp(full) < 0 {
			t.Fatal("Boolean volume identity", sumLo, sumHi)
		}
		common, e := r.CommonProof.Solid.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(nativeSum+common.Volume-500*math.Pi/3) > 1e-6 {
			t.Fatal("native Boolean partition", e)
		}
	}
}
func TestAlgebraicDifferenceRejectsAmbiguity(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, z := range [][2]float64{{0, 4}, {0, 5}, {-5, 4.75}, {math.Sqrt(21), 4.75}, {0, math.Sqrt(21)}} {
		r, e := SphereFiniteCylinderDifferenceAlgebraic(context.Background(), f, 5, 2, z[0], z[1], 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e == nil || len(r.Bodies) != 0 {
			t.Fatal("accepted", z)
		}
	}
	r, e := SphereFiniteCylinderDifferenceAlgebraic(context.Background(), f, 5, 2, 0, 4.75, 1e-8, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || len(r.Bodies) != 0 {
		t.Fatal("partial")
	}
}
