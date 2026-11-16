package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetPatchAnalyticPositionAndPartials(t *testing.T) {
	ctx := context.Background()
	source := normalOffsetParabolicCylinder(t, false)
	fixed := big.NewRat(1, 4)
	xy := NormalOffsetProjection{Axes: [2]int{0, 1}}
	offset, proof, err := NewVerifiedNormalOffsetPatch(ctx, source, fixed, xy, NormalOffsetCoverageOptions{})
	if err != nil || offset == nil || !proof.Complete || offset.Domain() != source.Domain() {
		t.Fatal("unable to construct the verified original normal graph", proof, err)
	}
	fixed.SetInt64(9)
	offset.Distance().SetInt64(-5)
	if offset.Distance().Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal("offset distance aliases caller storage")
	}
	u, v := big.NewRat(1, 2), big.NewRat(1, 3)
	pt, err := offset.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil || pt.Lower[2].Cmp(big.NewRat(1, 4)) != 0 || pt.Upper[2].Cmp(pt.Lower[2]) != 0 {
		t.Fatal("offset point at the apex is exactly one quarter above the source", pt, err)
	}
	a, b, err := offset.PartialsRational(ctx, u, v, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil || a.Lower[0].Cmp(big.NewRat(1, 1)) != 0 || a.Upper[0].Cmp(a.Lower[0]) != 0 ||
		a.Lower[2].Sign() != 0 || b.Lower[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("rational derivative at the apex must contain the normal derivative", a, b, err)
	}
	u = big.NewRat(3, 4)
	a, b, err = offset.PartialsRational(ctx, u, v, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// At x=1/2 both U components are 2-sqrt(2)/4. Verify the
	// enclosure by independent exact squaring, without rounding sqrt(2).
	for _, axis := range []int{0, 2} {
		dmin := new(big.Rat).Sub(big.NewRat(2, 1), a.Upper[axis])
		dmax := new(big.Rat).Sub(big.NewRat(2, 1), a.Lower[axis])
		if dmin.Sign() <= 0 || dmin.Mul(dmin, dmin).Cmp(big.NewRat(1, 8)) >= 0 || dmax.Mul(dmax, dmax).Cmp(big.NewRat(1, 8)) <= 0 {
			t.Fatal("irrational analytic partial escaped the directed enclosure", axis, a)
		}
	}
	fa, fb, err := offset.Partials(.75, .25)
	want := 2 - math.Sqrt(2)/4
	if err != nil || math.Abs(fa.X-want) > 1e-15 || math.Abs(fa.Z-want) > 1e-15 || fb != (Vec3{0, 1, 0}) {
		t.Fatal("numerical Surface adapter lost the analytic derivative", fa, fb, err)
	}
	if _, err := offset.Eval(-.125, .25); !errors.Is(err, ErrDomain) {
		t.Fatal("surface accepted an out-of-domain original parameter", err)
	}
	short, _, err := offset.PartialsRational(ctx, u, v, SurfacePatchEvaluationOptions{},
		SurfaceNormalOffsetEvaluationOptions{MaxOperations: a.ExactOperations - 1})
	if !errors.Is(err, ErrConvergence) || short.Lower[0] != nil {
		t.Fatal("insufficient derivative work published a partial result", short, err)
	}
}

func TestVerifiedNormalOffsetPatchRejectsFocalLineBeforeConstruction(t *testing.T) {
	p := normalOffsetParabolicCylinder(t, false)
	s, proof, err := NewVerifiedNormalOffsetPatch(context.Background(), p, big.NewRat(1, 2),
		NormalOffsetProjection{Axes: [2]int{0, 1}}, NormalOffsetCoverageOptions{MaxDepth: 6})
	if !errors.Is(err, ErrConvergence) || s != nil || proof.Complete || len(proof.Unresolved) == 0 {
		t.Fatal("invalid offset produced a usable Surface", s, proof, err)
	}
	checkNormalOffsetCover(t, p.Domain(), proof)
}
