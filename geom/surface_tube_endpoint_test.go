package geom

import (
	"math/big"
	"testing"
)

func TestDistinctAxisTubeEndpointExactEquality(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	r, e := CertifySurfaceTubeEndpointJunction(a, b, ParameterRectangle{.2, .5, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{.5, .8, 0, 1}, SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 2, LeftEnd: 1, RightEnd: 0})
	if e != nil || !r.Connected || r.Separated {
		t.Fatal(r, e)
	}
}
func TestDistinctAxisTubeEndpointsSeparated(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, s, t - .5} })
	r, e := CertifySurfaceTubeEndpointJunction(a, b, ParameterRectangle{.2, .4, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{.6, .8, 0, 1}, SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 2, LeftEnd: 1, RightEnd: 0})
	if e != nil || r.Connected || !r.Separated {
		t.Fatal(r, e)
	}
}
func TestTubeEndpointOverlapNotEquality(t *testing.T) {
	poles := []Vec3{}
	for i := 0; i < 2; i++ {
		for j, z := range []float64{-.2, -.2, .8} {
			poles = append(poles, Vec3{float64(i), float64(j) / 2, z})
		}
	}
	source, _ := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	a, _ := NewSurfacePatch(source, SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, 0} })
	r, e := CertifySurfaceTubeEndpointJunction(a, b, ParameterRectangle{.35, .4, .4, .5}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{.4, .45, .4, .5}, ParameterRectangle{0, 1, 0, 1}, SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 0, LeftEnd: 1, RightEnd: 0})
	if e != nil || r.Connected || r.Separated {
		t.Fatal(r, e)
	}
}

func TestTubeEndpointNonbinaryRationalWitness(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 3 * v, 0} })
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, 1, t - .5} })
	r, e := CertifySurfaceTubeEndpointJunction(a, b, ParameterRectangle{.2, .5, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{0, 1, 0, 1}, ParameterRectangle{.5, .8, 0, 1}, SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 2, LeftEnd: 1, RightEnd: 0})
	if e != nil || !r.Connected || r.Separated {
		t.Fatal(r, e)
	}
	if r.ParametersLeft[1].Min == r.ParametersLeft[1].Max {
		t.Fatal("1/3 must not have a singleton float enclosure")
	}
}

func TestTubeEndpointExactCircleCornerHint(t *testing.T) {
	a, _ := NewSurfacePatch(quadraticContactSurface(t, -25.0/256), SurfacePatchOptions{})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, t, 0} })
	o := SurfaceTubeEndpointOptions{LeftFreeAxis: 0, RightFreeAxis: 1, LeftEnd: 1, RightEnd: 0, ExactParameterHint: [4]*big.Rat{big.NewRat(11, 16), big.NewRat(3, 4), big.NewRat(11, 16), big.NewRat(3, 4)}}
	r, e := CertifySurfaceTubeEndpointJunction(a, b, ParameterRectangle{.67, .6875, .72, .78}, ParameterRectangle{.6, .8, .65, .85}, ParameterRectangle{.66, .71, .75, .765}, ParameterRectangle{.6, .8, .65, .85}, o)
	if e != nil || !r.Connected || r.Separated {
		t.Fatal(r, e)
	}
	hint := o.ExactParameterHint
	hint[2] = big.NewRat(7, 10)
	budget := &polynomialBudget{maxOperations: 100000, maxBits: 16384}
	left, right := pairParameterRanges(SurfacePairParameterBox{ParameterRectangle{.67, .6875, .72, .78}, ParameterRectangle{.6, .8, .65, .85}}), pairParameterRanges(SurfacePairParameterBox{ParameterRectangle{.66, .71, .75, .765}, ParameterRectangle{.6, .8, .65, .85}})
	ok, e := verifyEndpointHint(a, b, left, right, 0, .6875, 1, .75, hint, budget)
	if e != nil || ok {
		t.Fatal("wrong hint accepted", ok, e)
	}
}
