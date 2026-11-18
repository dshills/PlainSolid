package trimmed

import (
	"math/big"
	"testing"

	"plainsolid/geom"
)

func nativeSimpleCubicCrossingsFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// The original separations are (u-1/4)^3(u-3/4)/2,
	// 3(u-1/4)(u-3/4)^3/4, and (5u-1)^3(4u-3)/1024.
	g := [3][]*big.Rat{
		{big.NewRat(131, 512), big.NewRat(19, 64), new(big.Rat), big.NewRat(-5, 8), big.NewRat(1, 2)},
		{big.NewRat(337, 1024), big.NewRat(-33, 128), big.NewRat(21, 16), big.NewRat(-7, 4), big.NewRat(3, 4)},
		{big.NewRat(259, 1024), big.NewRat(335, 1024), big.NewRat(-99, 1024), big.NewRat(-547, 1024), big.NewRat(500, 1024)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}
