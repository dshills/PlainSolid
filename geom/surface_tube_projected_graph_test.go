package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestTubeProjectedGraphNonXY(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 0, 1, 1, 1}
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
			poles = append(poles, Vec3{float64(i) / 2, z, float64(j) / 2})
		}
	}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, .5, v} })
	a, e = a.Trim(ParameterRectangle{7.0 / 32, .25, 5.0 / 8, 11.0 / 16})
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Trim(ParameterRectangle{3.0 / 16, 9.0 / 32, 19.0 / 32, 23.0 / 32})
	if e != nil {
		t.Fatal(e)
	}
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{FreeAxis: 0})
	if e != nil {
		t.Fatal(e)
	}
	for _, axes := range [][2]int{{0, 2}, {2, 0}} {
		sign, _, e := AffineProjectedGraphOrientation(ctx, a, axes, SurfacePatchOptions{})
		want := 1
		if axes[0] == 2 {
			want = -1
		}
		if e != nil || sign != want {
			t.Fatal("projected orientation", sign, e)
		}
		endpoint, e := tube.DiscoverAlgebraicProjectedGraphEndpoint(ctx, true, axes, TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !endpoint.Proven {
			t.Fatal("projected endpoint", e)
		}
		free, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 4), ExactCurvePlaneEventOptions{})
		order, e := endpoint.Parameters[0].Compare(ctx, free, ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal("endpoint free", order, e)
		}
		r, e := tube.EvaluateRationalProjectedGraph(ctx, big.NewRat(15, 64), axes, TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !r.Proven {
			t.Fatal(r, e)
		}
		if r.PointBounds.Min.Y > .5 || r.PointBounds.Max.Y < .5 {
			t.Fatal("wrong restored XYZ", r.PointBounds)
		}
		binding, e := tube.BindProjectedGraphEvent(ctx, r.ModelEvent, axes, TubeAlgebraicGraphEndpointOptions{})
		if e != nil || binding.ModelEvent() != r.ModelEvent {
			t.Fatal(binding, e)
		}
		roots, _ := binding.Parameters()
		for i := range roots {
			order, e := roots[i].Compare(ctx, r.Parameters[i], ExactCurvePlaneEventOptions{})
			if e != nil || order.Order != 0 {
				t.Fatal(i, order, e)
			}
		}
		wrongRoot, _, _ := NewExactRationalRoot(ctx, big.NewRat(2, 3), ExactCurvePlaneEventOptions{})
		wrong, e := r.ModelEvent.cell.BindRoot(ctx, wrongRoot, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e := tube.BindProjectedGraphEvent(ctx, wrong, axes, TubeAlgebraicGraphEndpointOptions{}); e == nil {
			t.Fatal("false model event")
		}
	}
	if _, e := tube.EvaluateRationalGraph(ctx, big.NewRat(15, 64), TubeAlgebraicGraphEndpointOptions{}); e == nil {
		t.Fatal("original XY should not be affine graph")
	}
	if _, e := tube.EvaluateRationalProjectedGraph(ctx, big.NewRat(15, 64), [2]int{0, 0}, TubeAlgebraicGraphEndpointOptions{}); e == nil {
		t.Fatal("invalid axes")
	}
}
