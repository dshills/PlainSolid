package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceNormalOffsetSweepCertifiesAllDisplacementLayers(t *testing.T) {
	ctx := context.Background()
	for _, spec := range []struct {
		rotated bool
		view    NormalOffsetSweepProjection
	}{{false, NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}},
		{true, NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{false, true, false}}}} {
		p, err := normalOffsetParabolicCylinder(t, spec.rotated).Trim(ParameterRectangle{.375, .625, 0, 1})
		if err != nil {
			t.Fatal(err)
		}
		// The input is a weakly curved parabolic strip around its apex,
		// with both sides of the source included in the thickened range.
		lo, hi := big.NewRat(-1, 10), big.NewRat(1, 10)
		got, err := p.CertifyNormalOffsetSweep(ctx, lo, hi, spec.view, NormalOffsetCoverageOptions{})
		if err != nil || !got.Complete || len(got.Proven) == 0 || len(got.Unresolved) != 0 {
			t.Fatal("subcritical normal sweep should be injective in all three parameters", spec, got, err)
		}
		checkNormalOffsetCover(t, p.Domain(), got)
		short, err := p.CertifyNormalOffsetSweep(ctx, lo, hi, spec.view, NormalOffsetCoverageOptions{
			Patch: SurfacePatchOptions{MaxOperations: got.ExactOperations - 1},
		})
		if !errors.Is(err, ErrConvergence) || short.Complete || len(short.Unresolved) == 0 {
			t.Fatal("insufficient work lost unresolved swept material", short, err)
		}
		checkNormalOffsetCover(t, p.Domain(), short)
	}
}

func TestSurfaceNormalOffsetSweepKeepsKnownFocalLineUnresolved(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, false).Trim(ParameterRectangle{.375, .625, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	// One layer has vanished U differential at the entire u=1/2 line:
	// x'_(offset)=2-4t there. It cannot be an embedded thickening.
	got, err := p.CertifyNormalOffsetSweep(ctx, big.NewRat(1, 10), big.NewRat(1, 2), view,
		NormalOffsetCoverageOptions{MaxDepth: 8})
	if !errors.Is(err, ErrConvergence) || got.Complete || len(got.Unresolved) == 0 {
		t.Fatal("sweep containing a focal line was accepted", got, err)
	}
	checkNormalOffsetCover(t, p.Domain(), got)
	for _, r := range got.Proven {
		if r.UMin <= .5 && r.UMax >= .5 {
			t.Fatal("proven sweep cell covers the exactly singular line", r)
		}
	}
	view.Reversed[2] = true
	got, err = p.CertifyNormalOffsetSweep(ctx, new(big.Rat), big.NewRat(1, 10), view,
		NormalOffsetCoverageOptions{MaxCells: 1})
	if !errors.Is(err, ErrConvergence) || got.Complete || len(got.Proven) != 0 {
		t.Fatal("wrong displacement direction must not pass monotonicity", got, err)
	}
	checkNormalOffsetCover(t, p.Domain(), got)
}
