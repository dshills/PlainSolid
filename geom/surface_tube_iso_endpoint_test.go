package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func isoJunctionFixture(t *testing.T, rightStart float64) (VerifiedSurfaceTube, VerifiedSurfaceTube) {
	t.Helper()
	var poles []Vec3
	for i, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			z := 0.
			if i == 2 {
				z = 1
			}
			poles = append(poles, Vec3{u, 3 * v, z})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, t} })
	build := func(ad, bd ParameterRectangle, axis int) VerifiedSurfaceTube {
		pa, e := a.Trim(ad)
		if e != nil {
			t.Fatal(e)
		}
		pb, e := b.Trim(bd)
		if e != nil {
			t.Fatal(e)
		}
		tube, e := NewVerifiedSurfaceTube(pa, pb, SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		return tube
	}
	left := build(ParameterRectangle{.4, .5, .02, .2}, ParameterRectangle{.3, .8, .1, .5}, 0)
	right := build(ParameterRectangle{.4, .8, .02, .2}, ParameterRectangle{.3, .9, rightStart, .4}, 3)
	return left, right
}
func TestIsoEndpointJunctionNonlinearFreeRelation(t *testing.T) {
	left, right := isoJunctionFixture(t, .25)
	r, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), left, right, true, false, ExactCurveSegmentOptions{})
	if e != nil || !r.Connected || r.Separated || !r.Left.Proven || !r.Right.Proven {
		t.Fatal(r, e)
	}
	want := [4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 12), big.NewRat(1, 2), big.NewRat(1, 4)}
	for i, x := range r.Left.Parameters {
		if x.Cmp(want[i]) != 0 || r.Right.Parameters[i].Cmp(want[i]) != 0 {
			t.Fatal(r)
		}
	}
	// The branch obeys B.v=A.u², so the free axes have no global affine link.
	r.Left.Parameters[1].SetInt64(9)
	r, e = CertifyTubeEndpointsByIsoIntersection(context.Background(), right, left, false, true, ExactCurveSegmentOptions{})
	if e != nil || !r.Connected || r.Right.Parameters[1].Cmp(big.NewRat(1, 12)) != 0 {
		t.Fatal(r, e)
	}
}
func TestIsoEndpointJunctionSeparationAndLimits(t *testing.T) {
	for _, start := range []float64{.3, math.Nextafter(.25, 1)} {
		left, right := isoJunctionFixture(t, start)
		r, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), left, right, true, false, ExactCurveSegmentOptions{})
		if e != nil || r.Connected || !r.Separated {
			t.Fatal(start, r, e)
		}
	}
	left, right := isoJunctionFixture(t, .25)
	_, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), left, right, true, false, ExactCurveSegmentOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifyTubeEndpointsByIsoIntersection(canceled, left, right, true, false, ExactCurveSegmentOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = CertifyTubeEndpointsByIsoIntersection(context.Background(), left, left, true, false, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestIsoEndpointJunctionCurvedLevelAndLine(t *testing.T) {
	var poles []Vec3
	for _, u := range []float64{0, 1} {
		for i, v := range []float64{0, .5, 1} {
			z := u
			if i == 2 {
				z++
			}
			poles = append(poles, Vec3{u, v, z})
		}
	}
	s, _ := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, t} })
	makeTube := func(ad, bd ParameterRectangle, axis int) VerifiedSurfaceTube {
		pa, e := a.Trim(ad)
		if e != nil {
			t.Fatal(e)
		}
		pb, e := b.Trim(bd)
		if e != nil {
			t.Fatal(e)
		}
		tube, e := NewVerifiedSurfaceTube(pa, pb, SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(axis, ad, bd, e)
		}
		return tube
	}
	left := makeTube(ParameterRectangle{.18, .1875, .2, .3}, ParameterRectangle{.16, .22, .18, .32}, 0)
	right := makeTube(ParameterRectangle{.17, .22, .24, .28}, ParameterRectangle{.17, .22, .25, .26}, 3)
	for i, pair := range [][2]VerifiedSurfaceTube{{left, right}, {right, left}} {
		r, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), pair[0], pair[1], i == 0, i == 1, ExactCurveSegmentOptions{})
		if e != nil || !r.Connected || r.ReversedCurves != (i == 1) {
			t.Fatal(i, r, e)
		}
		if r.Left.Point[0].Cmp(big.NewRat(3, 16)) != 0 || r.Left.Point[1].Cmp(big.NewRat(1, 4)) != 0 {
			t.Fatal(r)
		}
	}
}

func TestIsoEndpointCrossingDoesNotBypassOtherSurfaceResidual(t *testing.T) {
	left, right := isoJunctionFixture(t, .25)
	shifted, e := right.a.Transformed(Translation(Vec3{Z: 1.0 / 1024}))
	if e != nil {
		t.Fatal(e)
	}
	changed, e := NewVerifiedSurfaceTube(shifted, right.b, SurfacePairTubeOptions{FreeAxis: right.axis})
	if e != nil {
		t.Fatal(e)
	}
	// The selected left-A and right-B level curves still cross at exactly the
	// same rational point, but it is not on the changed right-A support.
	r, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), left, changed, true, false, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || r.Connected || r.Separated || len(r.Intersection.Hits) != 1 {
		t.Fatal(r, e)
	}
}

func TestIsoEndpointSameChartCircleLevels(t *testing.T) {
	a, _ := NewSurfacePatch(quadraticContactSurface(t, -25.0/256), SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	build := func(ad, bd ParameterRectangle, axis int) VerifiedSurfaceTube {
		pa, e := a.Trim(ad)
		if e != nil {
			t.Fatal(e)
		}
		pb, e := b.Trim(bd)
		if e != nil {
			t.Fatal(e)
		}
		tube, e := NewVerifiedSurfaceTube(pa, pb, SurfacePairTubeOptions{FreeAxis: axis})
		if e != nil {
			t.Fatal(e)
		}
		return tube
	}
	left := build(ParameterRectangle{.67, .6875, .72, .78}, ParameterRectangle{.6, .8, .65, .85}, 0)
	right := build(ParameterRectangle{.66, .71, .75, .765}, ParameterRectangle{.6, .8, .65, .85}, 1)
	result, e := CertifyTubeEndpointsByIsoIntersection(context.Background(), left, right, true, false, ExactCurveSegmentOptions{})
	if e != nil || !result.Connected || !result.FixedChartCoordinates || result.Left.Parameters[0].Cmp(big.NewRat(11, 16)) != 0 || result.Right.Parameters[1].Cmp(big.NewRat(3, 4)) != 0 {
		t.Fatal(result, e)
	}
	nearby := build(ParameterRectangle{.66, .71, math.Nextafter(.75, 1), .765}, ParameterRectangle{.6, .8, .65, .85}, 1)
	result, e = CertifyTubeEndpointsByIsoIntersection(context.Background(), left, nearby, true, false, ExactCurveSegmentOptions{})
	if !errors.Is(e, ErrConvergence) || result.Connected {
		t.Fatal(result, e)
	}
}
