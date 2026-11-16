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
	whole, err := g.BuildContourSplines(ctx, 0, SurfacePatchOptions{})
	if !errors.Is(err, ErrConvergence) || whole.Complete || len(whole.Uses) != 0 {
		t.Fatal("an unsupported arc must not publish an approximate contour", whole, err)
	}
}

func TestProjectiveContactContourSplinesPreserveOriginalKnotAndClock(t *testing.T) {
	ctx := context.Background()
	var owners [2]*SurfacePatch
	for i := range owners {
		stations, vknots := []float64{0, .375, 1}, []float64{0, 0, .375, 1, 1}
		if i == 1 {
			stations, vknots = []float64{0, 1}, []float64{0, 0, 1, 1}
		}
		var poles []Vec3
		var weights []float64
		for x := range 2 {
			for _, y := range stations {
				z, w := 0., 1.
				if i == 1 {
					z, w = float64(x)-.5, 1+y
				}
				poles = append(poles, Vec3{float64(x), y, z})
				weights = append(weights, w)
			}
		}
		raw, err := NewBSplineSurface(1, 1, 2, len(stations), poles, []float64{0, 0, 1, 1}, vknots, weights)
		if err != nil {
			t.Fatal(err)
		}
		owners[i], err = NewSurfacePatch(raw, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := TraceProjectiveSurfaceContacts(ctx, owners[0], owners[1], [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil || !g.Complete || len(g.Contours) != 1 || len(g.Contours[0].Arcs) != 2 {
		t.Fatal("original knot should split the same complete contour exactly", err)
	}
	g.Contours[0].Arcs[0].Arc = -1
	g.Contours[0].Closed = true
	g.Arcs[0].Vertices = [2]int{-1, -1}
	r, err := g.BuildContourSplines(ctx, 0, SurfacePatchOptions{})
	if err != nil || !r.Complete || r.Closed || len(r.Uses) != 2 || r.Uses[0].Vertices[1] != r.Uses[1].Vertices[0] {
		t.Fatal("use the sealed directed topology across the original knot", r, err)
	}
	for i, use := range r.Uses {
		at := int64(1 - i)
		if use.Reversed {
			at = 1 - at
		}
		for owner, expect := range [2]*big.Rat{big.NewRat(3, 8), big.NewRat(3, 13)} {
			uv, err := use.Spline.UV[owner].patch.EvaluateRational(ctx, big.NewRat(at, 1), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
			if err != nil || uv.Point[0].Cmp(big.NewRat(1, 2)) != 0 || uv.Point[1].Cmp(expect) != 0 {
				t.Fatal("two independently authored clocks must preserve the exact original station", i, owner, uv, err)
			}
		}
	}
	short, err := g.BuildContourSplines(ctx, 0, SurfacePatchOptions{MaxOperations: r.ExactOperations - 1})
	if !errors.Is(err, ErrConvergence) || short.Complete || len(short.Uses) != 0 {
		t.Fatal("one-short overall work must not publish the already-proved prefix", short, err)
	}
}
