package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepBoundaryAreaRationalSlab(t *testing.T) {
	ctx := context.Background()
	slab, _ := sweepBoundarySlabFixture(t)
	q := big.NewRat
	r, err := slab.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{AbsoluteError: new(big.Rat)})
	if err != nil || !r.Complete || len(r.Faces) != 6 || r.VisitedCells != 6 ||
		r.Bounds.Lower.Cmp(q(382, 21)) != 0 || r.Bounds.Upper.Cmp(q(382, 21)) != 0 {
		t.Fatal("expected exact six-face area of the translated rotated rational slab", r, err)
	}
	for i, want := range [6]*big.Rat{q(13, 7), q(13, 7), q(26, 21), q(26, 21), q(6, 1), q(6, 1)} {
		f := r.Faces[i]
		if f.Side != NormalOffsetSweepSide(i) || f.Bounds.Lower.Cmp(want) != 0 || f.Bounds.Upper.Cmp(want) != 0 ||
			f.SquaredArea.Cmp(new(big.Rat).Mul(want, want)) != 0 {
			t.Fatal("lost original affine face ownership or exact physical area", i, f, want)
		}
	}
	t.Logf("translated rotated rational slab total %s, %d visited faces, %d exact operations",
		r.Bounds.Lower, r.VisitedCells, r.ExactOperations)
}

func TestVerifiedNormalOffsetSweepBoundaryAreaIrrationalSlab(t *testing.T) {
	ctx := context.Background()
	slab := sweepAreaShearedSlab(t)
	q := big.NewRat
	tolerance := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 150))
	r, err := slab.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{AbsoluteError: tolerance})
	if err != nil || !r.Complete || len(r.Faces) != 6 || r.VisitedCells != 6 ||
		new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower).Cmp(tolerance) > 0 {
		t.Fatal("expected a directed narrow enclosure and all six original area squares", r, err)
	}
	squares := [6]*big.Rat{q(15925, 20449), q(15925, 20449), q(6125, 20449), q(6125, 20449), q(61, 1), q(61, 1)}
	sumLo, sumHi := new(big.Rat), new(big.Rat)
	for i, square := range squares {
		f := r.Faces[i]
		if f.Side != NormalOffsetSweepSide(i) || f.SquaredArea.Cmp(square) != 0 {
			t.Fatal("wrong analytic squared face area", f, square)
		}
		lo, hi := f.Bounds.Lower, f.Bounds.Upper
		if lo.Sign() <= 0 || lo.Cmp(hi) >= 0 || new(big.Rat).Mul(lo, lo).Cmp(square) >= 0 ||
			new(big.Rat).Mul(hi, hi).Cmp(square) <= 0 {
			t.Fatal("directed endpoints did not enclose the exact irrational area", f)
		}
		sumLo.Add(sumLo, lo)
		sumHi.Add(sumHi, hi)
	}
	if r.Bounds.Lower.Cmp(sumLo) != 0 || r.Bounds.Upper.Cmp(sumHi) != 0 {
		t.Fatal("total endpoints must sum the complete directed face cover", r.Bounds)
	}
	t.Logf("irrational sheared slab total lies in [%s,%s], width %s, %d exact operations",
		r.Bounds.Lower.FloatString(50), r.Bounds.Upper.FloatString(50),
		new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower), r.ExactOperations)

	for _, opts := range []NormalOffsetSweepAreaOptions{
		{AbsoluteError: new(big.Rat)},
		{AbsoluteError: q(1, 1<<48), Patch: SurfacePatchOptions{MaxCoefficientBits: 64}},
	} {
		coarse, err := slab.BoundaryAreaRational(ctx, opts)
		if !errors.Is(err, ErrConvergence) || coarse.Complete || len(coarse.Faces) != 6 ||
			coarse.Bounds.Lower == nil || coarse.Bounds.Upper == nil ||
			coarse.Bounds.Lower.Cmp(r.Bounds.Lower) > 0 || coarse.Bounds.Upper.Cmp(r.Bounds.Upper) < 0 {
			t.Fatal("too-tight precision should retain the valid coarse total and each exact square", coarse, err)
		}
		for i, f := range coarse.Faces {
			if f.SquaredArea.Cmp(squares[i]) != 0 {
				t.Fatal("rounding limits altered the proved exact area square", i, f)
			}
		}
	}
	short, err := slab.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{MaxCells: 5})
	if !errors.Is(err, ErrConvergence) || short.Complete || len(short.Faces) != 5 || short.VisitedCells != 5 ||
		short.Bounds.Lower != nil || short.Bounds.Upper != nil {
		t.Fatal("an incomplete set of original faces cannot publish a total", short, err)
	}
	short, err = slab.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{Patch: SurfacePatchOptions{MaxOperations: 4}})
	if !errors.Is(err, ErrConvergence) || short.Complete || short.Bounds.Lower != nil || len(short.Faces) != 0 {
		t.Fatal("short exact arithmetic published unproved faces", short, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, err := slab.BoundaryAreaRational(cancelled, NormalOffsetSweepAreaOptions{}); !errors.Is(err, context.Canceled) ||
		r.Complete || r.Bounds.Lower != nil || len(r.Faces) != 0 {
		t.Fatal("cancelled area query published an enclosure", r, err)
	}
}

func sweepAreaShearedSlab(t *testing.T) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: ParameterRectangle{2, 3, -1, 1}}
	for u := range 2 {
		for v := range 2 {
			x := new(big.Rat).Add(q(int64(2*u), 1), q(3, 8))
			y := new(big.Rat).Sub(q(int64(3*v), 1), q(5, 7))
			z := new(big.Rat).Add(q(int64(u+2*v), 1), q(9, 11))
			// Constant nonunit homogeneous weight and raw U,V widths
			// retain the authored source law independently of area.
			w := q(3, 5)
			recipe.Controls = append(recipe.Controls, [4]string{
				x.Mul(x, w).RatString(), y.Mul(y, w).RatString(), z.Mul(z, w).RatString(), w.RatString(),
			})
		}
	}
	source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	slab, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, q(-1, 11), q(2, 13),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("authored affine slab with irrational unit normal did not verify", proof, err)
	}
	return slab
}
