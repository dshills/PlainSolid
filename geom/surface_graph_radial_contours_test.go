package geom_test

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/testgraphfixture"
	"reflect"
	"testing"
)

func TestExactThreeRadialContourFactorIdentity(t *testing.T) {
	ctx := context.Background()
	a, b, e := testgraphfixture.TripleRadialGraphs()
	if e != nil {
		t.Fatal(e)
	}
	pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	radii := testgraphfixture.TripleRadialRadii()
	for order, pair := range [][2]*geom.SurfacePatch{{pa, pb}, {pb, pa}} {
		r, e := geom.CertifySplineGraphRadialContours(ctx, pair[0], pair[1], center, radii, geom.GraphPointContactOptions{})
		if e != nil || r == nil {
			t.Fatal(e)
		}
		signs := []int{-1, 1, -1, 1}
		if order == 1 {
			signs = []int{1, -1, 1, -1}
		}
		if !reflect.DeepEqual(r.BandSigns(), signs) {
			t.Fatal("original height sign", r.BandSigns())
		}
		if len(r.Radii()) != 3 || r.ExactOperations() <= 0 {
			t.Fatal("missing contours")
		}
		got := r.Radii()
		got[0].SetInt64(99)
		if r.Radii()[0].Cmp(radii[0]) != 0 {
			t.Fatal("mutable proof")
		}
		t.Log("proof operations", r.ExactOperations())
	}
	for _, bad := range [][]*big.Rat{radii[:2], {radii[0], radii[0], radii[2]}, {radii[0], radii[1], big.NewRat(1, 2)}, {radii[0], radii[1], big.NewRat(24, 64)}} {
		if r, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, center, bad, geom.GraphPointContactOptions{}); r != nil || e == nil {
			t.Fatal("unproved contour accepted", bad)
		}
	}
	if r, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, center, radii, geom.GraphPointContactOptions{MaxOperations: 1}); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := geom.CertifySplineGraphRadialContours(canceled, pa, pb, center, radii, geom.GraphPointContactOptions{}); r != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancellation", e)
	}
	wrong := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(33, 64)}
	if r, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, wrong, radii, geom.GraphPointContactOptions{}); r != nil || e == nil {
		t.Fatal("wrong center accepted")
	}
	height, e := testgraphfixture.TripleRadialHeightControls()
	if e != nil {
		t.Fatal(e)
	}
	height[0][0] += 1.0 / 1073741824
	knots := make([]float64, 18)
	for i := 9; i < 18; i++ {
		knots[i] = 1
	}
	var poles []geom.Vec3
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			poles = append(poles, geom.Vec3{X: float64(i) / 8, Y: float64(j) / 8, Z: height[i][j]})
		}
	}
	changed, e := geom.NewBSplineSurface(8, 8, 9, 9, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	changedPatch, e := geom.NewSurfacePatch(changed, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := geom.CertifySplineGraphRadialContours(ctx, pa, changedPatch, center, radii, geom.GraphPointContactOptions{}); r != nil || e == nil {
		t.Fatal("nonradial perturbation accepted")
	}
	enormous := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 10000))
	if r, e := geom.CertifySplineGraphRadialContours(ctx, pa, pb, center, []*big.Rat{enormous, radii[1], radii[2]}, geom.GraphPointContactOptions{MaxCoefficientBits: 64}); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("input bits", e)
	}

}
