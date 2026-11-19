package trimmed

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nativeTwoPolynomialNodeFixture(t *testing.T, axis int, firstDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(firstDenominator*u-1)(2*u-1)(1+u^2)/64. The independent
	// source roof builder uses the complete quadratic gap in the dependent axis.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(17, 64), big.NewRat(22-firstDenominator, 64), big.NewRat(2*firstDenominator-23, 64), big.NewRat(6-firstDenominator, 64), big.NewRat(2*firstDenominator, 64)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativeWideTwoPolynomialNodeFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(4u-1)(4u-3)(1+u^2)/32 for the independent OCCT comparison.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(11, 32), big.NewRat(-1, 8), big.NewRat(7, 32), big.NewRat(-3, 8), big.NewRat(1, 2)}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func nativeTwoPolynomialNodeVolume(op parametric.BooleanOp, wide bool) float64 {
	// Over each free slab the negative gap integral is 105|g-f|^3/768.
	// Integrate the original product independently over the full rectangle
	// for its net contribution, and add the common base volume 769/256.
	if wide {
		if op == parametric.Union {
			return 6637852629.0 / 2147483648
		}
		return 6450735147.0 / 2147483648
	}
	if op == parametric.Union {
		return 23331176626458563.0 / 7547047813054464
	}
	return 22670564921241661.0 / 7547047813054464
}

func TestNativeTwoPolynomialNodeOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_TWO_POLYNOMIAL_NODES")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node-pair").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Axis, Order, Operation, Solids int
		Valid                          bool
		Volume                         float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 8 {
		t.Fatal(string(data), e)
	}
	var seen [2][2][2]bool
	for _, r := range rows {
		if r.Axis < 0 || r.Axis > 1 || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 1 || seen[r.Axis][r.Order][r.Operation] {
			t.Fatal(r)
		}
		seen[r.Axis][r.Order][r.Operation] = true
		op := [2]parametric.BooleanOp{parametric.Intersection, parametric.Union}[r.Operation]
		want := nativeTwoPolynomialNodeVolume(op, true)
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-want) > 1e-9 {
			t.Fatal(r, want)
		}
	}
}
