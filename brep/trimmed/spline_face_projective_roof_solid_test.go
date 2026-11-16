package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofSolidClosesExactCommonAndFuse(t *testing.T) {
	ctx := context.Background()
	as, bs := projectiveImprintShellFixture(t, true, false), projectiveImprintShellFixture(t, false, true)
	a, b := as.faces[5], bs.faces[5]
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for _, test := range []struct {
			op        parametric.BooleanOp
			vol, x, y float64
		}{
			{parametric.Intersection, 9.0 / 8, 31.0 / 54, 29.0 / 54},
			{parametric.Union, 11.0 / 8, 37.0 / 66, 35.0 / 66},
		} {
			q, err := BuildProjectiveSplineRoofSolid(ctx, a, b, axes, 0, test.op, ProjectiveRoofSolidOptions{})
			if err != nil || !q.Complete {
				t.Fatal("material envelope must close against its exact positive base", axes, test.op, err)
			}
			s := q.Solid
			if len(s.faces) != 9 || len(s.edges) != 19 || len(s.vertices) != 12 ||
				len(q.Walls) != 6 || q.BaseFace != 8 || q.Roofs != [2]int{0, 1} {
				t.Fatal("regularized roof must use two source faces, six original walls and one base", axes, test.op, q)
			}
			hasOwners := [2]int{}
			for _, wall := range q.Walls {
				hasOwners[wall.Owner]++
				if wall.Source.Section || wall.Source.OriginalEdge < 0 || wall.Source.OriginalRange[0].Cmp(wall.Source.OriginalRange[1]) >= 0 {
					t.Fatal("wall lost original directed roof edge ownership", wall)
				}
			}
			if hasOwners != [2]int{3, 3} {
				t.Fatal("both original surface regions must own three boundary walls", hasOwners)
			}
			edge := s.edges[q.Section]
			u, v := s.vertices[edge.Start], s.vertices[edge.End]
			if u.X != .5 || v.X != .5 || u.Y+v.Y != 1 || u.Z+v.Z != 2.5 {
				t.Fatal("sewn exact intersection seam lost original model identity", u, v)
			}
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(m.Volume-test.vol) > 1e-10 || math.Abs(m.Centroid.X-test.x) > 1e-10 ||
				math.Abs(m.Centroid.Y-test.y) > 1e-10 {
				t.Fatal("closed original envelope disagrees with the independent material integrals", axes, test, m, err)
			}
		}
	}
	r, err := BuildProjectiveSplineRoofSolid(ctx, a, b, [2]int{0, 1}, .5, parametric.Intersection, ProjectiveRoofSolidOptions{})
	if !errors.Is(err, geom.ErrConvergence) || r.Complete || len(r.Solid.faces) != 0 {
		t.Fatal("a roof touching its base cannot publish a closed original solid in this subset", r, err)
	}
}
