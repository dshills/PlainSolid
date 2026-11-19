//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

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
