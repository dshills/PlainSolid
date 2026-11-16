package parametric

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestNativeExactBezierCurvePreservesOriginalClockAndControls(t *testing.T) {
	ctx := context.Background()
	z := new(big.Rat)
	one := big.NewRat(1, 1)
	c, err := geom.NewExactHomogeneousSpline(ctx, 2, [][4]*big.Rat{
		{big.NewRat(2, 7), z, z, one},
		{big.NewRat(7, 11), z, one, big.NewRat(3, 5)},
		{big.NewRat(9, 7), one, z, one},
	}, []float64{-2, -2, -2, 3, 3, 3}, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := EncodeCurve(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeCurve(wire, 3)
	if err != nil {
		t.Fatal(err)
	}
	reopened, ok := got.(*geom.ExactSplineCurve)
	if !ok {
		t.Fatal("native exact curve became an ordinary float descriptor", got)
	}
	lo, hi := reopened.Domain()
	if lo != -2 || hi != 3 {
		t.Fatal("original spline clock changed", lo, hi)
	}
	x, err := geom.CompareExactCurvesContext(ctx, c, reopened, geom.ExactCurveComparisonOptions{})
	if err != nil || !x.Complete || !x.Equal {
		t.Fatal("native exact curve changed", x, err)
	}
	if next, err := EncodeCurve(reopened); err != nil || !bytes.Equal(next, wire) {
		t.Fatal("exact native replay bytes changed", err)
	}
	if _, err := DecodeCurve(wire, 2); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("exact curve avoided control count limit", err)
	}
	for _, mutation := range [][]byte{
		bytes.Replace(wire, []byte(`"2/7"`), []byte(`"4/14"`), 1),
		bytes.Replace(wire, []byte(`"3/5"`), []byte(`"0"`), 1),
		bytes.Replace(wire, []byte(`"Kind":`), []byte(`"Degree":2,"Kind":`), 1),
	} {
		if _, err := DecodeCurve(mutation, 3); !errors.Is(err, geom.ErrInvalid) {
			t.Fatal("invalid exact curve envelope was accepted", err)
		}
	}
}
