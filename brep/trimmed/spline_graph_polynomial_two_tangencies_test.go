package trimmed

import (
	"math/big"
	"testing"

	"plainsolid/geom"
)

func nativeTwoTangenciesFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// Subtract f to obtain (u-1/4)^2(u-3/4)^2, (7/8)(u-1/8)^2(u-5/8)^2,
	// or (5u-1)^2(4u-3)^2/256 for the nonbinary coordinate control.
	g := [3][]*big.Rat{
		{big.NewRat(73, 256), new(big.Rat), big.NewRat(1, 1), big.NewRat(-15, 8), big.NewRat(1, 1)},
		{big.NewRat(8367, 32768), big.NewRat(279, 1024), big.NewRat(65, 256), big.NewRat(-19, 16), big.NewRat(7, 8)},
		{big.NewRat(73, 256), big.NewRat(-18, 256), big.NewRat(385, 256), big.NewRat(-728, 256), big.NewRat(400, 256)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}
