package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func mixedBasisPartialCoincidence(t *testing.T) (*SurfacePatch, *SurfacePatch) {
	t.Helper()
	var poles []Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{u, v, u*v + max(u-.5, 0)})
		}
	}
	sa, e := NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := NewSurfacePatch(sa, SurfacePatchOptions{})
	poles = nil
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, .5, 1} {
			poles = append(poles, Vec3{u, v, u * v})
		}
	}
	sb, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := NewSurfacePatch(sb, SurfacePatchOptions{})
	// Reauthor B with nonconstant positive W=(1+u)(1+v), retaining
	// exact rational homogeneous fields for the same (u,v,uv) graph.
	weights := []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(2, 1)}
	product := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(2, 1)}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			b.net[i*3+j] = exactHomogeneous{new(big.Rat).Mul(product[i], weights[j]), new(big.Rat).Mul(weights[i], product[j]), new(big.Rat).Mul(product[i], product[j]), new(big.Rat).Mul(weights[i], weights[j])}
		}
	}
	if e = b.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}

	b, e = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return a, b
}
func TestMixedBasisPartialGraphCoincidence(t *testing.T) {
	a, b := mixedBasisPartialCoincidence(t)
	for swap, pair := range [2][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := PartitionSplineGraphContact(context.Background(), pair[0], pair[1], GraphPointContactOptions{})
		if e != nil || !r.Complete || len(r.Cells) != 4 || len(r.Adjacencies) != 4 || len(r.Pending) != 0 {
			t.Fatal(r, e)
		}
		coincident, ordered, interfaces := 0, 0, 0
		side := 1
		if swap == 1 {
			side = -1
		}
		for _, cell := range r.Cells {
			if cell.Domain.UMax == .5 {
				if !cell.Coincident {
					t.Fatal(cell)
				}
				coincident++
			} else {
				if cell.Coincident || cell.Side != side {
					t.Fatal(cell)
				}
				ordered++
			}
		}
		for _, boundary := range r.ZeroSides {
			if boundary.FixedAxis == 0 && boundary.Fixed.Cmp(big.NewRat(1, 2)) == 0 {
				if len(boundary.Cells) != 2 {
					t.Fatal(boundary)
				}
				interfaces++
			}
		}
		if coincident != 2 || ordered != 2 || interfaces != 2 || len(r.ZeroCorners) != 6 {
			t.Fatal(r)
		}
		// Independent exact witnesses lie on the common region/interface and on
		// the correctly signed separated side despite mismatched source bases.
		for _, u := range []*big.Rat{big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(2, 3)} {
			x, e := pair[0].EvaluateRational(context.Background(), u, big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := pair[1].EvaluateRational(context.Background(), u, big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := 0
			if u.Cmp(big.NewRat(1, 2)) > 0 {
				want = side
			}
			if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != want {
				t.Fatal("independent sign")
			}
		}
	}
}
func TestPartialGraphPartitionNearAndUnresolved(t *testing.T) {
	a, b := mixedBasisPartialCoincidence(t)
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2] = new(big.Rat).Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][3]))
	}
	r, e := PartitionSplineGraphContact(context.Background(), a, b, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.ZeroSides)+len(r.ZeroCorners) != 0 {
		t.Fatal(r, e)
	}
	for _, cell := range r.Cells {
		if cell.Coincident || cell.Side != 1 {
			t.Fatal(cell)
		}
	}
	a, b = graphContactFixture(t, false, true)
	r, e = PartitionSplineGraphContact(context.Background(), a, b, GraphPointContactOptions{})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Pending) == 0 {
		t.Fatal("mixed-sign saddle unsupported", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = PartitionSplineGraphContact(ctx, a, b, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = PartitionSplineGraphContact(context.Background(), a, b, GraphPointContactOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
