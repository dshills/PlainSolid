package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSurfaceExactTensorAcrossBothSeams(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 2, DegreeV: 1, CountU: 3, CountV: 3,
		Poles:   []Vec3{{0, 0, 0}, {0, 1, 1}, {1, 2, 2}, {1, 0, 2}, {2, 1, 3}, {1, 3, 4}, {3, 0, 1}, {4, 1, 1}, {2, 3, 3}},
		Weights: []float64{2, 3, 2, 3, 1, 2, 1, 2, 3},
	}
	axisU, axisV := &PeriodicSplineAxis{Start: -2, Period: 1}, &PeriodicSplineAxis{Start: 1, Period: 2}
	surface, err := NewPeriodicBSplineSurface(data, axisU, axisV)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uWindow := ExactParameterInterval{Lower: big.NewRat(-9, 4), Upper: big.NewRat(-5, 3)}
	vWindow := ExactParameterInterval{Lower: big.NewRat(11, 4), Upper: big.NewRat(17, 5)}
	cells, work, err := surface.ExactBezierCells(ctx, uWindow, vWindow, PeriodicSurfaceBezierOptions{})
	if err != nil || len(cells) != 4 || work == 0 {
		t.Fatalf("cells=%d, work=%d, error=%v", len(cells), work, err)
	}
	owners := [][4]int64{{-1, 2, 0, 2}, {-1, 2, 1, 0}, {0, 0, 0, 2}, {0, 0, 1, 0}}
	for i, cell := range cells {
		ownersGot := [4]int64{cell.U.Cycle.Int64(), int64(cell.U.Span), cell.V.Cycle.Int64(), int64(cell.V.Span)}
		if ownersGot != owners[i] {
			t.Fatalf("cell %d owner=%v, want %v", i, ownersGot, owners[i])
		}
		patch, err := cell.NormalizedPatch(ctx, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range []*big.Rat{big.NewRat(0, 1), big.NewRat(2, 7), big.NewRat(1, 1)} {
			for _, v := range []*big.Rat{big.NewRat(0, 1), big.NewRat(3, 5), big.NewRat(1, 1)} {
				pu := exactPeriodicCellParameter(cell.U, u)
				pv := exactPeriodicCellParameter(cell.V, v)
				au := exactPeriodicTensorLocal(pu, cell.U, big.NewRat(-2, 1), big.NewRat(1, 3))
				av := exactPeriodicTensorLocal(pv, cell.V, big.NewRat(1, 1), big.NewRat(2, 3))
				du := new(big.Rat).Sub(big.NewRat(1, 1), au)
				b0 := new(big.Rat).Quo(new(big.Rat).Mul(du, du), big.NewRat(2, 1))
				b2 := new(big.Rat).Quo(new(big.Rat).Mul(au, au), big.NewRat(2, 1))
				b1 := new(big.Rat).Sub(new(big.Rat).Sub(big.NewRat(1, 1), b0), b2)
				want := directPeriodicTensorPoint(data, cell, []*big.Rat{b0, b1, b2}, []*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), av), av})
				got, err := patch.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				for axis := range want {
					if got.Point[axis].Cmp(want[axis]) != 0 {
						t.Fatalf("cell %d at %v,%v coordinate %d: got %s, want %s", i, pu, pv, axis, got.Point[axis], want[axis])
					}
				}
			}
		}
	}
	if cells[0].U.Domain.Upper.Cmp(big.NewRat(-2, 1)) != 0 || cells[0].V.Domain.Upper.Cmp(big.NewRat(3, 1)) != 0 || cells[3].U.Domain.Upper.Cmp(uWindow.Upper) != 0 || cells[3].V.Domain.Upper.Cmp(vWindow.Upper) != 0 {
		t.Fatal("missed an original physical seam or clipped endpoint")
	}
	cells[0].U.Domain.Lower.SetInt64(78)
	cells[0].V.Cycle.SetInt64(78)
	if cells[1].U.Domain.Lower.Cmp(uWindow.Lower) != 0 || cells[2].V.Cycle.Sign() != 0 || uWindow.Lower.Cmp(big.NewRat(-9, 4)) != 0 {
		t.Fatal("cell parameters are not independently owned")
	}
	for _, options := range []PeriodicSurfaceBezierOptions{
		{MaxCells: 3}, {SurfacePatchOptions: SurfacePatchOptions{MaxPoles: 23}},
		{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: work - 1}},
	} {
		limited, _, err := surface.ExactBezierCells(ctx, uWindow, vWindow, options)
		if len(limited) != 0 || !errors.Is(err, ErrConvergence) {
			t.Fatalf("limit %v published %d cells, error %v", options, len(limited), err)
		}
	}
}

