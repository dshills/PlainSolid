package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSignedCellUnionMixedExteriorAndZeroPerimeter(t *testing.T) {
	a, b := mixedBasisPartialCoincidence(t)
	for _, pair := range []struct {
		a, b *SurfacePatch
		side int
	}{{a, b, 1}, {b, a, -1}} {
		r, e := FindSplineGraphSignedComponents(context.Background(), pair.a, pair.b, pair.side, GraphPointContactOptions{})
		if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Cycles) != 1 {
			t.Fatal(r, e)
		}
		cycle := r.Components[0].Cycles[0]
		if cycle.SignedArea.Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(cycle)
		}
		contacts, exterior := 0, 0
		for i, edge := range cycle.Edges {
			if edge.ZeroSide >= 0 {
				contacts++
				if edge.Exterior {
					t.Fatal("interior contact marked exterior", edge)
				}
			}
			if edge.Exterior {
				exterior++
			}
			if edge.EndCorner != cycle.Edges[(i+1)%len(cycle.Edges)].StartCorner {
				t.Fatal("open perimeter")
			}
		}
		if contacts != 2 || exterior != 4 {
			t.Fatal(contacts, exterior)
		}
	}
	// A single positive cell with two exact zero exterior sides.
	a, b = coincidenceGrid(t, 1, func(i, j int) float64 { return float64(i * j) })
	r, e := FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 {
		t.Fatal(r, e)
	}
	zero := 0
	for _, edge := range r.Components[0].Cycles[0].Edges {
		if !edge.Exterior {
			t.Fatal(edge)
		}
		if edge.ZeroSide >= 0 {
			zero++
		}
	}
	if zero != 2 {
		t.Fatal(zero)
	}
}
func TestSignedCellUnionHoleAndRetainedInternalContacts(t *testing.T) {
	a, b := coincidenceGrid(t, 4, func(i, j int) float64 {
		if i == 0 || j == 0 || i == 4 || j == 4 {
			return 1
		}
		return 0
	})
	r, e := FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Cycles) != 2 {
		t.Fatal(r, e)
	}
	cycles := r.Components[0].Cycles
	if cycles[0].SignedArea.Cmp(big.NewRat(1, 1)) != 0 || cycles[1].SignedArea.Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatal(cycles)
	}
	for _, edge := range cycles[1].Edges {
		if edge.ZeroSide < 0 || edge.Exterior {
			t.Fatal(edge)
		}
	}
	a, b = coincidenceGrid(t, 2, func(i, j int) float64 {
		if i != 1 {
			return .5
		}
		return 0
	})
	r, e = FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].InternalZeroSides) != 2 {
		t.Fatal(r, e)
	}
	if r.Components[0].Cycles[0].SignedArea.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(r)
	}
}
func TestSignedCellUnionPinchAndNearZero(t *testing.T) {
	a, b := coincidenceGrid(t, 2, func(i, j int) float64 {
		if i == 0 && j == 0 || i == 2 && j == 2 {
			return 1
		}
		return 0
	})
	r, e := FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.AmbiguousCorners) != 1 {
		t.Fatal(r, e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][3]))
	}
	r, e = FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].InternalZeroSides) != 0 || len(r.Partition.ZeroSides) != 0 {
		t.Fatal(r, e)
	}
	if _, e = FindSplineGraphSignedComponents(context.Background(), a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e = FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
func TestSignedCellUnionRetainsIsolatedZeroCorner(t *testing.T) {
	a, b := coincidenceGrid(t, 2, func(i, j int) float64 {
		x, y := i-1, j-1
		if x < 0 {
			x = -x
		}
		if y < 0 {
			y = -y
		}
		return float64(x + y)
	})
	r, e := FindSplineGraphSignedComponents(context.Background(), a, b, 1, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].InternalZeroSides) != 0 || len(r.Components[0].ContactCorners) != 1 {
		t.Fatal(r, e)
	}
	corner := r.Partition.ZeroCorners[r.Components[0].ContactCorners[0]]
	if corner.Parameters[0].Cmp(big.NewRat(1, 2)) != 0 || corner.Parameters[1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(corner)
	}
}
