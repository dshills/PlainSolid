package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestBuildProjectiveParallelContactBandsKeepsExactOriginalSigns(t *testing.T) {
	ctx := context.Background()
	var owners [2]*SurfacePatch
	for i := range owners {
		x, z, degree, knots := []float64{0, 3}, []float64{3, 3}, 1, []float64{0, 0, 3, 3}
		if i == 1 {
			// Original f(x) = 3+(x-1)(x-2), on the raw interval [0,3].
			x, z, degree, knots = []float64{0, 1.5, 3}, []float64{5, .5, 5}, 2, []float64{0, 0, 0, 3, 3, 3}
		}
		var poles []Vec3
		var weights []float64
		for u, h := range x {
			for v := range 2 {
				poles = append(poles, Vec3{h, float64(v), z[u]})
				weights = append(weights, 1+float64(i*v))
			}
		}
		raw, err := NewBSplineSurface(degree, 1, len(x), 2, poles, knots, []float64{0, 0, 1, 1}, weights)
		if err != nil {
			t.Fatal(err)
		}
		owners[i], err = NewSurfacePatch(raw, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	for order, pair := range [][2]*SurfacePatch{owners, {owners[1], owners[0]}} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			g, err := TraceProjectiveSurfaceContacts(ctx, pair[0], pair[1], axes, ProjectiveSurfaceContactGraphOptions{})
			if err != nil || !g.Complete || len(g.Contours) != 2 {
				t.Fatal("the two original transverse sections must be complete", order, axes, err)
			}
			bands, err := g.BuildParallelContactBands(ctx, SurfacePatchOptions{})
			if err != nil || !bands.Complete || bands.Axis != 0 || len(bands.Bands) != 3 || len(bands.Sections) != 2 {
				t.Fatal("ordered separating sections and all three material bands", order, axes, bands, err)
			}
			for i, s := range bands.Sections {
				if s.Station.Cmp(big.NewRat(int64(i+1), 3)) != 0 || len(s.Pieces) == 0 {
					t.Fatal("normalized thirds must retain exact independent physical sections x=1 and x=2", s)
				}
				for _, p := range s.Pieces {
					model, err := p.Spline.Model.patch.EvaluateRational(ctx, big.NewRat(2, 7), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
					if err != nil || model.Point[0].Cmp(big.NewRat(int64(i+1), 1)) != 0 || model.Point[2].Cmp(big.NewRat(3, 1)) != 0 {
						t.Fatal("section analytic piece changed its exact world law", i, model, err)
					}
				}
			}
			for i, b := range bands.Bands {
				want := []int{-1, 1, -1}[i] * (1 - 2*order)
				x := big.NewRat(int64(2*i+1), 2)
				d := new(big.Rat).Mul(new(big.Rat).Sub(x, big.NewRat(1, 1)), new(big.Rat).Sub(x, big.NewRat(2, 1)))
				d.Mul(d, big.NewRat(int64(2*order-1), 1))
				if b.Sign != want || b.Witness.Difference.Cmp(d) != 0 ||
					b.Common.U.Lower.Cmp(big.NewRat(int64(i), 3)) != 0 || b.Common.U.Upper.Cmp(big.NewRat(int64(i+1), 3)) != 0 {
					t.Fatal("the complete band must match the independent signed original parabola", order, axes, i, b)
				}
			}
			whole, err := g.BuildUnsplitParallelContactBands(ctx, SurfacePatchOptions{})
			if err != nil || !whole.Complete || len(whole.Sections) != 2 {
				t.Fatal("isolation-only partitions must recompose on both original supports", order, axes, err)
			}
			for i, s := range whole.Sections {
				for end := range 2 {
					p, err := s.Whole.Model.ExactPoint(float64(end))
					if err != nil || p != (Vec3{float64(i + 1), float64(end), 3}) {
						t.Fatal("one whole original model edge has the exact two physical terminals", order, axes, i, p, err)
					}
				}
			}
			short, err := g.BuildParallelContactBands(ctx, SurfacePatchOptions{MaxOperations: bands.ExactOperations - 1})
			if !errors.Is(err, ErrConvergence) || short.Complete || len(short.Sections)+len(short.Bands) != 0 {
				t.Fatal("one-short exact budget must leave the whole partition unpublished", short, err)
			}
		}
	}
	var controls []Vec3
	for _, x := range []float64{0, 3} {
		for _, y := range []float64{0, .375, 1} {
			controls = append(controls, Vec3{x, y, 3})
		}
	}
	knot, err := NewBSplineSurface(1, 1, 2, 3, controls, []float64{0, 0, 3, 3}, []float64{0, 0, .375, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewSurfacePatch(knot, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, owners[1], [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil {
		t.Fatal("original knot contact topology", err)
	}
	if r, err := g.BuildParallelContactBands(ctx, SurfacePatchOptions{}); err != nil || !r.Complete {
		t.Fatal("real support knot still has a complete piecewise proof", err)
	}
	if r, err := g.BuildUnsplitParallelContactBands(ctx, SurfacePatchOptions{}); !errors.Is(err, ErrConvergence) ||
		r.Complete || len(r.Sections) != 0 {
		t.Fatal("the unsplit native candidate cannot erase an original knot", r, err)
	}
}
