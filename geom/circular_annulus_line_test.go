package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestCircularAnnulusLine(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(-1, 2)}
	q := func(y, a, b float64) CircularAnnulusLineIntersections {
		t.Helper()
		r, e := IntersectCircularAnnulusLine(ctx, center, big.NewRat(1, 1), big.NewRat(2, 1),
			Vec2{X: -3.5, Y: y}, Vec2{X: 1}, a, b, o)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	check := func(r ToroidLineRoot, x float64) {
		t.Helper()
		a, _ := r.Min.Float64()
		b, _ := r.Max.Float64()
		if x < a-1e-15 || x > b+1e-15 || b-a > o.ParameterTolerance*1.01 {
			t.Fatal(a, b, x)
		}
	}
	r := q(-.25, 0, 8)
	if len(r.Rims) != 4 || len(r.Spans) != 2 || r.Rims[0].Inner || !r.Rims[1].Inner || !r.Rims[2].Inner || r.Rims[3].Inner {
		t.Fatal(r)
	}
	outer, inner := math.Sqrt(4-.0625), math.Sqrt(1-.0625)
	for i, span := range r.Spans {
		for j, root := range span {
			check(root, [2][2]float64{{4 - outer, 4 - inner}, {4 + inner, 4 + outer}}[i][j])
		}
	}
	r = q(.5, 0, 8)
	if len(r.Rims) != 3 || !r.Rims[1].Inner || r.Rims[1].Parameter.Multiplicity != 2 || len(r.Spans) != 1 {
		t.Fatal("inner tangent does not disconnect material", r)
	}
	r = q(1.5, 0, 8)
	if len(r.Rims) != 1 || r.Rims[0].Parameter.Multiplicity != 2 || len(r.Spans) != 0 {
		t.Fatal("outer tangent is a point", r)
	}
	r = q(-.5, 2.5, 2.5)
	if r.Point == nil || len(r.Spans) != 0 || len(r.Rims) != 0 {
		t.Fatal(r)
	}
	r = q(-.5, 3.25, 4.75)
	if r.Point != nil || len(r.Spans) != 0 || len(r.Rims) != 0 {
		t.Fatal("segment lies entirely in hole", r)
	}
	r = q(-.5, 3, 4.75)
	if len(r.Rims) != 1 || len(r.Spans) != 0 || r.Rims[0].Parameter.Min.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("clipped tangent at interval endpoint", r)
	}
	o.ParameterTolerance = 1
	closeRadius := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 36)))
	r, e := IntersectCircularAnnulusLine(ctx, center, big.NewRat(1, 1), closeRadius, Vec2{X: -3.5, Y: -.25}, Vec2{X: 1}, 0, 8, o)
	if e != nil || len(r.Spans) != 2 || r.Spans[0][0].Max.Cmp(r.Spans[0][1].Min) >= 0 {
		t.Fatal("near rims need exact ordering even with loose tolerance", r, e)
	}
	o.MaxSubdivisions = 1
	r, e = IntersectCircularAnnulusLine(ctx, center, big.NewRat(1, 1), closeRadius, Vec2{X: -3.5, Y: -.25}, Vec2{X: 1}, 0, 8, o)
	if !errors.Is(e, ErrConvergence) || len(r.Rims) != 0 || len(r.Spans) != 0 {
		t.Fatal(r, e)
	}
}
