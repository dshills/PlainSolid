package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestRadialTransferClosedStrata(t *testing.T) {
	rat := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	rect := func(x0, x1, y0, y1 *big.Rat) radialExactRectangle { return radialExactRectangle{{x0, x1}, {y0, y1}} }
	center := [2]*big.Rat{rat(0, 1), rat(0, 1)}
	radii := []*big.Rat{rat(1, 1)}
	tangent := rect(rat(1, 1), rat(2, 1), rat(-1, 2), rat(1, 2))
	vertex := rect(rat(1, 1), rat(2, 1), rat(0, 1), rat(1, 1))
	eps := rat(1, 1<<40)
	x, y := rat(4, 5), rat(3, 5)
	tiny := rect(new(big.Rat).Sub(x, eps), new(big.Rat).Add(x, eps), new(big.Rat).Sub(y, eps), new(big.Rat).Add(y, eps))
	left := tiny
	left[0][1] = new(big.Rat).Sub(x, new(big.Rat).Quo(eps, rat(2, 1)))
	for _, tc := range []struct {
		name    string
		box     radialExactRectangle
		targets []radialExactRectangle
		want    bool
	}{
		{"uncovered edge tangency", tangent, nil, false},
		{"uncovered vertex tangency", vertex, nil, false},
		{"covered boundary only", tangent, []radialExactRectangle{rect(rat(1, 1), rat(1, 1), rat(-1, 2), rat(1, 2))}, true},
		{"tiny enclosed arc", tiny, []radialExactRectangle{left}, false},
		{"whole rectangle", tiny, []radialExactRectangle{tiny}, true},
		{"outside", rect(rat(2, 1), rat(3, 1), rat(2, 1), rat(3, 1)), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &polynomialBudget{maxOperations: 100000, maxBits: 4096}
			var out SurfaceTubeUnionCoverage
			ok, e := radialRectangleStrataCovered(context.Background(), tc.box, tc.targets, center, radii, SurfaceTubeUnionCoverageOptions{MaxPieces: 1000, MaxCells: 1000}, b, &out)
			if e != nil || ok != tc.want {
				t.Fatal(ok, e, out)
			}
		})
	}
}
