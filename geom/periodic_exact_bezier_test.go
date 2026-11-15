package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactPeriodicBezierSpansAcrossBothSeams(t *testing.T) {
	poles := []Vec3{{1, 2, 0}, {3, -1, 1}, {4, 0, 3}, {-1, 3, -2}, {-2, 0, 1}}
	weights := []float64{1, .5, .25, .75, .125}
	curve, err := NewPeriodicBSpline(3, poles, weights, 1.25, 1.5)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := big.NewRat(11, 10), big.NewRat(57, 20)
	pieces, work, err := curve.ExactBezierSpans(context.Background(), lo, hi, ExactBezierSpanOptions{})
	if err != nil || len(pieces) != 7 || work <= 0 {
		t.Fatalf("expected seven exact pieces, got %d, %d operations, %v", len(pieces), work, err)
	}
	if pieces[0].Start.Cmp(lo) != 0 || pieces[6].End.Cmp(hi) != 0 || pieces[2].Start.Cmp(big.NewRat(31, 20)) != 0 {
		t.Fatal("trimmed endpoints or the nonbinary physical knot are incorrect")
	}
	for i, piece := range pieces {
		cycle := int64(0)
		if i == 0 {
			cycle = -1
		}
		if i == 6 {
			cycle = 1
		}
		if piece.Cycle.Cmp(big.NewInt(cycle)) != 0 || piece.Span != (i+4)%len(poles) {
			t.Fatalf("wrong original owner of piece %d: %v/%d", i, piece.Cycle, piece.Span)
		}
		if i != 0 && pieces[i-1].End.Cmp(piece.Start) != 0 {
			t.Fatal("gap in the physical parameter range", i)
		}
		original := new(big.Rat).Add(big.NewRat(5, 4), big.NewRat(int64(i-1)*3, 10))
		for _, u := range []*big.Rat{new(big.Rat), big.NewRat(2, 7), big.NewRat(1, 1)} {
			actual := evaluatePeriodicBezier(piece.Control, u)
			parameter := new(big.Rat).Add(piece.Start, new(big.Rat).Mul(u, new(big.Rat).Sub(piece.End, piece.Start)))
			source := new(big.Rat).Quo(new(big.Rat).Sub(parameter, original), big.NewRat(3, 10))
			expected := evaluateCyclicCubicBasis(poles, weights, piece.Span, source)
			for axis := range actual {
				if actual[axis].Cmp(expected[axis]) != 0 {
					t.Fatalf("piece %d at %v, homogeneous coordinate %d: got %v, want %v", i, parameter, axis, actual[axis], expected[axis])
				}
			}
		}
	}
	// Shared endpoints and the requested range are independently owned.
	pieces[0].Control[3][0].SetInt64(99)
	pieces[0].End.SetInt64(99)
	pieces[0].Start.SetInt64(99)
	if pieces[1].Control[0][0].Cmp(big.NewRat(99, 1)) == 0 || pieces[1].Start.Cmp(big.NewRat(5, 4)) != 0 || lo.Cmp(big.NewRat(11, 10)) != 0 {
		t.Fatal("returned data aliases another piece or the argument")
	}
}

func TestExactPeriodicBezierSpansFarCyclesAndLimits(t *testing.T) {
	curve, err := NewPeriodicBSpline(1, []Vec3{{1, 2, 0}, {3, 2, 0}, {1, 4, 0}}, nil, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	lo := new(big.Rat).Add(new(big.Rat).SetInt(cycle), big.NewRat(1, 3))
	hi := new(big.Rat).Add(new(big.Rat).SetInt(cycle), big.NewRat(2, 3))
	pieces, _, err := curve.ExactBezierSpans(ctx, lo, hi, ExactBezierSpanOptions{})
	if err != nil || len(pieces) != 1 || pieces[0].Cycle.Cmp(cycle) != 0 || pieces[0].Span != 1 ||
		pieces[0].Control[0][0].Cmp(big.NewRat(3, 1)) != 0 || pieces[0].Control[1][1].Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("huge exact cycle lost its phase or original span", pieces, err)
	}
	for _, tc := range []struct {
		start, end *big.Rat
		options    ExactBezierSpanOptions
	}{
		{lo, hi, ExactBezierSpanOptions{SurfacePatchOptions: SurfacePatchOptions{MaxCoefficientBits: 64}}},
		{new(big.Rat), big.NewRat(1, 1), ExactBezierSpanOptions{MaxSpans: 2}},
		{new(big.Rat), big.NewRat(1, 1), ExactBezierSpanOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 10}}},
		{new(big.Rat), big.NewRat(1, 1), ExactBezierSpanOptions{SurfacePatchOptions: SurfacePatchOptions{MaxPoles: 5}}},
	} {
		if pieces, _, err := curve.ExactBezierSpans(ctx, tc.start, tc.end, tc.options); len(pieces) != 0 || !errors.Is(err, ErrConvergence) {
			t.Fatal("range or work limit published pieces", pieces, err)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if pieces, _, err := curve.ExactBezierSpans(ctx, lo, hi, ExactBezierSpanOptions{}); len(pieces) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled extraction published pieces", pieces, err)
	}
}

func evaluatePeriodicBezier(control [][4]*big.Rat, t *big.Rat) [4]*big.Rat {
	work := append([][4]*big.Rat(nil), control...)
	v := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for n := len(work) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			for axis := range work[i] {
				work[i][axis] = new(big.Rat).Add(new(big.Rat).Mul(v, work[i][axis]), new(big.Rat).Mul(t, work[i+1][axis]))
			}
		}
	}
	return work[0]
}

func evaluateCyclicCubicBasis(poles []Vec3, weights []float64, span int, t *big.Rat) [4]*big.Rat {
	t2 := new(big.Rat).Mul(t, t)
	t3 := new(big.Rat).Mul(t2, t)
	b1 := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 1), t3), big.NewRat(4, 1))
	b1.Sub(b1, new(big.Rat).Mul(big.NewRat(6, 1), t2))
	b2 := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(3, 1), t3))
	b2.Add(b2, new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Add(t, t2)))
	v := new(big.Rat).Sub(big.NewRat(1, 1), t)
	b0 := new(big.Rat).Mul(v, new(big.Rat).Mul(v, v))
	out := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	for j, factor := range []*big.Rat{b0, b1, b2, t3} {
		i := (span + j) % len(poles)
		weight := new(big.Rat).Mul(new(big.Rat).SetFloat64(weights[i]), new(big.Rat).Quo(factor, big.NewRat(6, 1)))
		out[3].Add(out[3], weight)
		for axis := 0; axis < 3; axis++ {
			out[axis].Add(out[axis], new(big.Rat).Mul(weight, new(big.Rat).SetFloat64(coordinate(poles[i], axis))))
		}
	}
	return out
}
