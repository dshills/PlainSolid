package parabolacircle

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestReclockOsculationSpan(t *testing.T) {
	original := geom.ExactBezierSpan{Start: 0, End: 1, Control: [][4]*big.Rat{
		{big.NewRat(1, 2), big.NewRat(1, 8), big.NewRat(2, 1), big.NewRat(1, 1)},
		{big.NewRat(7, 16), big.NewRat(1, 8), big.NewRat(2, 1), big.NewRat(1, 1)},
		{big.NewRat(3, 4), big.NewRat(1, 2), big.NewRat(4, 1), big.NewRat(2, 1)},
	}}
	one := big.NewRat(1, 1)
	evaluate := func(span geom.ExactBezierSpan, t *big.Rat) [3]*big.Rat {
		work := append([][4]*big.Rat(nil), span.Control...)
		left := new(big.Rat).Sub(one, t)
		for n := len(work) - 1; n > 0; n-- {
			for i := range n {
				for j := range work[i] {
					a := new(big.Rat).Mul(work[i][j], left)
					b := new(big.Rat).Mul(work[i+1][j], t)
					work[i][j] = a.Add(a, b)
				}
			}
		}
		var point [3]*big.Rat
		for j := range point {
			point[j] = new(big.Rat).Quo(work[0][j], work[0][3])
		}
		return point
	}
	for _, speed := range []*big.Rat{big.NewRat(2, 1), big.NewRat(1, 2)} {
		got := original
		if err := reclockOsculationSpan(&got, speed, 1024); err != nil {
			t.Fatal(err)
		}
		for _, station := range []*big.Rat{new(big.Rat), big.NewRat(1, 7), big.NewRat(3, 8), one} {
			x := new(big.Rat).Mul(speed, station)
			y := new(big.Rat).Add(new(big.Rat).Sub(one, station), x)
			x.Quo(x, y)
			if expected, actual := evaluate(original, x), evaluate(got, station); !reflect.DeepEqual(expected, actual) {
				t.Fatalf("speed %s, station %s changed the exact trace: %v != %v", speed, station, actual, expected)
			}
		}
	}
	short := original
	if err := reclockOsculationSpan(&short, big.NewRat(2, 1), 2); !errors.Is(err, geom.ErrConvergence) ||
		&short.Control[0] != &original.Control[0] {
		t.Fatal("coefficient budget published a partial clock", err)
	}
}
