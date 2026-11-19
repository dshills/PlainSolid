package step

import (
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestClippedFuseSTEPNonDecimalPoleRequiresExplicitBound(t *testing.T) {
	t.Parallel()
	q := func(a, b int64) *big.Rat { return big.NewRat(a, b) }
	s := geom.ExactBezierSpan{Start: 0, End: 1, Control: [][4]*big.Rat{
		{q(0, 1), q(0, 1), q(0, 1), q(1, 1)},
		{q(1, 7), q(1, 3), q(0, 1), q(1, 1)},
		{q(1, 1), q(1, 1), q(0, 1), q(1, 1)},
	}}
	for _, tol := range []*big.Rat{q(0, 1), q(1, 1000000000000000000), q(1, 100000000000000)} {
		calls := int64(0)
		id, err := emitClippedFuseSTEPBezier(func(string) int64 { calls++; return calls }, s, 2, tol)
		if tol.Cmp(q(1, 100000000000000)) < 0 {
			if !errors.Is(err, geom.ErrUnrepresentable) || calls != 0 || id != 0 {
				t.Fatalf("tolerance %s: entities %d result %d error %v", tol, calls, id, err)
			}
		} else if err != nil || calls != 4 || id != 4 {
			t.Fatalf("bounded decimal conversion: entities %d result %d error %v", calls, id, err)
		}
	}
}
