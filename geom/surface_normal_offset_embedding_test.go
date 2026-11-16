package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectedNormalOffsetEmbeddingKeepsBothSignedChartDirections(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		rotated    bool
		projection NormalOffsetProjection
	}{{false, NormalOffsetProjection{Axes: [2]int{0, 1}}},
		{true, NormalOffsetProjection{Axes: [2]int{1, 0}, Reversed: [2]bool{false, true}}}} {
		p := normalOffsetParabolicCylinder(t, tc.rotated)
		// The exact projected U derivative has sign
		// 1 - 2d/(1+4x²)^(3/2) throughout the displacement.
		// The projected V derivative is constant and independent of U.
		for _, distance := range []*big.Rat{big.NewRat(9, 20), big.NewRat(-2, 1)} {
			got, err := p.CertifyProjectedNormalOffsetEmbedding(ctx, distance, tc.projection, NormalOffsetCoverageOptions{})
			if err != nil || !got.Complete || len(got.Proven) <= 1 {
				t.Fatal("entire injective normal graph should certify", tc, distance, len(got.Proven), len(got.Unresolved), err)
			}
			checkNormalOffsetCover(t, p.Domain(), got)
		}
		atFocal, err := p.CertifyProjectedNormalOffsetEmbedding(ctx, big.NewRat(1, 2), tc.projection, NormalOffsetCoverageOptions{MaxDepth: 8})
		if !errors.Is(err, ErrConvergence) || atFocal.Complete || len(atFocal.Proven) == 0 || len(atFocal.Unresolved) == 0 {
			t.Fatal("focal-line loss must block the embedding", atFocal, err)
		}
		checkNormalOffsetCover(t, p.Domain(), atFocal)
		for _, r := range atFocal.Proven {
			if r.UMin <= .5 && r.UMax >= .5 {
				t.Fatal("certified cell contains the focal line", r)
			}
		}
	}
}

func TestProjectedNormalOffsetEmbeddingRetainsUnresolvedProjection(t *testing.T) {
	ctx := context.Background()
	p := normalOffsetParabolicCylinder(t, false)
	// The XZ projection drops the whole V tangent even at zero distance.
	got, err := p.CertifyProjectedNormalOffsetEmbedding(ctx, new(big.Rat), NormalOffsetProjection{Axes: [2]int{0, 2}},
		NormalOffsetCoverageOptions{MaxCells: 3})
	if !errors.Is(err, ErrConvergence) || got.Complete || len(got.Proven) != 0 || len(got.Unresolved) == 0 {
		t.Fatal("singular projection must not yield any covered region", got, err)
	}
	checkNormalOffsetCover(t, p.Domain(), got)
	if _, err := p.CertifyProjectedNormalOffsetEmbedding(ctx, new(big.Rat), NormalOffsetProjection{Axes: [2]int{1, 1}},
		NormalOffsetCoverageOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("both projection rows cannot alias the same world coordinate", err)
	}
}
