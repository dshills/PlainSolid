package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveSurfaceHeightDifferencePreservesOriginalUnionAndTinySigns(t *testing.T) {
	ctx := context.Background()
	zero, one := new(big.Rat), big.NewRat(1, 1)
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, rotated := range []bool{false, true} {
		axes := [2]int{0, 1}
		if rotated {
			axes = [2]int{2, 0}
		}
		a := arcTestPatch(t, func(u, v float64) Vec3 {
			if rotated {
				return Vec3{v, u * v, u}
			}
			return Vec3{u, v, u * v}
		})
		b := projectiveOverlapTestChart(t, rotated, false)
		for i, p := range b.net {
			// An independent height law z_B = x*y + 2^-120*(x-1/2).
			term := new(big.Rat).Sub(p[axes[0]], new(big.Rat).Quo(p[3], big.NewRat(2, 1)))
			b.net[i][3-axes[0]-axes[1]] = new(big.Rat).Add(p[3-axes[0]-axes[1]], term.Mul(term, tiny))
		}
		budget := &polynomialBudget{maxOperations: b.options.MaxOperations, maxBits: b.options.MaxCoefficientBits}
		if err := b.computeBounds(budget); err != nil {
			t.Fatal(err)
		}
		r, err := BuildProjectiveSurfaceHeightDifference(ctx, a, b, axes, GeneralSurfaceHeightOptions{})
		if err != nil || !r.Complete || r.Overlap.Coincident || r.Overlap.NoArea || len(r.Cells) != 4 {
			t.Fatal("failed to build bounded exact common height cells", rotated, r, err)
		}
		if r.Cells[0].Common.U.Upper.Cmp(big.NewRat(1, 4)) != 0 || r.Cells[0].Common.V.Upper.Cmp(big.NewRat(9, 10)) != 0 ||
			r.Cells[1].Common.V.Lower.Cmp(big.NewRat(9, 10)) != 0 || r.Cells[2].Common.U.Lower.Cmp(big.NewRat(1, 4)) != 0 {
			t.Fatal("rounded or reordered an original nonbinary union cut", r.Cells)
		}
		signs := map[int]bool{}
		for _, c := range r.Cells {
			if !c.OriginalCells[0].Valid() || !c.OriginalCells[1].Valid() || c.Difference.Domain() != (ParameterRectangle{0, 1, 0, 1}) {
				t.Fatal("missing original knot-cell owner", c)
			}
			for _, u := range [4]*big.Rat{zero, big.NewRat(1, 3), big.NewRat(37, 101), one} {
				for _, v := range [3]*big.Rat{zero, big.NewRat(5, 8), one} {
					p, err := c.Difference.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					lerp := func(d ExactParameterInterval, t *big.Rat) *big.Rat {
						return new(big.Rat).Add(d.Lower, new(big.Rat).Mul(t, new(big.Rat).Sub(d.Upper, d.Lower)))
					}
					worldX := lerp(c.Common.U, u)
					worldY := new(big.Rat).Quo(new(big.Rat).Add(one, lerp(c.Common.V, v)), big.NewRat(2, 1))
					want := new(big.Rat).Sub(big.NewRat(1, 2), worldX)
					want.Mul(want, tiny)
					if p.Point[0].Cmp(worldX) != 0 || p.Point[1].Cmp(worldY) != 0 || p.Point[2].Cmp(want) != 0 {
						t.Fatal("corrupt common world projection or original height residual", rotated, p.Point, worldX, worldY, want)
					}
					signs[p.Point[2].Sign()] = true
				}
			}
		}
		if !signs[-1] || !signs[0] || !signs[1] {
			t.Fatal("independent positive, zero and negative controls did not all run", signs)
		}
		for _, o := range []GeneralSurfaceHeightOptions{{MaxCells: 3}, {MaxPoles: 35}, {MaxOperations: r.ExactOperations - 1}} {
			partial, err := BuildProjectiveSurfaceHeightDifference(ctx, a, b, axes, o)
			if !errors.Is(err, ErrConvergence) || partial.Complete || len(partial.Cells) != 0 {
				t.Fatal("incomplete height atlas would lose potential contact", o, partial, err)
			}
		}
	}
}