func TestPeriodicSurfaceExactTensorMixedAxisAndFarCycle(t *testing.T) {
	data := BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 3,
		KnotsV: []float64{-2, -2, .25, 1, 1},
		Poles:  []Vec3{{0, 0, 0}, {0, 0, 3}, {0, 0, 4}, {2, 0, 0}, {2, 0, 3}, {2, 0, 4}, {0, 2, 0}, {0, 2, 3}, {0, 2, 4}},
	}
	surface, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	a := new(big.Rat).SetInt(cycle)
	u := ExactParameterInterval{Lower: a, Upper: new(big.Rat).Add(a, big.NewRat(1, 3))}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(2, 3)}
	cells, _, err := surface.ExactBezierCells(ctx, u, v, PeriodicSurfaceBezierOptions{})
	if err != nil || len(cells) != 2 {
		t.Fatalf("mixed cells=%d, error=%v", len(cells), err)
	}
	for i, cell := range cells {
		if cell.U.Cycle.Cmp(cycle) != 0 || cell.U.Span != 0 || cell.V.Cycle != nil || cell.V.Span != i {
			t.Fatalf("mixed owner %+v, ordinary owner %+v", cell.U, cell.V)
		}
		patch, err := cell.NormalizedPatch(ctx, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		point, err := patch.EvaluateRational(ctx, big.NewRat(2, 7), big.NewRat(3, 7), SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		physicalV := exactPeriodicCellParameter(cell.V, big.NewRat(3, 7))
		// Both ordinary source spans have Z=(4v+8)/3.
		z := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(4, 1), physicalV), big.NewRat(8, 1)), big.NewRat(3, 1))
		if point.Point[0].Cmp(big.NewRat(4, 7)) != 0 || point.Point[1].Sign() != 0 || point.Point[2].Cmp(z) != 0 {
			t.Fatalf("mixed point %v, expected (4/7,0,%v)", point.Point, z)
		}
	}
	if cells[0].V.Domain.Upper.Cmp(big.NewRat(1, 4)) != 0 || cells[1].V.Domain.Lower.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal("ordinary knot missing")
	}
	v.Lower = big.NewRat(-3, 1)
	if result, _, err := surface.ExactBezierCells(ctx, u, v, PeriodicSurfaceBezierOptions{}); len(result) != 0 || !errors.Is(err, ErrDomain) {
		t.Fatalf("out-of-domain ordinary axis published %d cells, error %v", len(result), err)
	}
}

func exactPeriodicCellParameter(span ExactSurfaceKnotSpan, fraction *big.Rat) *big.Rat {
	width := new(big.Rat).Sub(span.Domain.Upper, span.Domain.Lower)
	return new(big.Rat).Add(span.Domain.Lower, new(big.Rat).Mul(fraction, width))
}

func exactPeriodicTensorLocal(parameter *big.Rat, span ExactSurfaceKnotSpan, start, width *big.Rat) *big.Rat {
	x := new(big.Rat).Quo(new(big.Rat).Sub(parameter, start), width)
	x.Sub(x, new(big.Rat).SetInt(new(big.Int).Mul(span.Cycle, big.NewInt(3))))
	return x.Sub(x, big.NewRat(int64(span.Span), 1))
}

func directPeriodicTensorPoint(data BSplineSurfaceData, cell ExactPeriodicSurfaceBezierCell, u, v []*big.Rat) [3]*big.Rat {
	x, y, z, w := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
	for i, au := range u {
		for j, av := range v {
			index := ((cell.U.Span+i)%data.CountU)*data.CountV + (cell.V.Span+j)%data.CountV
			factor := new(big.Rat).Mul(new(big.Rat).Mul(au, av), new(big.Rat).SetFloat64(data.Weights[index]))
			p := data.Poles[index]
			x.Add(x, new(big.Rat).Mul(factor, new(big.Rat).SetFloat64(p.X)))
			y.Add(y, new(big.Rat).Mul(factor, new(big.Rat).SetFloat64(p.Y)))
			z.Add(z, new(big.Rat).Mul(factor, new(big.Rat).SetFloat64(p.Z)))
			w.Add(w, factor)
		}
	}
	return [3]*big.Rat{x.Quo(x, w), y.Quo(y, w), z.Quo(z, w)}
}
