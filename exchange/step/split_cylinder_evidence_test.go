package step

import (
	"bytes"
	"math/big"
	"os"
	"testing"
)

// The independently transferred OCCT shape is valid at its tolerance, but its
// represented generatrices and vertices do not lie on the exact cylinder.
func TestSplitCylinderOCCTExactSourceDiscrepancy(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("testdata/split-cylinder-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []int64{23, 25, 59, 87, 117, 118} {
		r, e := simple(f, id, "CARTESIAN_POINT", 2)
		if e != nil {
			t.Fatal(e)
		}
		p := r.Args[1].Items
		if len(p) != 3 {
			t.Fatal(id)
		}
		x, e := liftSourceRat(p[0])
		if e != nil {
			t.Fatal(e)
		}
		y, e := liftSourceRat(p[1])
		if e != nil {
			t.Fatal(e)
		}
		residual := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		residual.Sub(residual, big.NewRat(4, 1))
		if residual.Sign() <= 0 || residual.Cmp(new(big.Rat).Mul(y, y)) != 0 {
			t.Fatalf("unexpected exact residual at %d: %v", id, residual)
		}
	}
	for _, data := range [][]byte{b, bytes.ReplaceAll(b, []byte("4.898587196589E-16"), []byte("1.E-3"))} {
		if m, e := ReadAnalyticBodies(bytes.NewReader(data), Limits{}); e == nil {
			t.Fatalf("unsupported split shell accepted: %v", m)
		}
	}
}
