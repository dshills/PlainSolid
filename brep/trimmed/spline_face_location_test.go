package trimmed

import (
	"context"
	"math/big"
	"testing"

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
