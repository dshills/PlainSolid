package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveQuadraticSaddleRetainsIndependentOriginalCharts(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		a, b, axes := projectiveQuadraticSaddleCharts(t, rotated)
		r, err := CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, b, axes, 2, ProjectiveQuadraticSaddleOptions{})
		if err != nil || !r.Proven || r.LineAxis != [2]int{1, 0} || r.Center.OnOverlapBoundary {
			t.Fatalf("independent original charts did not prove all four saddle rays: %+v; %v", r, err)
		}
		verify := func(point ProjectiveContactVertex, u, v, bu, bv *big.Rat) {
			t.Helper()
			assertProjectiveBoundaryRoot(t, point.Common[0], u)
			assertProjectiveBoundaryRoot(t, point.Common[1], v)
			assertProjectiveBoundaryRoot(t, point.OriginalUV[0][0], u)
			assertProjectiveBoundaryRoot(t, point.OriginalUV[0][1], new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).Quo(v, big.NewRat(2, 1))))
			assertProjectiveBoundaryRoot(t, point.OriginalUV[1][0], bu)
			assertProjectiveBoundaryRoot(t, point.OriginalUV[1][1], bv)
		}
		half, vMid := big.NewRat(1, 2), big.NewRat(3, 4)
		bu, bv := big.NewRat(-2, 5), big.NewRat(32, 5)
		verify(r.Center, half, vMid, bu, bv)
		verify(r.Rays[0][0].End, half, new(big.Rat), big.NewRat(-2, 1), bv)
		verify(r.Rays[0][1].End, half, big.NewRat(9, 10), new(big.Rat), bv)
		verify(r.Rays[1][0].End, big.NewRat(1, 4), vMid, bu, big.NewRat(7, 1))
		verify(r.Rays[1][1].End, big.NewRat(1, 1), vMid, bu, big.NewRat(64, 13))
		if r.Rays[0][1].End.OnOverlapBoundary || r.Rays[0][1].Side != [2]int{0, 1} ||
			!r.Rays[1][1].End.OnOverlapBoundary || r.Rays[1][1].Side != [2]int{1, 0} {
			t.Fatal("interior source knot and exterior overlap side must remain distinct", r.Rays)
		}
		other, err := CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, b, axes, 3, ProjectiveQuadraticSaddleOptions{})
		if !errors.Is(err, ErrConvergence) || other.Proven || other.Center.Common[0] != nil {
			t.Fatal("the neighboring tile does not contain the critical point", other, err)
		}
	}
}

func projectiveQuadraticSaddleCharts(t *testing.T, rotated bool) (*SurfacePatch, *SurfacePatch, [2]int) {
	t.Helper()
	lift, axes := 2, [2]int{0, 1}
	if rotated {
		lift, axes = 1, [2]int{2, 0}
	}
	a := arcTestPatch(t, func(u, v float64) Vec3 {
		z := (u - .5) * (v - .875)
		if rotated {
			return Vec3{v, z, u}
		}
		return Vec3{u, v, z}
	})
	b := projectiveOverlapTestChart(t, rotated, false)
	for i := range b.net {
		b.net[i][lift] = new(big.Rat)
	}
	if err := b.computeBounds(&polynomialBudget{maxOperations: b.options.MaxOperations, maxBits: b.options.MaxCoefficientBits}); err != nil {
		t.Fatal(err)
	}
	return a, b, axes
}

