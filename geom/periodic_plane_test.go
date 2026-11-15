package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSplinePlaneSeamsAndCoincidence(t *testing.T) {
	curve, err := NewPeriodicBSpline(1, []Vec3{
		{0, 0, 0}, {1, 1, 0}, {0, 2, 0}, {0, 3, 0}, {-1, 4, 0},
	}, nil, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	plane, err := NewPlane(Vec3{}, Vec3{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start, end := big.NewRat(-1, 10), big.NewRat(11, 10)
	got, err := curve.IntersectPlaneExact(ctx, plane, start, end, PeriodicSplinePlaneOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		parameter *big.Rat
		cycle     int64
		span      int
	}{
		{big.NewRat(0, 1), -1, 4}, {big.NewRat(0, 1), 0, 0},
		{big.NewRat(2, 5), 0, 1}, {big.NewRat(3, 5), 0, 3},
		{big.NewRat(1, 1), 0, 4}, {big.NewRat(1, 1), 1, 0},
	}
	if len(got.Hits) != len(want) || len(got.Coincident) != 1 {
		t.Fatalf("got %d hits and %d coincident pieces", len(got.Hits), len(got.Coincident))
	}
	for i, hit := range got.Hits {
		root := hit.Parameter.Interval()
		w := want[i]
		if root.Lower.Cmp(w.parameter) != 0 || root.Upper.Cmp(w.parameter) != 0 || hit.Cycle.Cmp(big.NewInt(w.cycle)) != 0 || hit.Span != w.span || hit.Multiplicity != 1 {
			t.Fatalf("hit %d has interval %+v and owner %+v, want %+v", i, root, hit, w)
		}
	}
	piece := got.Coincident[0]
	if piece.Cycle.Sign() != 0 || piece.Span != 2 || piece.Start.Cmp(big.NewRat(2, 5)) != 0 || piece.End.Cmp(big.NewRat(3, 5)) != 0 || piece.Control[0][0].Sign() != 0 || piece.Control[1][0].Sign() != 0 {
		t.Fatalf("coincident source piece = %+v", piece)
	}
	limited, err := curve.IntersectPlaneExact(ctx, plane, start, end, PeriodicSplinePlaneOptions{
		ExactBezierSpanOptions: ExactBezierSpanOptions{
			SurfacePatchOptions: SurfacePatchOptions{MaxOperations: got.Operations - 1},
		},
	})
	if !errors.Is(err, ErrConvergence) || len(limited.Hits) != 0 || len(limited.Coincident) != 0 {
		t.Fatalf("limited proof = %+v, %v", limited, err)
	}
}

func TestPeriodicSplinePlaneAlgebraicRoots(t *testing.T) {
	ctx := context.Background()
	plane, err := NewPlane(Vec3{}, Vec3{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		poles        []Vec3
		polynomial   []*big.Rat
		multiplicity int
		kind         ExactRootKind
	}{
		{"crossing", []Vec3{{1, 0, 0}, {-2, 1, 0}, {4, 2, 0}}, []*big.Rat{big.NewRat(-1, 1), big.NewRat(-18, 1), big.NewRat(81, 1)}, 1, ExactRootKindIrrational},
		{"tangency", []Vec3{{3, 0, 0}, {-1, 1, 0}, {3, 2, 0}}, []*big.Rat{big.NewRat(-1, 1), big.NewRat(6, 1)}, 2, ExactRootKindRational},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curve, err := NewPeriodicBSpline(2, tc.poles, nil, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			got, err := curve.IntersectPlaneExact(ctx, plane, big.NewRat(0, 1), big.NewRat(1, 3), PeriodicSplinePlaneOptions{})
			if err != nil || len(got.Hits) != 1 || len(got.Coincident) != 0 {
				t.Fatalf("intersection = %+v, %v", got, err)
			}
			hit := got.Hits[0]
			if hit.Cycle.Sign() != 0 || hit.Span != 0 || hit.Multiplicity != tc.multiplicity {
				t.Fatalf("hit = %+v", hit)
			}
			sign, err := hit.Parameter.SignPolynomial(ctx, tc.polynomial, ExactCurvePlaneEventOptions{})
			if err != nil || sign.Sign != 0 {
				t.Fatalf("exact sign = %+v, %v", sign, err)
			}
			class, err := hit.Parameter.Classify(ctx, ExactCurvePlaneEventOptions{})
			if err != nil || class.Kind != tc.kind {
				t.Fatalf("root classification = %+v, %v", class, err)
			}
		})
	}
}
