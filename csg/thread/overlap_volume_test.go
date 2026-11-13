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
