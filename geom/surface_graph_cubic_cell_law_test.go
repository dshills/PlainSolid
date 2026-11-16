package geom

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"
)

func TestDiscoverProjectedGraphCubicAcrossOriginalKnotCells(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	opts := GraphPointContactOptions{}
	for _, cusp := range []bool{false, true} {
		r, depth, scale := big.NewRat(13, 16), big.NewRat(1, 16), big.NewRat(1, 1)
		if cusp {
			r, depth, scale = big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 3)
		}
		original := cubicGraphSourceFixture(t, r, big.NewRat(1, 2), depth, scale, false)
		original[1] = independentCubicGraphOwner(t, original[1])
		var refined [2]*SurfacePatch
		for i, knots := range [2][2]float64{{.375, .25}, {3.5, -1}} {
			p, err := original[i].InsertUKnot(knots[0], 1, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			refined[i], err = p.InsertVKnot(knots[1], 1, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}
		for order := range original {
			t.Run(fmt.Sprintf("cusp_%v_order_%d", cusp, order), func(t *testing.T) {
				if cusp {
					a, err := DiscoverProjectedGraphCuspidalCubic(ctx, original[order], original[1-order], axes, order, opts)
					if err != nil {
						t.Fatal(err)
					}
					b, err := DiscoverProjectedGraphCuspidalCubic(ctx, refined[order], refined[1-order], axes, order, opts)
					if err != nil {
						t.Fatal(err)
					}
					cubicCellCompareOperations(t, a, b, &a.ExactOperations, &b.ExactOperations)
					if b, err := DiscoverProjectedGraphCuspidalCubic(ctx, refined[order], refined[1-order], axes, order, GraphPointContactOptions{MaxCells: 1}); !errors.Is(err, ErrConvergence) || b != nil {
						t.Fatal("incomplete original cell inventory accepted", err)
					}
				} else {
					a, err := DiscoverProjectedGraphNodalCubic(ctx, original[order], original[1-order], axes, order, opts)
					if err != nil {
						t.Fatal(err)
					}
					b, err := DiscoverProjectedGraphNodalCubic(ctx, refined[order], refined[1-order], axes, order, opts)
					if err != nil {
						t.Fatal(err)
					}
					cubicCellCompareOperations(t, a, b, &a.ExactOperations, &b.ExactOperations)
				}
			})
		}
	}
}

func cubicCellCompareOperations(t *testing.T, original, refined any, a, b *int) {
	t.Helper()
	if *b <= *a {
		t.Fatal("original knot cells were not charged to the proof", *a, *b)
	}
	*a, *b = 0, 0
	if !reflect.DeepEqual(original, refined) {
		t.Fatal("inserting source knots changed exact charts, points, jets or signed regions")
	}
}

// Add independently controlled hinge heights to the original carrier.
func addCubicCellHinge(t *testing.T, patch *SurfacePatch, knot float64, slope *big.Rat) *SurfacePatch {
	t.Helper()
	return addCubicCellAxisHinge(t, patch, knot, 0, 1, slope)
}

func addCubicCellAxisHinge(t *testing.T, patch *SurfacePatch, knot float64, axis int, side int64, slope *big.Rat) *SurfacePatch {
	t.Helper()
	insert, degree := patch.InsertUKnot, patch.degreeU
	if axis == 1 {
		insert, degree = patch.InsertVKnot, patch.degreeV
	}
	q, err := insert(knot, degree, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knots, count := q.knotsU, q.countU
	if axis == 1 {
		knots, count = q.knotsV, q.countV
	}
	at := new(big.Rat).SetFloat64(knot)
	shifts := make([]*big.Rat, count)
	for i := range shifts {
		g := new(big.Rat)
		for _, k := range knots[i+1 : i+degree+1] {
			g.Add(g, new(big.Rat).SetFloat64(k))
		}
		g.Quo(g, big.NewRat(int64(degree), 1))
		offset := new(big.Rat).Sub(g, at)
		offset.Mul(offset, big.NewRat(side, 1))
		if offset.Sign() > 0 {
			shifts[i] = new(big.Rat).Mul(offset, slope)
		}
	}
	for i := range q.net {
		coordinate := i / q.countV
		if axis == 1 {
			coordinate = i % q.countV
		}
		if shift := shifts[coordinate]; shift != nil {
			p := &q.net[i]
			p[2] = new(big.Rat).Add(p[2], new(big.Rat).Mul(shift, p[3]))
		}
	}
	if err := q.computeBounds(&polynomialBudget{maxOperations: 8000000, maxBits: 16384}); err != nil {
		t.Fatal(err)
	}
	return q
}

func TestDiscoverProjectedGraphCuspidalCubicWithSharedCarrierCrease(t *testing.T) {
	ctx := context.Background()
	src := cubicGraphSourceFixture(t, big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), false)
	axes := [2]int{0, 1}
	opts := GraphPointContactOptions{}
	hinge := .4375
	delta := big.NewRat(1, 16)
	refined := [2]*SurfacePatch{addCubicCellHinge(t, src[0], hinge, delta), addCubicCellHinge(t, src[1], hinge, delta)}
	p, err := DiscoverProjectedGraphCuspidalCubic(ctx, refined[0], refined[1], axes, 0, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Vertices[0].Point[2].RatString() != "137/64" || p.Vertices[1].Point[2].RatString() != "653/256" ||
		p.Vertices[2].Point[2].RatString() != "677/256" || p.Scale.RatString() != "1/3" ||
		p.Regions[0].HeightSide != 1 || p.Regions[1].HeightSide != -1 {
		t.Fatal("wrong creased source vertices or signed regions", p.Vertices)
	}
	for _, u := range []*big.Rat{big.NewRat(27, 64), big.NewRat(29, 64)} {
		a, err := src[0].EvaluateRational(ctx, u, big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		b, err := refined[0].EvaluateRational(ctx, u, big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		want := new(big.Rat)
		if u.Cmp(big.NewRat(7, 16)) > 0 {
			want.Set(delta)
		}
		if new(big.Rat).Sub(b.Du[2], a.Du[2]).Cmp(want) != 0 {
			t.Fatal("test carrier lacks the original physical slope change", u)
		}
	}
	for _, u := range []*big.Rat{big.NewRat(1, 4), big.NewRat(7, 16), big.NewRat(1, 1)} {
		// Exact section points at t=0,±1/4,±1/2 satisfy
		// x=1/4+3*t² and y=1/2+3*t³.
		var tAbs *big.Rat
		switch u.RatString() {
		case "1/4":
			tAbs = new(big.Rat)
		case "7/16":
			tAbs = big.NewRat(1, 4)
		case "1":
			tAbs = big.NewRat(1, 2)
		}
		for _, sign := range []int64{-1, 1} {
			v := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).Mul(big.NewRat(3*sign, 1), new(big.Rat).Mul(tAbs, new(big.Rat).Mul(tAbs, tAbs))))
			a, err := refined[0].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			b, err := refined[1].EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
			if err != nil || a.Point[2].Cmp(b.Point[2]) != 0 {
				t.Fatal("section failed at an independently evaluated knot or endpoint", u, v, err)
			}
		}
	}

	t.Run("unmatched crease changes only later cells", func(t *testing.T) {
		changed := addCubicCellHinge(t, src[1], hinge, big.NewRat(1, 32))
		if out, err := DiscoverProjectedGraphCuspidalCubic(ctx, refined[0], changed, axes, 0, opts); !errors.Is(err, ErrConvergence) || out != nil {
			t.Fatal("accepted a changed residual in the right cells", err)
		}
	})
	t.Run("required cusp jet uses the outgoing longitudinal C0 cell", func(t *testing.T) {
		a, b := addCubicCellHinge(t, src[0], .25, delta), addCubicCellHinge(t, src[1], .25, delta)
		out, err := DiscoverProjectedGraphCuspidalCubic(ctx, a, b, axes, 0, opts)
		if err != nil || out.Germs[0].ModelSecond[2].RatString() != "15/4" || out.Germs[1].ModelSecond[2].RatString() != "15/4" {
			t.Fatal("longitudinal knot jet differs from the analytic departure side", err)
		}
	})
}
