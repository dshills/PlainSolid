//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofBooleanProvesUnequalFootprintContainment(t *testing.T) {
	ctx := context.Background()
	a := projectiveFootprintRoof(t, .5, 1, 0, 0)
	b := projectiveFootprintRoof(t, 0, 1.5, 1, 0)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for i, pair := range [][2]Face{{a, b}, {b, a}} {
			r, err := BuildProjectiveSplineRoofBoolean(ctx, pair[0], pair[1], axes, 0, parametric.Union, ProjectiveRoofSolidOptions{})
			winner := 1 - i
			if err != nil || !r.Complete || r.Empty || r.Roofs[winner] != 0 || r.Roofs[i] != -1 ||
				r.Solid.faces[0].Surface != b.Surface || len(r.SectionEdges) != 0 {
				t.Fatal("the original containing stock must be chosen without clipping its nonbinary chart", axes, i, err)
			}
			for _, wall := range r.Walls {
				if wall.Owner != winner || wall.Source.OriginalEdge < 0 {
					t.Fatal("a contained Fuse must keep the containing stock's original wall owner", wall)
				}
			}
			m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(m.Volume-57.0/16) > 1e-10 {
				t.Fatal("independent integral of the complete containing stock changed", m, err)
			}
		}
		r, err := BuildProjectiveSplineRoofBoolean(ctx, a, b, axes, 0, parametric.Difference, ProjectiveRoofSolidOptions{})
		if err != nil || !r.Complete || !r.Empty || len(r.Solid.faces) != 0 {
			t.Fatal("a completely contained stock is truly empty after Cut", axes, err)
		}
	}
	if r, err := BuildProjectiveSplineRoofBoolean(ctx, b, a, [2]int{0, 1}, 0, parametric.Difference, ProjectiveRoofSolidOptions{}); !errors.Is(err, geom.ErrConvergence) || r.Complete {
		t.Fatal("a smaller original tool needs a separate proof for exterior stock material", r, err)
	}
}

func TestBuildProjectiveSplineRoofBooleanClipsCoveringToolAndKeepsFloorOwner(t *testing.T) {
	ctx := context.Background()
	a := projectiveFootprintRoof(t, .5, 1, -.75, 1)
	b := projectiveFootprintRoof(t, 0, 2, 0, 0)
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		r, err := BuildProjectiveSplineRoofBoolean(ctx, a, b, axes, 0, parametric.Difference, ProjectiveRoofSolidOptions{})
		if err != nil || !r.Complete || r.Empty || len(r.SectionEdges) != 1 || len(r.FloorSources) != 3 ||
			r.Solid.faces[0].Surface != a.Surface || r.Solid.faces[1].Surface != b.Surface {
			t.Fatal("the original tool must be clipped to the entire original stock footprint", axes, err)
		}
		generated := 0
		for _, floor := range r.FloorSources {
			if floor.OriginalUse == -1 && floor.OriginalEdge == -1 {
				generated++
			} else if floor.OriginalEdge != b.Outer[floor.OriginalUse].Edge {
				t.Fatal("the clipped tool borrowed an edge ID", floor)
			}
		}
		m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || generated != 1 || math.Abs(m.Volume-1.0/32) > 1e-10 || math.Abs(m.Centroid.X-11.0/12) > 1e-10 {
			t.Fatal("the exact clipped floor and independently integrated layer must agree", axes, generated, m, err)
		}
	}
	for _, left := range []float64{1, 2} {
		distant := projectiveFootprintRoof(t, left, left+1, 0, 0)
		r, err := BuildProjectiveSplineRoofBoolean(ctx, a, distant, [2]int{0, 1}, 0, parametric.Difference, ProjectiveRoofSolidOptions{})
		if err != nil || !r.Complete || r.Empty || r.Roofs != [2]int{0, -1} || r.Solid.faces[0].Surface != a.Surface {
			t.Fatal("disjoint or edge-only tool leaves the exact original stock", left, err)
		}
	}
}
