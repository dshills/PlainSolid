package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedAnisotropicPointParts(t *testing.T) {
	ctx := context.Background()
	w := anisotropicSource(t, nil)
	r := big.NewRat
	tests := []struct {
		p        [3]*big.Rat
		location CylinderLocation
		parts    CylinderBoundaryPart
	}{
		{[3]*big.Rat{r(23, 12), r(19, 42), r(1, 3)}, CylinderBoundary, CylinderLateral},
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}, CylinderBoundary, CylinderLowerCap},
		{[3]*big.Rat{r(5, 4), r(-1, 2), r(1, 1)}, CylinderBoundary, CylinderLateral | CylinderUpperCap},
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(1, 2)}, CylinderInside, 0},
		{[3]*big.Rat{r(100, 1), r(100, 1), r(1, 1)}, CylinderOutside, 0},
	}
	for _, test := range tests {
		p, e := LocateWrappedAnisotropicRational(ctx, w, test.p, SurfacePatchOptions{})
		if e != nil || p.Location != test.location || p.Parts != test.parts || p.ExactOperations <= 0 {
			t.Fatal(p, e, test)
		}
	}
	// The weighted normalized profile contains (.71,.71), which lies outside
	// the unit circle; replacing the original conic with an ellipse is wrong.
	q := [3]*big.Rat{new(big.Rat).Add(r(1, 4), new(big.Rat).Mul(r(71, 100), r(7, 3))), new(big.Rat).Add(r(-1, 2), new(big.Rat).Mul(r(71, 100), r(4, 3))), r(1, 3)}
	if p, e := LocateWrappedAnisotropicRational(ctx, w, q, SurfacePatchOptions{}); e != nil || p.Location != CylinderInside {
		t.Fatal("circular substitution", p, e)
	}
	if p, e := LocateWrappedAnisotropicRational(ctx, w, q, SurfacePatchOptions{MaxOperations: 1}); e == nil || p.ExactOperations != 0 {
		t.Fatal("partial result", p, e)
	}
}
