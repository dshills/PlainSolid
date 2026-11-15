package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSplineShellsClosedEllipseTangency(t *testing.T) {
	ctx := context.Background()
	inner := quarticRoofSolid(t, 1.0/16, 1.0/16)
	exterior, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	enclosure, _ := makeOpenSplineBox(t, [3]float64{-.125, -.125, -2}, [3]float64{1.125, 1.125, 0}, 0)
	m := geom.Affine{{0, 0, 1, 4}, {1, 0, 0, -2}, {0, 1, 0, 3}}
	placed := func(s Solid) Solid {
		t.Helper()
		q, err := s.Transformed(m)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	o := SplineShellCombineOptions{ClosedEllipseTangency: &geom.GraphPointContactOptions{}}
	for _, tc := range []struct {
		sources [2]Solid
		faces   [2]int
		op      parametric.BooleanOp
		owner   int
		outer   int
	}{
		{[2]Solid{exterior, inner}, [2]int{4, 1}, parametric.Intersection, -1, -1},
		{[2]Solid{inner, exterior}, [2]int{1, 4}, parametric.Difference, 0, -1},
		{[2]Solid{enclosure, inner}, [2]int{5, 1}, parametric.Intersection, 1, 0},
		{[2]Solid{inner, enclosure}, [2]int{1, 5}, parametric.Union, 1, 1},
		{[2]Solid{inner, enclosure}, [2]int{1, 5}, parametric.Difference, -1, 1},
	} {
		sources := [2]Solid{placed(tc.sources[0]), placed(tc.sources[1])}
		s, h, e := CombineSplineShells(ctx, sources[0], sources[1], tc.op, o)
		p := h.ClosedEllipse
		if e != nil || p == nil || !p.Contact.Proven || p.Enclosing != tc.outer || p.Faces != tc.faces ||
			p.Projection != [2]int{1, 2} || p.SeparatedPairs != 35 {
			t.Fatal("automatic original pair, projection and material relation", tc.op, p, e)
		}
		if tc.owner < 0 {
			if len(s.faces) != 0 {
				t.Fatal("expected empty regularized material", tc.op)
			}
			continue
		}
		faces, edges := [2][][]int{h.FacesA, h.FacesB}, [2][][]int{h.EdgesA, h.EdgesB}
		if len(s.faces) != 6 || s.faces[tc.faces[tc.owner]].Surface != sources[tc.owner].faces[tc.faces[tc.owner]].Surface ||
			len(faces[tc.owner][0]) != 1 || len(faces[1-tc.owner][0]) != 0 ||
			len(edges[tc.owner][0]) != 1 || len(edges[1-tc.owner][0]) != 0 {
			t.Fatal("native original boundary and complete owner rows", tc.op, h)
		}
	}
	o.ClosedEllipseTangency = &geom.GraphPointContactOptions{MaxOperations: -1}
	if s, h, e := CombineSplineShells(ctx, enclosure, inner, parametric.Intersection, o); !errors.Is(e, geom.ErrInvalid) || h.ClosedEllipse != nil || len(s.faces) != 0 {
		t.Fatal("invalid discovery budget must fail atomically", e)
	}
}
