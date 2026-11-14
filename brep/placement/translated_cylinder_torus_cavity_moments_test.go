package placement

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestTranslatedCylinderTorusCavityMeasures(t *testing.T) {
	ctx := context.Background()
	s, _ := geom.NewTranslatedCylinderTorusCavityMaterial(5, 2, 10, 0, 7, 1, 1, 3)
	r := big.NewRat
	tol := r(1, 1000000)
	m, e := BoundTranslatedCylinderTorusCavityMoments(ctx, s, TorusCylinderOccurrenceMomentOptions{Source: geom.TorusCylinderMomentOptions{VolumeTolerance: tol, SecondMomentTolerance: tol, CentroidTolerance: tol, InertiaTolerance: tol}})
	if e != nil {
		t.Fatal(e)
	}
	check := func(b [2]*big.Rat, want float64) {
		t.Helper()
		lo, _ := b[0].Float64()
		hi, _ := b[1].Float64()
		if want < lo-1e-9 || want > hi+1e-9 {
			t.Fatal(lo, hi, want)
		}
	}
	check(m.First[2], 2450*math.Pi-120*math.Pi*math.Pi)
	check(m.Second[0][2], -120*math.Pi*math.Pi)
	check(m.Second[2][2], (34300./3)*math.Pi-400*math.Pi*math.Pi)
	matrix := [3][4]*big.Rat{{r(1, 1), r(0, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(1, 1), r(0, 1), r(0, 1)}, {r(0, 1), r(0, 1), r(1, 1), r(0, 1)}}
	bounds, e := s.BoundAffineRange(ctx, matrix, 64, geom.SurfacePatchOptions{})
	if e != nil || bounds.Minimum[2][0].Sign() != 0 || bounds.Maximum[2][1].Cmp(r(7, 1)) != 0 {
		t.Fatal(bounds, e)
	}
}
