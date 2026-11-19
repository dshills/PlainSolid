package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"testing"
)

func TestCombineSplineShellsAffinePaired(t *testing.T) {
	ctx := context.Background()
	for fixture := 0; fixture < 2; fixture++ {
		a, b := pairedAffineShellFixture(t)
		if fixture == 1 {
			a, b = affineMaterialOnePairedFixture(t)
		}
		for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
			out, h, e := CombineSplineShells(ctx, a, b, op, SplineShellCombineOptions{AffinePaired: &ConvexAffineSplineShellOptions{}})
			if fixture == 0 && op == parametric.Difference {
				if e == nil || len(out.Faces()) != 0 || h.AffinePaired != nil || h.Method != "" {
					t.Fatal("singular fallback/publication")
				}
				continue
			}
			if e != nil {
				t.Fatal(e)
			}
			if h.Method != "exact-convex-affine-paired" || h.AffinePaired == nil || len(h.AffinePaired.FaceSources()) != len(out.faces) {
				t.Fatal("lost proof history")
			}
			faces, edges := map[int]bool{}, map[int]bool{}
			for _, table := range [][][]int{h.FacesA, h.FacesB} {
				for _, ids := range table {
					for _, id := range ids {
						if faces[id] {
							t.Fatal("duplicate face owner")
						}
						faces[id] = true
					}
				}
			}
			for _, table := range [][][]int{h.EdgesA, h.EdgesB} {
				for _, ids := range table {
					for _, id := range ids {
						edges[id] = true
					}
				}
			}
			for _, ids := range [][]int{h.SectionEdges, h.GeneratedEdges} {
				for _, id := range ids {
					edges[id] = true
				}
			}
			if len(faces) != len(out.faces) || len(edges) != len(out.edges) || len(h.SectionEdges) == 0 || len(h.GeneratedEdges) == 0 {
				t.Fatal("incomplete ancestry")
			}
			direct, e := CombineConvexAffineSplineShells(ctx, a, b, op, ConvexAffineSplineShellOptions{})
			if e != nil || direct.ExactVolume().Cmp(h.AffinePaired.ExactVolume()) != 0 {
				t.Fatal("normal/direct mismatch", e)
			}
		}
	}
	a, b := pairedAffineShellFixture(t)
	for _, o := range []SplineShellCombineOptions{
		{AffinePaired: &ConvexAffineSplineShellOptions{Authorization: AffineShellPairedImprintOptions{MaxFacePairs: 35}}},
		{AffinePaired: &ConvexAffineSplineShellOptions{Material: AffineShellMaterialOptions{MaxOperations: 1}}},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, Algebraic: &SplineShellAlgebraicNativeOptions{}},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, DisableAffineDiscovery: true},
		{AffinePaired: &ConvexAffineSplineShellOptions{}, DiscoverVerticalQuadraticWarp: true},
	} {
		out, h, e := CombineSplineShells(ctx, a, b, parametric.Intersection, o)
		if e == nil || len(out.faces) != 0 || h.Method != "" || h.AffinePaired != nil {
			t.Fatal("non-atomic dispatch")
		}
	}
}
