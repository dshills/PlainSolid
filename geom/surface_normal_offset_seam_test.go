package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceNormalOffsetSeamMatchesTheWholeOriginalRationalEdge(t *testing.T) {
	ctx := context.Background()
	a := normalSeamParaboloidPatch(t, false, false, 0)
	b := normalSeamParaboloidPatch(t, true, true, 0)
	options := SurfaceNormalOffsetSeamOptions{ReverseB: true, FlipNormalB: true}
	ok, err := CertifySurfaceNormalOffsetSeam(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, options)
	if err != nil || !ok.Complete || !ok.SharedOriginalEdge {
		t.Fatal("opposite original chart orientations should produce the same normals", ok, err)
	}
	options.MaxOperations = ok.ExactOperations - 1
	short, err := CertifySurfaceNormalOffsetSeam(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, options)
	if !errors.Is(err, ErrConvergence) || short.Complete {
		t.Fatal("exhausted proof published a complete seam", short, err)
	}
	options.MaxOperations, options.FlipNormalB = 0, false
	reversed, err := CertifySurfaceNormalOffsetSeam(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, options)
	if err != nil || reversed.Complete || !reversed.SharedOriginalEdge {
		t.Fatal("opposite signed normals would separate a nonzero offset", reversed, err)
	}
	options.ReverseB = false
	wrong, err := CertifySurfaceNormalOffsetSeam(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, options)
	if err != nil || wrong.Complete || wrong.SharedOriginalEdge {
		t.Fatal("opposite free stations do not describe the same original points", wrong, err)
	}

	crease := normalSeamParaboloidPatch(t, true, false, 1)
	got, err := CertifySurfaceNormalOffsetSeam(ctx, a, crease, SurfaceEdgeUMax, SurfaceEdgeUMin, SurfaceNormalOffsetSeamOptions{})
	if err != nil || got.Complete || !got.SharedOriginalEdge {
		t.Fatal("creased surface must not be sewn without a join", got, err)
	}
}

func TestSurfaceNormalOffsetSeamFindsAnInteriorSingularStation(t *testing.T) {
	ctx := context.Background()
	patch := func(right bool) *SurfacePatch {
		t.Helper()
		var h [][4]*big.Rat
		for i := int64(0); i < 2; i++ {
			x := i - 1
			if right {
				x = i
			}
			for _, y := range []int64{-1, 1, -1, 1} {
				h = append(h, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat), big.NewRat(1, 1)})
			}
		}
		p, err := NewExactHomogeneousBezierSurface(ctx, 1, 3, h, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Both edges are (0,(2v-1)^3,0). Their oriented unit normals
	// exist and agree at both ends, but the seam tangent vanishes at v=1/2.
	got, err := CertifySurfaceNormalOffsetSeam(ctx, patch(false), patch(true), SurfaceEdgeUMax, SurfaceEdgeUMin, SurfaceNormalOffsetSeamOptions{})
	if err != nil || got.Complete || !got.SharedOriginalEdge {
		t.Fatal("an interior singular station must prevent a complete seam", got, err)
	}
}

func normalSeamParaboloidPatch(t *testing.T, right, reverse bool, crease int64) *SurfacePatch {
	t.Helper()
	var h [][4]*big.Rat
	for i := int64(0); i <= 2; i++ {
		for j := int64(0); j <= 2; j++ {
			at := j
			if reverse {
				at = 2 - j
			}
			x, y, w := big.NewRat(i-2, 2), big.NewRat(at-1, 1), big.NewRat(1, 1)
			zx := []int64{1, 0, 0}[i]
			if right {
				x, w, zx = big.NewRat(i, 2), big.NewRat(2, 1), []int64{0, 0, 1}[i]
			}
			z := new(big.Rat).Add(big.NewRat(zx+[]int64{1, -1, 1}[at], 1), new(big.Rat).Mul(x, big.NewRat(crease, 4)))
			h = append(h, [4]*big.Rat{x.Mul(x, w), y.Mul(y, w), z.Mul(z, w), w})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(context.Background(), 2, 2, h, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
