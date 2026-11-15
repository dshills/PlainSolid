package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSplineShellsConvexAffinePolygonCrossing(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCrossingFixture(t)
	o := SplineShellCombineOptions{AffinePolygon: &ConvexAffinePolygonBooleanOptions{}}
	for _, q := range []struct {
		op     parametric.BooleanOp
		volume int64
	}{{parametric.Intersection, 9675}, {parametric.Union, 56373}, {parametric.Difference, 23349}} {
		out, h, e := CombineSplineShells(ctx, a, b, q.op, o)
		if e != nil || h.AffinePolygon == nil || h.Method != "exact-convex-affine-polygon-crossing" ||
			h.AffinePolygon.ExactVolume().Cmp(big.NewRat(q.volume, 8192)) != 0 || len(h.SectionEdges) != 5 ||
			len(out.Topology().Shells) != 1 {
			t.Fatal("exact native material, section and volume", q.op, h, e)
		}
		parents := [2]Solid{a, b}
		faces := [2][][]int{h.FacesA, h.FacesB}
		edges := [2][][]int{h.EdgesA, h.EdgesB}
		for i, src := range h.AffinePolygon.FaceSources() {
			if !slices.Contains(faces[src.Owner][src.Face], i) || slices.Contains(h.ReversedFaces, i) != src.Reversed ||
				out.faces[i].Surface != parents[src.Owner].faces[src.Face].Surface {
				t.Fatal("complete face ancestry", q.op, i, src)
			}
			fragment := h.AffinePolygon.SourceFaceFragments(src.Owner, src.Face)
			for j, use := range fragment.cycles[src.Fragment].Uses {
				edge := out.faces[i].Outer[j].Edge
				if use.OriginalEdge >= 0 && !slices.Contains(edges[src.Owner][use.OriginalEdge], edge) {
					t.Fatal("complete retained edge ancestry", q.op, i, use)
				}
			}
		}
	}
	o.AffinePaired = &ConvexAffineSplineShellOptions{}
	if q, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o); !errors.Is(e, geom.ErrInvalid) || len(q.faces) != 0 || h.AffinePolygon != nil {
		t.Fatal("multiple explicit material dispatch paths", e)
	}
}
