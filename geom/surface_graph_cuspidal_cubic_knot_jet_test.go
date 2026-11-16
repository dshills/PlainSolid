package geom

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func TestDiscoverProjectedGraphCuspidalCubicAtSharedLongitudinalKnot(t *testing.T) {
	ctx := context.Background()
	src := cubicGraphSourceFixture(t, big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), false)
	// The same physical hinge rises toward positive model X. That enters
	// increasing U on A and decreasing V on B, which uses (2+2Y,1-4X).
	roofs := [2]*SurfacePatch{
		addCubicCellAxisHinge(t, src[0], .25, 0, 1, big.NewRat(1, 16)),
		addCubicCellAxisHinge(t, independentCubicGraphOwner(t, src[1]), 0, 1, -1, big.NewRat(1, 64)),
	}
	bu, bv := big.NewRat(3, 1), new(big.Rat)
	defaultB, err := roofs[1].EvaluateRational(ctx, bu, bv, SurfacePatchEvaluationOptions{})
	if err != nil || defaultB.Dv[2].RatString() != "-9/64" {
		t.Fatal("the default B evaluator must continue to use its original right cell", err)
	}
	left, _, err := roofs[1].SplitV(0)
	if err != nil {
		t.Fatal(err)
	}
	departureB, err := left.EvaluateRational(ctx, bu, bv, SurfacePatchEvaluationOptions{})
	if err != nil || departureB.Dv[2].RatString() != "-5/32" {
		t.Fatal("independent left-cell evaluation does not contain the genuine hinge", err)
	}

	for order := range 2 {
		t.Run(fmt.Sprintf("order_%d", order), func(t *testing.T) {
			axes := [2]int{0, 1}
			p, err := DiscoverProjectedGraphCuspidalCubic(ctx, roofs[order], roofs[1-order], axes, order, GraphPointContactOptions{})
			if err != nil {
				t.Fatal(err)
			}
			cusp := p.Vertices[0]
			if cusp.Point[2].RatString() != "137/64" || cusp.UV[order][0].RatString() != "1/4" ||
				cusp.UV[1-order][1].Sign() != 0 || p.SourceDirection != 1-2*order ||
				p.Regions[0].HeightSide != 1-2*order || p.Regions[1].HeightSide != 2*order-1 {
				t.Fatal("joint reproof changed original cusp identity or signed regions", p)
			}
			for i, germ := range p.Germs {
				second := [3]string{"6", "0", "15/4"}
				third := [2][3]string{{"0", "-18", "-9/16"}, {"0", "18", "9/16"}}[i]
				terminal := [2][3]string{{"3", "-9/4", "93/64"}, {"3", "9/4", "147/64"}}[i]
				for axis := range 3 {
					if germ.ModelSecond[axis].RatString() != second[axis] ||
						germ.ModelThird[axis].RatString() != third[axis] ||
						germ.TerminalModel[axis].RatString() != terminal[axis] {
						t.Fatal("wrong source second, third or terminal jet", i, germ)
					}
				}
				if germ.UVSecond[order][0].RatString() != "6" ||
					germ.UVSecond[1-order][1].RatString() != "-24" {
					t.Fatal("reversed original clock did not enter its left cell", germ)
				}
				for _, q := range []*big.Rat{big.NewRat(1, 16), big.NewRat(1, 2)} {
					x := new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(q, q))
					y := new(big.Rat).Mul(big.NewRat(int64(2*i-1), 1), new(big.Rat).Mul(q, x))
					u, v := new(big.Rat).Add(big.NewRat(1, 4), x), new(big.Rat).Add(big.NewRat(1, 2), y)
					z := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Mul(u, big.NewRat(1, 2)))
					z.Add(z, new(big.Rat).Mul(u, new(big.Rat).Mul(v, big.NewRat(1, 8))))
					z.Add(z, new(big.Rat).Mul(x, big.NewRat(1, 16)))
					param := [2][2]*big.Rat{{u, v}, {
						new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Mul(v, big.NewRat(2, 1))),
						new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(u, big.NewRat(4, 1))),
					}}
					for owner, roof := range roofs {
						uv := param[owner]
						got, err := roof.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
						if err != nil || got.Point[2].Cmp(z) != 0 {
							t.Fatal("original open germ differs from independently authored physical roof", i, q, owner, err)
						}
					}
				}
			}
			limited := GraphPointContactOptions{MaxOperations: p.ExactOperations - 1}
			if out, err := DiscoverProjectedGraphCuspidalCubic(ctx, roofs[order], roofs[1-order], axes, order, limited); !errors.Is(err, ErrConvergence) || out != nil {
				t.Fatal("resource-truncated one-sided proof was published", err)
			}
		})
	}
}

func TestCubicGraphKnotJetAmbiguitiesAreRefused(t *testing.T) {
	ctx := context.Background()
	axes, opts := [2]int{0, 1}, GraphPointContactOptions{}
	cusp := cubicGraphSourceFixture(t, big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), false)
	for _, test := range []struct {
		name string
		knot float64
	}{
		{"opposite departure sides at a dependent cusp knot", .5},
		{"ambiguous terminal C0 knot", .125},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := addCubicCellAxisHinge(t, cusp[0], test.knot, 1, 1, big.NewRat(1, 16))
			b := addCubicCellAxisHinge(t, cusp[1], test.knot, 1, 1, big.NewRat(1, 16))
			if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, a, b, axes, 0, opts); !errors.Is(err, ErrConvergence) || p != nil {
				t.Fatal("the scoped longitudinal proof accepted ambiguous source jets", err)
			}
		})
	}
	a := addCubicCellHinge(t, cusp[0], .25, big.NewRat(1, 16))
	b := addCubicCellHinge(t, cusp[1], .25, big.NewRat(1, 32))
	if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, a, b, axes, 0, opts); !errors.Is(err, ErrConvergence) || p != nil {
		t.Fatal("one-sided support evaluation admitted an unmatched whole-domain crease", err)
	}
	node := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), big.NewRat(1, 1), false)
	a = addCubicCellHinge(t, node[0], .8125, big.NewRat(1, 16))
	b = addCubicCellHinge(t, node[1], .8125, big.NewRat(1, 16))
	if p, err := DiscoverProjectedGraphNodalCubic(ctx, a, b, axes, 0, opts); !errors.Is(err, ErrConvergence) || p != nil {
		t.Fatal("transverse node selected an unproved one-sided knot jet", err)
	}
	isolated := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(-1, 8), big.NewRat(1, 1), false)
	a = addCubicCellHinge(t, isolated[0], .8125, big.NewRat(1, 16))
	b = addCubicCellHinge(t, isolated[1], .8125, big.NewRat(1, 16))
	if p, err := DiscoverProjectedGraphAcnodalCubic(ctx, a, b, axes, 0, opts); !errors.Is(err, ErrConvergence) || p != nil {
		t.Fatal("isolated node selected an unproved one-sided knot jet", err)
	}
}
