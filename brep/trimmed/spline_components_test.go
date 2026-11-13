package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCombineSplineShellsRetainsSeparatedComponents(t *testing.T) {
	ctx := context.Background()
	toroid, _, e := NewWrappedToroid(ctx, independentWrappedToroid(t), 1e-9, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	box1, _ := makeOpenSplineBox(t, [3]float64{4, 0, 0}, [3]float64{5, 1, 1}, 0)
	box2, _ := makeOpenSplineBox(t, [3]float64{7, 0, 0}, [3]float64{8, 1, 1}, 0)
	one, h, e := CombineSplineShells(ctx, toroid, box1, parametric.Union, SplineShellCombineOptions{})
	if e != nil || h.Method != "disjoint-components" || len(one.Topology().Shells) != 2 || len(h.EdgesA) != 2 ||
		one.faces[h.FacesA[0][0]].Surface != toroid.faces[0].Surface {
		t.Fatal(h, e)
	}
	two, h, e := CombineSplineShells(ctx, one, box2, parametric.Union, SplineShellCombineOptions{})
	if e != nil || len(two.Topology().Shells) != 3 || two.Topology().EulerCharacteristic != 4 {
		t.Fatal(h, two.Topology(), e)
	}
	proof, e := two.ValidateDisjointSplineComponents(ctx, SplineRectangleEmbeddingOptions{})
	if e != nil || len(proof) != 3 || proof[0].Toroid == nil || proof[1].Rectangle == nil || proof[2].Rectangle == nil {
		t.Fatal(proof, e)
	}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		out, h, e := CombineSplineShells(ctx, one, box2, op, SplineShellCombineOptions{})
		if e != nil || h.Method != "disjoint-components" {
			t.Fatal(h, e)
		}
		want := 0
		if op == parametric.Difference {
			want = len(one.faces)
		}
		if len(out.faces) != want {
			t.Fatal(op, want, len(out.faces))
		}
	}
	if _, _, e := CombineDisjointSplineComponents(ctx, two, box2, parametric.Union, SplineRectangleEmbeddingOptions{}); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("overlap must remain unresolved", e)
	}
}
