package thread

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestPairCertifiesPositiveRunoutVolume(t *testing.T) {
	ctx := context.Background()
	p, e := BuildPair(ctx, pairFixture(t, geom.RightHanded), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := [2]*big.Rat{big.NewRat(29, 16), big.NewRat(31, 16)}
	phase := [2]*big.Rat{new(big.Rat), big.NewRat(1, 8)}
	z := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8)}
	got, e := p.CertifyOverlapVolumeBox(ctx, new(big.Rat), r, phase, z, geom.HelicalStartsLineOptions{})
	if e != nil || !got.Proven || got.PiCoefficient.Cmp(big.NewRat(15, 1024)) != 0 {
		t.Fatal(got, e)
	}
	relief := PairReliefs{
		ShaftTip: trimmed.HelicalReliefSpec{Radius: 1.875, Depth: .5},
		NutLower: trimmed.HelicalReliefSpec{Radius: 2.125, Depth: .5},
	}
	finished, e := BuildRelievedPair(ctx, pairFixture(t, geom.RightHanded), relief, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if out, e := finished.CertifyOverlapVolumeBox(ctx, new(big.Rat), r, phase, z, geom.HelicalStartsLineOptions{}); !errors.Is(e, geom.ErrDomain) || out.Proven {
		t.Fatal("box crosses removed relief", out, e)
	}
	z = [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 4)}
	got, e = finished.CertifyOverlapVolumeBox(ctx, new(big.Rat), r, phase, z, geom.HelicalStartsLineOptions{})
	if e != nil || !got.Proven || got.PiCoefficient.Cmp(big.NewRat(15, 1024)) != 0 {
		t.Fatal(got, e)
	}
}

func TestPairIntegratesConstantRunoutCell(t *testing.T) {
	ctx := context.Background()
	p, e := BuildPair(ctx, pairFixture(t, geom.LeftHanded), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := [2]*big.Rat{big.NewRat(29, 16), big.NewRat(31, 16)}
	phase := [2]*big.Rat{new(big.Rat), big.NewRat(1, 8)}
	got, e := p.IntegrateOverlapCell(ctx, big.NewRat(8, 1), r, phase, geom.HelicalStartsLineOptions{})
	if e != nil || !got.Proven || got.PiCoefficient.Sign() != 0 {
		t.Fatal("cap-only contact must have zero common volume", got, e)
	}
	got, e = p.IntegrateOverlapCell(ctx, new(big.Rat), r, phase, geom.HelicalStartsLineOptions{})
	// The active lattice indices are 2 through 5. At each index k
	// the shaft cutter [k−1/8,k+5/8] and nut cutter [k+1/2,k+1]
	// overlap and meet the next shaft cutter. After the common phase
	// translation the cut union is [15/8,6], of length 33/8.
	if e != nil || !got.Proven || got.LengthSlope.Sign() != 0 || got.LengthIntercept.Cmp(big.NewRat(31, 8)) != 0 ||
		got.PiCoefficient.Cmp(big.NewRat(465, 2048)) != 0 {
		t.Fatal("wrong constant-cell overlap", got, e)
	}
}
