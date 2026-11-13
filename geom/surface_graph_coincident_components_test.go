package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func coincidenceGrid(t *testing.T, n int, height func(int, int) float64) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var a, b []Vec3
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			u, v := float64(i)/float64(n), float64(j)/float64(n)
			a = append(a, Vec3{u, v, height(i, j)})
			b = append(b, Vec3{u, v, 0})
		}
	}
	knots := []float64{0, 0}
	for i := 1; i < n; i++ {
		knots = append(knots, float64(i)/float64(n))
	}
	knots = append(knots, 1, 1)
	sa, e := NewBSplineSurface(1, 1, n+1, n+1, a, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	sb, e := NewBSplineSurface(1, 1, n+1, n+1, b, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	pb, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	return pa, pb
}
func TestCoincidentComponentExactHolePerimeter(t *testing.T) {
	a, b := coincidenceGrid(t, 4, func(i, j int) float64 {
		if i == 2 && j == 2 {
			return 1
		}
		return 0
	})
	r, e := FindSplineGraphCoincidentComponents(context.Background(), a, b, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Cells) != 12 || len(r.Components[0].Cycles) != 2 {
		t.Fatal(r, e)
	}
	cycles := r.Components[0].Cycles
	if cycles[0].Hole || !cycles[1].Hole || cycles[0].SignedArea.Cmp(big.NewRat(1, 1)) != 0 || cycles[1].SignedArea.Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatal(cycles)
	}
	for _, cycle := range cycles {
		for i, edge := range cycle.Edges {
			next := cycle.Edges[(i+1)%len(cycle.Edges)]
			if edge.EndCorner != next.StartCorner || !r.Partition.Cells[edge.Cell].Coincident || r.Partition.Cells[edge.Cell].ZeroSides[edge.Edge] != edge.ZeroSide {
				t.Fatal(edge, next)
			}
		}
	}
	// Reversing operands leaves coincidence topology and UV orientation unchanged.
	reverse, e := FindSplineGraphCoincidentComponents(context.Background(), b, a, GraphPointContactOptions{})
	if e != nil || !reverse.Complete || len(reverse.Components) != 1 || reverse.Components[0].Cycles[1].SignedArea.Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatal(reverse, e)
	}
}
func TestCoincidentSeparateComponentsAndCornerAmbiguity(t *testing.T) {
	a, b := coincidenceGrid(t, 4, func(i, j int) float64 {
		if i == 2 {
			return 1
		}
		return 0
	})
	r, e := FindSplineGraphCoincidentComponents(context.Background(), a, b, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 2 {
		t.Fatal(r, e)
	}
	for _, component := range r.Components {
		if len(component.Cycles) != 1 || component.Cycles[0].SignedArea.Cmp(big.NewRat(1, 4)) != 0 {
			t.Fatal(component)
		}
	}
	a, b = coincidenceGrid(t, 2, func(i, j int) float64 {
		if i == 0 && j == 2 || i == 2 && j == 0 {
			return 1
		}
		return 0
	})
	r, e = FindSplineGraphCoincidentComponents(context.Background(), a, b, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Components) != 2 || len(r.AmbiguousCorners) != 1 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = FindSplineGraphCoincidentComponents(ctx, a, b, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestMixedBasisCoincidentPerimeter(t *testing.T) {
	a, b := mixedBasisPartialCoincidence(t)
	r, e := FindSplineGraphCoincidentComponents(context.Background(), a, b, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Cycles) != 1 || r.Components[0].Cycles[0].SignedArea.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(r, e)
	}
}
