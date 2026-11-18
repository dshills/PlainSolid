package trimmed

import (
	"math/big"
	"testing"
)

func TestMixedRoofClearanceSeesInteriorSign(t *testing.T) {
	p := zeroTrianglePower5()
	p[0][0].Sub(big.NewRat(2, 9), big.NewRat(1, 100))
	p[1][0].SetFrac64(-2, 3)
	p[0][1].SetFrac64(-2, 3)
	p[2][0].SetInt64(1)
	p[0][2].SetInt64(1)
	lo, hi := mixedRoofPolynomialRange(p)
	if lo.Cmp(big.NewRat(-1, 100)) > 0 || lo.Sign() >= 0 || hi.Sign() <= 0 {
		t.Fatal("vertex-only signed clearance")
	}
}
