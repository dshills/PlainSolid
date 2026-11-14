package trimvalidate

import (
	"math/big"
	"math/rand"
	"testing"
)

func TestCoordinateHullExactDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(71))
	for n := 0; n < 1000; n++ {
		var a, b curve
		for _, c := range []*curve{&a, &b} {
			for i := range c.p {
				for k := range c.p[i] {
					c.p[i][k] = big.NewRat(int64(rng.Intn(31)-15), int64(1+rng.Intn(17)))
				}
			}
		}
		for _, allowed := range []*point{nil, &a.p[0]} {
			if got, want := hullSeparated(a, b, allowed), hullSeparatedAxes(a, b, allowed); got != want {
				t.Fatal(n, got, want)
			}
		}
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 4096))
	a := curve{p: [3]point{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}}}
	b := curve{p: [3]point{{new(big.Rat).Add(big.NewRat(1, 1), tiny), big.NewRat(0, 1)}, {big.NewRat(2, 1), big.NewRat(0, 1)}, {big.NewRat(2, 1), big.NewRat(1, 1)}}}
	if !coordinateHullSeparated(a, b) {
		t.Fatal("exact subnormal gap lost")
	}
	b.p[0][0] = big.NewRat(1, 1)
	if coordinateHullSeparated(a, b) {
		t.Fatal("touching hulls separated")
	}
	b.p[0][0] = new(big.Rat).Sub(big.NewRat(1, 1), tiny)
	if coordinateHullSeparated(a, b) {
		t.Fatal("overlapping hulls separated")
	}
}
