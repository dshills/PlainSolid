package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFiniteDifferenceSectionClip(t *testing.T) {
	s, e := NewOffAxisSphereFiniteCylinderDifference(5, 1, Vec2{X: 2}, -1, 1)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	eq := func(r *ExactAlgebraicRoot, w int64) {
		t.Helper()
		q, _, e := NewExactRationalRoot(ctx, big.NewRat(w, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, e := r.Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !v.Proven || v.Order != 0 {
			t.Fatal(w, v, e)
		}
	}
	r, e := s.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 2 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	for i, ab := range [][2]int64{{-5, 1}, {3, 5}} {
		eq(r.Intervals[i].Start, ab[0])
		eq(r.Intervals[i].End, ab[1])
		if r.Intervals[i].Boundary != 0 {
			t.Fatal("interior boundary", r)
		}
	}
	if r.Intervals[0].StartBoundary != BoreSphere || r.Intervals[0].EndBoundary != BoreCylinder {
		t.Fatal(r)
	}
	// Reverse original parameter direction, preserving the two distinct pieces.
	r, e = s.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: -1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 2 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[0].Start, -5)
	eq(r.Intervals[0].End, -3)
	eq(r.Intervals[1].Start, -1)
	eq(r.Intervals[1].End, 5)
	// A horizontal pocket cap remains in the closed 3D material. Its support
	// interval is retained between two interior intervals, with shared endpoints.
	r, e = s.ClipSectionLine(ctx, 1, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 3 || len(r.Points) != 0 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[1].Start, 1)
	eq(r.Intervals[1].End, 3)
	if r.Intervals[1].Boundary != BoreUpperCap {
		t.Fatal(r)
	}
	for i := 0; i < 2; i++ {
		c, e := r.Intervals[i].End.Compare(ctx, r.Intervals[i+1].Start, ExactCurvePlaneEventOptions{})
		if e != nil || !c.Proven || c.Order != 0 {
			t.Fatal("section gap", c, e)
		}
	}
	// An isolated tangent tool support inside a filled section is retained as
	// a shared endpoint rather than silently erased by interval merging.
	r, e = s.ClipSectionLine(ctx, 0, Vec2{Y: 1}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 2 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[0].End, 2)
	eq(r.Intervals[1].Start, 2)
	if r.Intervals[0].EndBoundary != BoreCylinder {
		t.Fatal(r)
	}
	// At the north pole the section is an isolated point.
	r, e = s.ClipSectionLine(ctx, 5, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(r.Points) != 1 || len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
	eq(r.Points[0].Parameter, 0)
	if r.Points[0].Boundary != BoreSphere {
		t.Fatal(r)
	}
	r, e = s.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, -2, -1, o)
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[0].Start, -2)
	eq(r.Intervals[0].End, -1)
	if r.Intervals[0].StartBoundary != 0 || r.Intervals[0].EndBoundary != 0 {
		t.Fatal(r)
	}
	r, e = s.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, 2, 2.5, o)
	if e != nil || len(r.Intervals)+len(r.Points) != 0 {
		t.Fatal(r, e)
	}
}

func TestFiniteDifferenceSectionClipLimits(t *testing.T) {
	s, _ := NewOffAxisSphereFiniteCylinderDifference(5, 1, Vec2{X: 2}, -1, 1)
	for _, o := range []ToroidLineOptions{{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}}, {ParameterTolerance: 1e-30, MaxSubdivisions: 1}} {
		r, e := s.ClipSectionLine(context.Background(), 2, Vec2{}, Vec2{X: 1}, -8, 8, o)
		if e == nil || len(r.Points)+len(r.Intervals) != 0 {
			t.Fatal(r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := s.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, -8, 8, ToroidLineOptions{ParameterTolerance: 1e-8})
	if e != context.Canceled || len(r.Points)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
	if _, e := s.ClipSectionLine(context.Background(), 0, Vec2{}, Vec2{}, -8, 8, ToroidLineOptions{ParameterTolerance: 1e-8}); e == nil {
		t.Fatal("zero direction")
	}
}
