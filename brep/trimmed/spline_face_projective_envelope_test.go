//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSelectProjectiveSplineFaceEnvelopeRetainsOriginalMaterialSides(t *testing.T) {
	ctx := context.Background()
	s0, s1 := projectiveImprintShellFixture(t, true, false), projectiveImprintShellFixture(t, false, true)
	a, b := s0.faces[5], s1.faces[5]
	axes := [2]int{0, 1}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		r, err := SelectProjectiveSplineFaceEnvelope(ctx, a, b, axes, op, ProjectiveSplineEnvelopeOptions{})
		if err != nil || !r.Complete || r.Section.Model == nil {
			t.Fatal("original common rectangle must select one exact fragment per owner", r, err)
		}
		want := [2]int{-1, 1}
		if op == parametric.Union {
			want = [2]int{1, -1}
		}
		if r.RetainedSide != want {
			t.Fatal("wrong independent source material sides", r.RetainedSide, want)
		}
		traversals := [2]bool{}
		for i, part := range r.Faces {
			q := r.Witness[i]
			ref := big.NewRat(int64(want[i]), 2)
			if !q.OnOverlapBoundary || q.Difference.Cmp(ref) != 0 || q.Sign != want[i] {
				t.Fatal("exact original corner must reproduce the independent height difference", i, q, ref)
			}
			left, right := 1., 0.
			for _, v := range part.Vertices {
				left, right = min(left, v.X), max(right, v.X)
			}
			l, h := 0., .5
			if want[i] > 0 {
				l, h = .5, 1
			}
			if left != l || right != h {
				t.Fatal("retained original part is on the wrong physical side", i, left, right)
			}
			last := len(part.Face.Outer) - 1
			u := part.Face.Outer[last]
			if !r.Sources[i][last].Section || part.Edges[u.Edge].Curve != r.Section.Model {
				t.Fatal("the two original material choices must preserve one model seam", i)
			}
			traversals[i] = u.Reversed != part.Face.Reversed
		}
		if traversals[0] == traversals[1] {
			t.Fatal("the two retained source faces must oppose the common seam")
		}
	}
	r, err := SelectProjectiveSplineFaceEnvelope(ctx, a, b, axes, parametric.Intersection, ProjectiveSplineEnvelopeOptions{MaxClassifyOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || r.Section.Model != nil {
		t.Fatal("short sign classification cannot publish either source face", r, err)
	}
	partial := projectiveTrimTestRectangle(b.Surface, geom.ParameterRectangle{UMin: .25, UMax: .75, VMin: 0, VMax: 1}, 30)
	r, err = SelectProjectiveSplineFaceEnvelope(ctx, a, partial, axes, parametric.Union, ProjectiveSplineEnvelopeOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("a partial projected domain cannot stand for the full material envelope", r, err)
	}
}
