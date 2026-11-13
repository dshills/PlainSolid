package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFiniteSphereBooleanCapSections(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	c, _ := NewOffAxisSphereFiniteCylinderCommon(5, 3, Vec2{X: 4}, 0, 4)
	u, _ := NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 4}, 0, 4)
	d, _ := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 4}, 0, 4)
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
	for _, height := range []float64{0, 4} {
		edge := int64(5)
		mask := BoreLowerCap
		if height == 4 {
			edge = 3
			mask = BoreUpperCap
		}
		cr, e := c.ClipSectionLine(ctx, height, Vec2{}, Vec2{X: 1}, -8, 8, o)
		if e != nil || len(cr.Intervals) != 1 || len(cr.Points) != 0 {
			t.Fatal(cr, e)
		}
		eq(cr.Intervals[0].Start, 1)
		eq(cr.Intervals[0].End, edge)
		if cr.Intervals[0].Boundary != mask {
			t.Fatal(cr)
		}
		ur, e := u.ClipSectionLine(ctx, height, Vec2{}, Vec2{X: 1}, -8, 8, o)
		if e != nil || len(ur.Intervals) != 2 {
			t.Fatal(ur, e)
		}
		eq(ur.Intervals[0].Start, -edge)
		eq(ur.Intervals[0].End, edge)
		eq(ur.Intervals[1].Start, edge)
		eq(ur.Intervals[1].End, 7)
		if ur.Intervals[0].Boundary != 0 || ur.Intervals[1].Boundary != mask {
			t.Fatal(ur)
		}
		dr, e := d.ClipSectionLine(ctx, height, Vec2{}, Vec2{X: 1}, -8, 8, o)
		if e != nil || len(dr.Intervals) != 2 {
			t.Fatal(dr, e)
		}
		eq(dr.Intervals[0].Start, -edge)
		eq(dr.Intervals[0].End, 1)
		eq(dr.Intervals[1].Start, 1)
		eq(dr.Intervals[1].End, edge)
		if dr.Intervals[1].Boundary != mask {
			t.Fatal(dr)
		}
		// The identical geometric cap-circle cut has one exact root across ops.
		cmp, e := cr.Intervals[0].End.Compare(ctx, ur.Intervals[0].End, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
	}
	// Away from caps union hides both internal crossing events; common keeps
	// only the overlap, with one cylinder and one irrational sphere endpoint.
	cr, e := c.ClipSectionLine(ctx, 2, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(cr.Intervals) != 1 {
		t.Fatal(cr, e)
	}
	eq(cr.Intervals[0].Start, 1)
	ur, e := u.ClipSectionLine(ctx, 2, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(ur.Intervals) != 1 {
		t.Fatal(ur, e)
	}
	eq(ur.Intervals[0].End, 7)
	if ur.Intervals[0].Boundary != 0 {
		t.Fatal(ur)
	}
}

func TestFiniteSphereBooleanSectionTangencyAndLimits(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	c, _ := NewOffAxisSphereFiniteCylinderCommon(5, 1, Vec2{X: 6}, -1, 1)
	u, _ := NewOffAxisSphereFiniteCylinderUnion(5, 1, Vec2{X: 6}, -1, 1)
	cr, e := c.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(cr.Points) != 1 || len(cr.Intervals) != 0 || cr.Points[0].Boundary != BoreSphere|BoreCylinder {
		t.Fatal(cr, e)
	}
	ur, e := u.ClipSectionLine(ctx, 0, Vec2{}, Vec2{X: 1}, -8, 8, o)
	if e != nil || len(ur.Intervals) != 2 || len(ur.Points) != 0 {
		t.Fatal(ur, e)
	}
	if ur.Intervals[0].EndBoundary != BoreSphere|BoreCylinder || ur.Intervals[1].StartBoundary != BoreSphere|BoreCylinder {
		t.Fatal(ur)
	}
	for _, clip := range []func(context.Context, float64, Vec2, Vec2, float64, float64, ToroidLineOptions) (SphereCylinderSectionClip, error){c.ClipSectionLine, u.ClipSectionLine} {
		r, e := clip(ctx, 0, Vec2{}, Vec2{X: 1}, -8, 8, ToroidLineOptions{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}})
		if e == nil || len(r.Intervals)+len(r.Points) != 0 {
			t.Fatal(r, e)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		r, e = clip(canceled, 0, Vec2{}, Vec2{X: 1}, -8, 8, o)
		if e != context.Canceled || len(r.Intervals)+len(r.Points) != 0 {
			t.Fatal(r, e)
		}
	}
}
