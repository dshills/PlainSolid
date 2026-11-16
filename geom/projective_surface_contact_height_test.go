package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveContactHeightClassifiesBothOriginalLifts(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	hole := ExactSurfaceRectangle{ExactParameterInterval{big.NewRat(2, 5), big.NewRat(3, 5)}, ExactParameterInterval{big.NewRat(9, 16), big.NewRat(5, 8)}}
	g, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{
		Cover: ProjectiveSurfaceContactCoverOptions{OriginalHoles: [2][]ExactSurfaceRectangle{{hole}, nil}},
	})
	if err != nil || !g.Complete {
		t.Fatal("exact original contact prerequisites not proved", err, g)
	}
	// Erase exposed result fields; classification must depend only on
	// the authentic source cell ledger and not caller-editable output.
	g.Cover.Atlas.Cells = nil
	g.Cover.CommonHoles = nil
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	for _, sample := range []struct {
		u, v        *big.Rat
		hole, outer bool
	}{
		{big.NewRat(1, 8), big.NewRat(9, 10), false, false},
		{big.NewRat(7, 8), big.NewRat(9, 10), false, false},
		{big.NewRat(1, 2), big.NewRat(1, 8), true, false},
		{big.NewRat(1, 2), new(big.Rat), false, true},
	} {
		p, err := g.ClassifyCommonHeightRational(ctx, [2]*big.Rat{sample.u, sample.v}, SurfacePatchEvaluationOptions{})
		if err != nil || p.OnTrimBoundary != sample.hole || p.OnOverlapBoundary != sample.outer {
			t.Fatal("exact original station not classified on its right boundary", sample, p, err)
		}
		want := new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 2), sample.u), tiny)
		if p.Sign != want.Sign() || p.Difference.Cmp(want) != 0 {
			t.Fatal("small positive, zero or negative lift was changed", p, want)
		}
		yz := new(big.Rat).Quo(new(big.Rat).Add(big.NewRat(1, 1), sample.v), big.NewRat(2, 1))
		if p.Projected[0].Cmp(sample.u) != 0 || p.Projected[1].Cmp(yz) != 0 {
			t.Fatal("original common physical projection changed", p, sample)
		}
		var z [2]*big.Rat
		for i, patch := range [2]*SurfacePatch{a, b} {
			q, err := patch.EvaluateRational(ctx, p.OriginalUV[i][0], p.OriginalUV[i][1], SurfacePatchEvaluationOptions{})
			if err != nil || q.Point[0].Cmp(p.Projected[0]) != 0 || q.Point[1].Cmp(p.Projected[1]) != 0 {
				t.Fatal("original rational UV replay failed", i, p, q, err)
			}
			z[i] = q.Point[2]
		}
		if new(big.Rat).Sub(z[0], z[1]).Cmp(p.Difference) != 0 {
			t.Fatal("difference does not replay in both original charts", p, z)
		}
	}
	if p, err := g.ClassifyCommonHeightRational(ctx, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 16)}, SurfacePatchEvaluationOptions{}); !errors.Is(err, ErrDomain) || p.Difference != nil {
		t.Fatal("material hole interior must not acquire a source classification", p, err)
	}
	if p, err := g.ClassifyCommonHeightRational(ctx, [2]*big.Rat{big.NewRat(1, 2), new(big.Rat)}, SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, ErrConvergence) || p.Difference != nil {
		t.Fatal("short work published a false original ordering", p, err)
	}
}
