package trimmed

import (
	"math/big"
	"testing"
)

// The lower and upper graphs are Z=X^degree/16+(1+X)Y/4+W.
// Degree four control grids represent X^3 as well as X^4 exactly.

func TestNativeExactTrimCoefficientBounds(t *testing.T) {
	for _, raw := range []string{"1e999999999999", "0x10", "01/2", "2/4", "-0", "1/0", "+1"} {
		if _, e := parseNativeExactTrimCoefficient(raw); e == nil {
			t.Errorf("accepted noncanonical or oversized coefficient %q", raw)
		}
	}
	if r, e := parseNativeExactTrimCoefficient("-2/3"); e != nil || r.Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatal("canonical signed coordinate", r, e)
	}
}
