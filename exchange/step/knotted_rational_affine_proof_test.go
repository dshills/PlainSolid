package step

import (
	"math/big"
	"testing"
)

func TestQuadraticOriginalSpanRestriction(t *testing.T) {
	t.Parallel()
	knots := [3]*big.Rat{big.NewRat(-3, 1), big.NewRat(0, 1), big.NewRat(3, 1)}
	want := [2][4][3]string{{{"1", "-2", "1"}, {"0", "2", "-3/2"}, {"0", "0", "1/2"}, {"0", "0", "0"}}, {{"0", "0", "0"}, {"1/2", "-1", "1/2"}, {"1/2", "1", "-3/2"}, {"0", "0", "1"}}}
	for s := 0; s < 2; s++ {
		got, e := quadraticOriginalSpanBasis(knots, s)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 4; i++ {
			for j := 0; j < 3; j++ {
				if got[i][j].RatString() != want[s][i][j] {
					t.Fatal(s, i, j, got[i][j], want[s][i][j])
				}
			}
		}
	}
	// Nonuniform exact span restriction must reproduce both partition of unity
	// and the original parameter from independently computed Greville values.
	knots = [3]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(3, 1)}
	greville := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(2, 1), big.NewRat(3, 1)}
	for s := 0; s < 2; s++ {
		basis, e := quadraticOriginalSpanBasis(knots, s)
		if e != nil {
			t.Fatal(e)
		}
		for c := 0; c < 3; c++ {
			unity, line := new(big.Rat), new(big.Rat)
			for i := 0; i < 4; i++ {
				unity.Add(unity, basis[i][c])
				line.Add(line, new(big.Rat).Mul(greville[i], basis[i][c]))
			}
			u, v := new(big.Rat), new(big.Rat)
			if c == 0 {
				u.SetInt64(1)
				v.Set(knots[s])
			}
			if c == 1 {
				v.Sub(knots[s+1], knots[s])
			}
			if unity.Cmp(u) != 0 || line.Cmp(v) != 0 {
				t.Fatal(s, c, unity, line)
			}
		}
	}
}
