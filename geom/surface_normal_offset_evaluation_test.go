package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceNormalOffsetRationalEnclosesIrrationalCoordinates(t *testing.T) {
	ctx := context.Background()
	p := normalOffsetParabolicCylinder(t, false)
	jet, err := p.EvaluateRational(ctx, big.NewRat(3, 4), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The source point is (1/2,1/3,1/4), and the unit normal
	// is (-1,0,1)/sqrt(2). A 1/3 offset has squared displacement
	// 1/18 along each of X and Z.
	inside, err := jet.NormalOffsetRational(ctx, big.NewRat(1, 3), SurfaceNormalOffsetEvaluationOptions{SqrtBits: 128})
	if err != nil {
		t.Fatal(err)
	}
	pz := big.NewRat(1, 4)
	zlo, zhi := new(big.Rat).Sub(inside.Lower[2], pz), new(big.Rat).Sub(inside.Upper[2], pz)
	sq := func(r *big.Rat) *big.Rat { return new(big.Rat).Mul(r, r) }
	if zlo.Sign() <= 0 || sq(zlo).Cmp(big.NewRat(1, 18)) >= 0 || sq(zhi).Cmp(big.NewRat(1, 18)) <= 0 {
		t.Fatal("normal offset enclosure lost the algebraic coordinate", zlo, zhi)
	}
	px, py := big.NewRat(1, 2), big.NewRat(1, 3)
	if inside.Lower[0].Cmp(new(big.Rat).Sub(px, zhi)) != 0 || inside.Upper[0].Cmp(new(big.Rat).Sub(px, zlo)) != 0 ||
		inside.Lower[1].Cmp(py) != 0 || inside.Upper[1].Cmp(py) != 0 {
		t.Fatal("normal components lost the original orientation", inside)
	}
	coarse, err := jet.NormalOffsetRational(ctx, big.NewRat(1, 3), SurfaceNormalOffsetEvaluationOptions{SqrtBits: 12})
	if err != nil || coarse.Lower[2].Cmp(inside.Lower[2]) >= 0 || coarse.Upper[2].Cmp(inside.Upper[2]) <= 0 {
		t.Fatal("higher square-root precision did not tighten the enclosure", coarse, err)
	}
	limited, err := jet.NormalOffsetRational(ctx, big.NewRat(1, 3), SurfaceNormalOffsetEvaluationOptions{
		SqrtBits: 128, MaxOperations: inside.ExactOperations - 1,
	})
	if !errors.Is(err, ErrConvergence) || limited.Lower[0] != nil || limited.Upper[0] != nil {
		t.Fatal("exhausted work published a partial offset point", limited, err)
	}
}

func TestSurfaceNormalOffsetRationalKeepsTinyNonzeroTangents(t *testing.T) {
	ctx := context.Background()
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 600))
	zero := new(big.Rat)
	p := RationalSurfacePatchJet{Point: [3]*big.Rat{zero, zero, big.NewRat(5, 7)},
		Du: [3]*big.Rat{tiny, zero, zero}, Dv: [3]*big.Rat{zero, tiny, zero}}
	got, err := p.NormalOffsetRational(ctx, big.NewRat(-3, 7), SurfaceNormalOffsetEvaluationOptions{})
	if err != nil || got.Lower[2].Cmp(big.NewRat(2, 7)) != 0 || got.Upper[2].Cmp(got.Lower[2]) != 0 {
		t.Fatal("a rational unit normal should give exact output", got, err)
	}
	got.Lower[2].SetInt64(12)
	if p.Point[2].Cmp(big.NewRat(5, 7)) != 0 || got.Upper[2].Cmp(big.NewRat(2, 7)) != 0 {
		t.Fatal("result bounds must own independent rational storage")
	}
	p.Dv = p.Du
	if _, err := p.NormalOffsetRational(ctx, big.NewRat(1, 7), SurfaceNormalOffsetEvaluationOptions{}); !errors.Is(err, ErrDegenerate) {
		t.Fatal("singular tangent pair has no unit normal", err)
	}
}
