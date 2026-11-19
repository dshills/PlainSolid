//go:build research

package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofBooleanFindsExactMaterialProof(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	a, b := projectiveOrderedRoofFixture(t, 1, 1, 1), projectiveOrderedRoofFixture(t, 1.5, 2, 1)
	r, err := BuildProjectiveSplineRoofBoolean(ctx, a, b, axes, 0, parametric.Difference, ProjectiveRoofSolidOptions{})
	if err != nil || !r.Complete || !r.Empty || len(r.Solid.faces) != 0 || len(r.SectionEdges) != 0 {
		t.Fatal("the dispatcher must accept an exact empty contained volume", r, err)
	}
	r, err = BuildProjectiveSplineRoofBoolean(ctx, a, b, axes, 0, parametric.Union, ProjectiveRoofSolidOptions{})
	if err != nil || !r.Complete || r.Empty || len(r.Solid.faces) != 6 || len(r.SectionEdges) != 0 || r.Roofs != [2]int{-1, 0} {
		t.Fatal("strict original ordering must keep its authored upper owner", r.Roofs, err)
	}

	a, b = projectiveContourImprintFace(t, true, true, false), projectiveContourImprintFace(t, false, false, false)
	opts := ProjectiveRoofSolidOptions{MatchExteriorPartitions: true}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
		r, err = BuildProjectiveSplineRoofBoolean(ctx, a, b, axes, 0, op, opts)
		if err != nil || !r.Complete || r.Empty || len(r.SectionEdges) != 2 || r.Roofs != [2]int{0, 1} {
			t.Fatal("one complete original contour must dispatch without a supplied seam or material sign", op, r.Roofs, err)
		}
	}
	partial := projectiveOrderedRoofFixture(t, 1.5, 1, .5)
	r, err = BuildProjectiveSplineRoofBoolean(ctx, a, partial, axes, 0, parametric.Union, opts)
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Solid.faces) != 0 {
		t.Fatal("original unmatched footprint cannot publish a result", err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	r, err = BuildProjectiveSplineRoofBoolean(canceled, a, b, axes, 0, parametric.Union, opts)
	if !errors.Is(err, context.Canceled) || r.Complete {
		t.Fatal("a canceled candidate cannot publish any material", err)
	}
}
