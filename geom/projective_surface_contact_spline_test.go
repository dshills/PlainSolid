package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveContactSplineKeepsBothOriginalPcurvesAndCurvedModel(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u*v + u - v} })
	b := projectiveOverlapTestChart(t, false, false)
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil || !g.Complete || len(g.Contours) != 1 || len(g.Arcs) < 2 {
		t.Fatal("curved 3D section over an exact common straight projection did not trace", err, g)
	}
	for i := range g.Arcs {
		c, err := g.BuildArcSpline(ctx, i, SurfacePatchOptions{})
		if err != nil || c.Model == nil || c.UV[0] == nil || c.UV[1] == nil {
			t.Fatal("could not construct an exact shared contact spline", i, c, err)
		}
		for _, x := range []*big.Rat{new(big.Rat), big.NewRat(2, 7), big.NewRat(1, 2), big.NewRat(1, 1)} {
			p, err := c.Model.patch.EvaluateRational(ctx, x, big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			wantZ := new(big.Rat).Mul(p.Point[0], p.Point[0])
			if p.Point[0].Cmp(p.Point[1]) != 0 || p.Point[2].Cmp(wantZ) != 0 {
				t.Fatal("shared model lost its independent parabolic law", p.Point)
			}
			for owner, original := range [2]*SurfacePatch{a, b} {
				uv, err := c.UV[owner].patch.EvaluateRational(ctx, x, big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
				if err != nil || uv.Point[2].Sign() != 0 {
					t.Fatal("pcurve failure", uv, err)
				}
				model, err := original.EvaluateRational(ctx, uv.Point[0], uv.Point[1], SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for j := range p.Point {
					if model.Point[j].Cmp(p.Point[j]) != 0 {
						t.Fatal("one of the original chart clocks moved off the model curve", owner, x, model.Point, p.Point)
					}
				}
			}
		}
	}
	c, err := g.BuildArcSpline(ctx, 0, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(err, ErrConvergence) || c.Model != nil || c.UV[0] != nil || c.UV[1] != nil {
		t.Fatal("unproved pcurve construction must be atomic", c, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if c, err := g.BuildArcSpline(canceled, 0, SurfacePatchOptions{}); !errors.Is(err, context.Canceled) || c.Model != nil {
		t.Fatal("canceled spline proof published results", c, err)
	}
}

func TestProjectiveContactSplineRejectsTheChordOfAnOriginalCurve(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	var pts []Vec3
	for i, h := range []float64{.25, .75, .25} {
		for j := range 2 {
			v := float64(j)
			pts = append(pts, Vec3{float64(i) / 2, v, v - h})
		}
	}
	s, err := NewBSplineSurface(2, 1, 3, 2, pts, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil || !g.Complete || len(g.Arcs) != 1 {
		t.Fatal("original parabolic pcurve must stay fully certified as an implicit arc", g, err)
	}
	for _, v := range g.Vertices {
		assertProjectiveBoundaryRoot(t, v.Common[1], big.NewRat(1, 4))
	}
	got, err := g.BuildArcSpline(ctx, 0, SurfacePatchOptions{})
	if !errors.Is(err, ErrConvergence) || got.Model != nil || got.UV[0] != nil {
		t.Fatal("matching rational terminals do not authenticate the whole chord", got, err)
	}
}