func TestProjectiveQuadraticSaddleClipsIrrationalEndpointsAndCorners(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	axis := [2]int{0, 1}
	makeHeight := func(weight, level float64) *SurfacePatch {
		t.Helper()
		q := []float64{.25, -.25, .25}
		var poles []Vec3
		for i := range q {
			for j := range q {
				poles = append(poles, Vec3{float64(i) / 2, float64(j) / 2, weight*q[j] - q[i] + level})
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		s, err := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewSurfacePatch(s, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	irrational := makeHeight(2, 0)
	r, err := CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, irrational, axis, 0, ProjectiveQuadraticSaddleOptions{})
	if err != nil || !r.Proven || r.LineAxis != [2]int{0, 0} {
		t.Fatalf("irrational saddle was not proved: %+v; %v", r, err)
	}
	assertProjectiveBoundaryRoot(t, r.Center.Common[0], big.NewRat(1, 2))
	assertProjectiveBoundaryRoot(t, r.Center.Common[1], big.NewRat(1, 2))
	for line, across := range r.Rays {
		for direction, ray := range across {
			side := 2*direction - 1
			aboveCenter := (2*line - 1) * side
			if ray.Side != [2]int{side, 0} || !ray.End.OnOverlapBoundary {
				t.Fatal("lost side identity", ray.Side)
			}
			assertProjectiveBoundaryRoot(t, ray.End.Common[0], big.NewRat(int64(direction), 1))
			for _, uv := range ray.End.OriginalUV {
				root := uv[1]
				sign, err := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
				order, orderErr := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
				if err != nil || orderErr != nil || sign.Sign != 0 || order.Sign != aboveCenter ||
					root.Interval().Lower.Cmp(root.Interval().Upper) == 0 {
					t.Fatal("wrong original irrational branch endpoint", line, direction, sign, order, err, orderErr)
				}
			}
		}
	}
	graph, err := TraceProjectiveSurfaceContacts(ctx, a, irrational, axis, ProjectiveSurfaceContactGraphOptions{
		Cover: ProjectiveSurfaceContactCoverOptions{IsolateQuadraticSaddles: true},
	})
	if err != nil || !graph.Complete || len(graph.Arcs) != 4 || len(graph.SaddleVertices) != 1 {
		t.Fatal("irrational rays did not participate in the complete contact graph", graph, err)
	}
	for i := range graph.Arcs {
		arc := graph.Arcs[i]
		station := big.NewRat(int64(1+2*(arc.Local%2)), 4)
		// The evaluation continues to use its sealed source even if a
		// caller overwrites the copied result's public topology and centre.
		graph.Cover.Saddles[0].Center = ProjectiveContactVertex{}
		graph.Arcs[i].Saddle = false
		q, _, err := graph.EvaluateArcRational(ctx, i, station, SurfacePatchEvaluationOptions{})
		ax, axErr := graph.ArcFreeAxis(i)
		if err != nil || axErr != nil || ax != 0 {
			t.Fatal("irrational saddle arc lost its sealed original map", i, ax, err, axErr)
		}
		for _, uvs := range q.OriginalUV {
			assertProjectiveBoundaryRoot(t, uvs[0], station)
			result, err := uvs[1].SignPolynomial(ctx, []*big.Rat{big.NewRat(7, 32), big.NewRat(-1, 1), big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
			if err != nil || result.Sign != 0 {
				t.Fatal("original saddle midpoint not on its defining exact branch", result, err)
			}
		}
	}
	r, err = CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, makeHeight(1, 0), axis, 0, ProjectiveQuadraticSaddleOptions{})
	if err != nil || !r.Proven {
		t.Fatal("four exact corners must each belong to one ray", r, err)
	}
	for line, across := range r.Rays {
		for direction, ray := range across {
			x := 2*direction - 1
			y := (2*line - 1) * x
			if ray.Side != [2]int{x, y} {
				t.Fatal("duplicate or incorrectly classified corner", line, direction, ray.Side)
			}
		}
	}
	// On each U side there is one corner and a second, distinct zero.
	// A retained closed isolating interval may not include both zeros.
	var mixedPoles []Vec3
	for i, u := range []float64{1.0 / 16, -1.0 / 16, 1.0 / 16} {
		for j, v := range []float64{1.0 / 16, -3.0 / 16, 9.0 / 16} {
			mixedPoles = append(mixedPoles, Vec3{float64(i) / 2, float64(j) / 2, v - u})
		}
	}
	knots := []float64{0, 0, 0, 1, 1, 1}
	s, err := NewBSplineSurface(2, 2, 3, 3, mixedPoles, knots, knots, nil)
	if err != nil {
		t.Fatal(err)
	}
	mixed, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, mixed, axis, 0, ProjectiveQuadraticSaddleOptions{})
	if err != nil || !r.Proven {
		t.Fatal("each original U side must retain its corner and interior root", r, err)
	}
	for line, across := range r.Rays {
		for direction, ray := range across {
			want, side := big.NewRat(1, 2), 0
			if line != direction {
				want.SetInt64(0)
				side = -1
			}
			if ray.Side != [2]int{2*direction - 1, side} {
				t.Fatal("corner identity belongs only to the original corner event", ray.Side)
			}
			assertProjectiveBoundaryRoot(t, ray.End.Common[1], want)
		}
	}
	for _, trial := range []struct {
		height *SurfacePatch
		opts   ProjectiveQuadraticSaddleOptions
	}{
		{makeHeight(2, .125), ProjectiveQuadraticSaddleOptions{}},
		{irrational, ProjectiveQuadraticSaddleOptions{MaxOperations: 1}},
		{irrational, ProjectiveQuadraticSaddleOptions{MaxRootSubdivisions: 1}},
	} {
		r, err := CertifyProjectiveSurfaceQuadraticSaddle(ctx, a, trial.height, axis, 0, trial.opts)
		if !errors.Is(err, ErrConvergence) || r.Proven || r.Center.Common[0] != nil || r.Rays[0][0].End.Common[0] != nil {
			t.Fatal("an unproved critical level or exhausted budget must publish no contact topology", r, err)
		}
	}
}
