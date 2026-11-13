package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestDiscoverGraphEllipseMixedDegreesAndKnots(t *testing.T) {
	a, _ := graphContactFixture(t, false, false)
	hint := GraphEllipseHint{Center: [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}, RadiusSquared: [2]*big.Rat{big.NewRat(1, 20), big.NewRat(1, 30)}}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			z := new(big.Rat)
			delta := big.NewRat(-1, 1)
			for axis, index := range []int{i, j} {
				degree := a.degreeU
				if axis == 1 {
					degree = a.degreeV
				}
				linear := big.NewRat(int64(index), int64(degree))
				quadratic := big.NewRat(int64(index*(index-1)), int64(degree*(degree-1)))
				z.Add(z, quadratic)
				center := hint.Center[axis]
				term := new(big.Rat).Add(new(big.Rat).Sub(quadratic, new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(2, 1), center), linear)), new(big.Rat).Mul(center, center))
				delta.Add(delta, new(big.Rat).Quo(term, hint.RadiusSquared[axis]))
			}
			a.net[i*a.countV+j][2].Add(z, delta)
		}
	}
	var poles []Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles = append(poles, Vec3{float64(i) / 2, float64(j) / 2, z})
		}
	}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, _ = a.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := DiscoverSplineGraphEllipse(context.Background(), pair[0], pair[1], GraphPointContactOptions{})
		if e != nil || !r.Proven {
			t.Fatal(r, e)
		}
		for axis := 0; axis < 2; axis++ {
			if r.Ellipse.Center[axis].Cmp(hint.Center[axis]) != 0 || r.Ellipse.RadiusSquared[axis].Cmp(hint.RadiusSquared[axis]) != 0 {
				t.Fatal(r.Ellipse)
			}
		}
	}
}
func TestDiscoverGraphEllipseRejectsOtherConicsAndLimits(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	r, e := DiscoverSplineGraphEllipse(context.Background(), a, b, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	a, b = graphContactFixture(t, false, false)
	r, e = DiscoverSplineGraphEllipse(context.Background(), a, b, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal("point contact accepted", r, e)
	}
	for i := range a.net {
		a.net[i][2].Sub(a.net[i][2], big.NewRat(1, 16))
	}
	r, e = DiscoverSplineGraphEllipse(context.Background(), a, b, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Ellipse.RadiusSquared[0].Cmp(big.NewRat(1, 16)) != 0 {
		t.Fatal(r, e)
	}
	r, e = DiscoverSplineGraphEllipse(context.Background(), a, b, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	// A mixed term makes a rotated conic, outside this axis-aligned subset.
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			a.net[i*a.countV+j][2].Add(a.net[i*a.countV+j][2], big.NewRat(int64(i*j), int64(a.degreeU*a.degreeV)))
		}
	}
	r, e = DiscoverSplineGraphEllipse(context.Background(), a, b, GraphPointContactOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
}
