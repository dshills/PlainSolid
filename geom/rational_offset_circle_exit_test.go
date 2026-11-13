package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRationalOffsetCircleExit(t *testing.T) {
	a, e := BoundRationalOffsetCircleExit(context.Background(), big.NewRat(3, 1), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if e != nil || !a.Proven {
		t.Fatal(a, e)
	}
	b, e := BoundRationalOffsetCircleExit(context.Background(), big.NewRat(3, 1), big.NewRat(-1, 3), SurfacePatchEvaluationOptions{})
	if e != nil || a.FirstAbsolute.Cmp(b.FirstAbsolute) != 0 || a.SecondAbsolute.Cmp(b.SecondAbsolute) != 0 || a.Value[0].Cmp(b.Value[0]) != 0 {
		t.Fatal(a, b, e)
	}
	if _, e = BoundRationalOffsetCircleExit(context.Background(), big.NewRat(1, 9), big.NewRat(-1, 3), SurfacePatchEvaluationOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e = BoundRationalOffsetCircleExit(context.Background(), big.NewRat(1, 1), nil, SurfacePatchEvaluationOptions{}); e != ErrInvalid {
		t.Fatal(e)
	}
}

func TestRationalPolarFanBounds(t *testing.T) {
	b, e := BoundRationalPolarFan(context.Background(), big.NewRat(4, 1), big.NewRat(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || b.UU.Cmp(big.NewRat(2, 1)) != 0 || b.UV.Cmp(big.NewRat(2, 1)) != 0 || b.VV.Sign() != 0 {
		t.Fatal(b, e)
	}
}
