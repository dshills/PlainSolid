package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestSplineModelPointLocationWithTrims(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	top := 5
	q := []geom.Vec2{{X: .25, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .75}, {X: .25, Y: .75}}
	s, h, e := s.SplitFaceLoop(top, q)
	if e != nil {
		t.Fatal(e)
	}
	point := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: 1 + .125*x*y} }
	for _, c := range []struct {
		point geom.Vec3
		face  int
		want  SplineFacePointLocation
	}{{point(.5, .5), h.Outer, SplineFacePointOutside},
		{point(.5, .5), h.Inner, SplineFacePointInterior},
		{point(.25, .5), h.Outer, SplineFacePointOnTrim},
		{point(.125, .125), h.Outer, SplineFacePointInterior},
		{geom.Vec3{X: .125, Y: .125, Z: 1.01}, h.Outer, SplineFacePointOutside}} {
		r, e := s.LocateOnSplineFace(ctx, c.face, c.point, SplineFacePointOptions{})
		if e != nil || r.Location != c.want {
			t.Fatal(c, r, e)
		}
	}
	k := []float64{0, 0, 1, 1}
	affine, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 3}, {X: 3}, {X: 3, Y: 3}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	// 1/3 UV has no exact binary representation. It must be classified
	// without conversion to the ordinary floating UV evaluator.
	c, e := RectangularSplineFace(affine, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e := (Solid{faces: []Face{c.Face}}).LocateOnSplineFace(ctx, 0, geom.Vec3{X: 1, Y: 1}, SplineFacePointOptions{})
	if e != nil || r.Location != SplineFacePointInterior || r.UV[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	r, e = s.LocateRationalOnSplineFace(ctx, h.Outer, [3]*big.Rat{big.NewRat(1, 7), big.NewRat(1, 5), big.NewRat(281, 280)}, SplineFacePointOptions{})
	if e != nil || r.Location != SplineFacePointInterior || r.UV[0].Cmp(big.NewRat(1, 7)) != 0 {
		t.Fatal(r, e)
	}
}

func TestAlgebraicSplineFacePointAndTrimBox(t *testing.T) {
	ctx := context.Background()
	// X=u, Y=u+v+v², Z=2(v+v²). The rational target has
	// v=(sqrt(3)-1)/2, so no exact rational UV witness exists.
	var poles []geom.Vec3
	for i := range 2 {
		for _, y := range []float64{0, .5, 2} {
			poles = append(poles, geom.Vec3{X: float64(i), Y: float64(i) + y, Z: 2 * y})
		}
	}
	bs, e := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	f, e := RectangularSplineFace(bs, false)
	if e != nil {
		t.Fatal(e)
	}
	o := SplineFacePointOptions{Triangular: &geom.TriangularSurfaceInverseOptions{}}
	target := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 6), big.NewRat(1, 1)}
	s := Solid{faces: []Face{f.Face}}
	r, e := s.LocateRationalOnSplineFace(ctx, 0, target, o)
	if e != nil || r.Location != SplineFacePointInterior || r.Triangle == nil || !r.Triangle.ModelPointValidated ||
		r.Triangle.Parameters[1] != nil {
		t.Fatal(r, e)
	}
	s.faces[0].Holes = [][]Use{{
		{UVStart: geom.Vec2{X: .25, Y: .25}, UVEnd: geom.Vec2{X: .25, Y: .5}},
		{UVStart: geom.Vec2{X: .25, Y: .5}, UVEnd: geom.Vec2{X: .5, Y: .5}},
		{UVStart: geom.Vec2{X: .5, Y: .5}, UVEnd: geom.Vec2{X: .5, Y: .25}},
		{UVStart: geom.Vec2{X: .5, Y: .25}, UVEnd: geom.Vec2{X: .25, Y: .25}},
	}}
	r, e = s.LocateRationalOnSplineFace(ctx, 0, target, o)
	if e != nil || r.Location != SplineFacePointOutside || !r.Triangle.ModelPointValidated {
		t.Fatal(r, e)
	}
	target[0], target[1] = big.NewRat(1, 4), big.NewRat(3, 4)
	r, e = s.LocateRationalOnSplineFace(ctx, 0, target, o)
	if e != nil || r.Location != SplineFacePointOnTrim || !r.Triangle.ModelPointValidated {
		t.Fatal("algebraic point on an exact isoparametric trim", r, e)
	}
	hit, e := s.IntersectTriangularSplineFaceAxisLine(ctx, 0, 2, geom.Vec3{X: .25, Y: .75, Z: 4}, -3, 0, 2,
		geom.TriangularSurfaceInverseOptions{}, trimvalidate.Options{})
	if e != nil || !hit.Complete || !hit.Found || !hit.OnTrim || hit.ParameterLower.Cmp(big.NewRat(1, 1)) > 0 ||
		hit.ParameterUpper.Cmp(big.NewRat(1, 1)) < 0 {
		t.Fatal(hit, e)
	}
	hit, e = s.IntersectTriangularSplineFaceAxisLine(ctx, 0, 2, geom.Vec3{X: .25, Y: .75, Z: 4}, -3, 0, .5,
		geom.TriangularSurfaceInverseOptions{}, trimvalidate.Options{})
	if e != nil || !hit.Complete || hit.Found {
		t.Fatal("root lies outside bounded line", hit, e)
	}
}

func TestCoupledSplineFaceAxisLine(t *testing.T) {
	ctx := context.Background()
	q := [3]float64{0, 0, .25}
	var poles []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			u, v := float64(i)/2, float64(j)/2
			poles = append(poles, geom.Vec3{X: u + q[j], Y: v + q[i], Z: u + v})
		}
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	bs, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	f, e := RectangularSplineFace(bs, false)
	if e != nil {
		t.Fatal(e)
	}
	s := Solid{faces: []Face{f.Face}}
	r, e := s.IntersectSplineFaceAxisLine(ctx, 0, 2, geom.Vec3{X: .5, Y: .5}, 2, 0, 1,
		geom.RationalSurfaceInverseOptions{}, trimvalidate.Options{})
	// Symmetry gives u=v=t and t²/4+t=1/2; Z=2t and line speed is 2.
	residual := func(x *big.Rat) *big.Rat {
		y := new(big.Rat).Mul(x, x)
		y.Quo(y, big.NewRat(4, 1))
		return y.Add(y, new(big.Rat).Sub(x, big.NewRat(1, 2)))
	}
	if e != nil || !r.Complete || !r.Found || r.OnTrim || r.Projection == nil ||
		residual(r.ParameterLower).Sign() > 0 || residual(r.ParameterUpper).Sign() < 0 {
		t.Fatal(r, e)
	}
}
